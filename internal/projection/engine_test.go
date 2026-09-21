package projection

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/observation"
	"github.com/rbee3u/plutus/internal/portfolio"
)

func TestRebuildActivatesEmptyPortfolio(t *testing.T) {
	db := mysql.OpenTestDB(t)
	portfolioID, jobID := setupPortfolioJob(t, db)
	svc := NewService(db)
	if err := svc.Rebuild(context.Background(), Request{PortfolioID: portfolioID, JobID: jobID}); err != nil {
		t.Fatal(err)
	}
	active, err := jobs.NewService(db).ActiveGenerationID(context.Background(), portfolioID)
	if err != nil || active == uuid.Nil {
		t.Fatalf("active %s %v", active, err)
	}
	gen, err := sqlcgen.New(db).GetProjectionGeneration(context.Background(), sqlcgen.GetProjectionGenerationParams{
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(active),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gen.Status != "active" || gen.Timezone != "Asia/Shanghai" {
		t.Fatalf("generation %+v", gen)
	}
	vals, err := sqlcgen.New(db).ListPortfolioValuationProjections(context.Background(), mustBin(active))
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) == 0 {
		t.Fatal("expected a valuation row")
	}
	foundMissing := false
	for _, item := range vals {
		if item.Completeness != "complete" || !item.Amount.Valid {
			foundMissing = true
			break
		}
	}
	if !foundMissing {
		t.Fatalf("expected missing empty valuation got %+v", vals)
	}
}

func TestRebuildKeepsPreviousGenerationOnUnreadyActivate(t *testing.T) {
	db := mysql.OpenTestDB(t)
	portfolioID, jobID := setupPortfolioJob(t, db)
	svc := NewService(db)
	if err := svc.Rebuild(context.Background(), Request{PortfolioID: portfolioID, JobID: jobID}); err != nil {
		t.Fatal(err)
	}
	first, err := jobs.NewService(db).ActiveGenerationID(context.Background(), portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	building, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := sqlcgen.New(db).InsertProjectionGeneration(context.Background(), sqlcgen.InsertProjectionGenerationParams{
		ID:                mustBin(building),
		PortfolioID:       mustBin(portfolioID),
		JobID:             mustBin(jobID),
		Status:            "building",
		Timezone:          "UTC",
		DisplayAssetID:    mustBin(catalog.AssetUSD),
		SourceWatermarkAt: now,
		StartedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}
	err = svc.activate(context.Background(), portfolioID, building)
	if err == nil {
		t.Fatal("expected unready activation to fail")
	}
	active, err := jobs.NewService(db).ActiveGenerationID(context.Background(), portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	if active != first {
		t.Fatalf("pointer moved from %s to %s", first, active)
	}
}

func TestRebuildUsesReportedEquityAndCapturesTimezone(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := context.Background()
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	user, err := authSvc.CreateUser(ctx, "pj-"+uuid.NewString()[:8], "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ports := portfolio.NewService(db)
	items, err := ports.List(ctx, user.ID)
	if err != nil || len(items) == 0 {
		t.Fatal(err)
	}
	portfolioID := items[0].ID
	accessSvc := access.NewService(db, 15*time.Minute, 30*time.Minute, nil)
	ident, err := accessSvc.CreateIdentity(ctx, portfolioID, &access.CreateIdentityInput{
		PlatformID:  catalog.PlatformBinance,
		ExternalRef: "uid-" + uuid.NewString()[:8],
		DisplayName: "Binance",
		AccountMode: catalog.AccountModeBinancePM,
		Kind:        "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := accessSvc.CreateConnection(ctx, portfolioID, &access.CreateConnectionInput{
		IdentityID: ident.ID,
		Label:      "primary",
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := observation.NewService(db)
	account, err := obs.CreateAccount(ctx, portfolioID, &observation.CreateAccountInput{
		IdentityID:  ident.ID,
		ExternalRef: "unified",
		DisplayName: "Unified",
		AccountType: "unified",
		Status:      "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	listingID := insertMappedUSDListing(t, db, portfolioID, user.ID)
	snapshot, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID:     ident.ID,
		ConnectionID:   conn.ID,
		AdapterName:    "test",
		AdapterVersion: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	amount := "1250.50"
	observed := time.Date(2026, 9, 18, 16, 0, 0, 0, time.UTC)
	if _, err := obs.Create(ctx, portfolioID, &observation.CreateInput{
		IdentitySnapshotID: snapshot.ID,
		AccountID:          &account.ID,
		Kind:               "reported_equity",
		SourceObservedAt:   observed,
		Completeness:       "complete",
		ReportedEquity: &observation.ReportedEquity{
			DenominationListingID: listingID,
			Amount:                amount,
			CoveredIdentityIDs:    []uuid.UUID{ident.ID},
			CoveredAccountIDs:     []uuid.UUID{account.ID},
		},
	}); err != nil {
		t.Fatal(err)
	}
	job, err := jobs.NewService(db).EnqueueRebuild(ctx, portfolioID, &user.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)
	svc.now = func() time.Time { return time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC) }
	if err := svc.Rebuild(ctx, Request{PortfolioID: portfolioID, JobID: job.ID}); err != nil {
		t.Fatal(err)
	}
	active, err := jobs.NewService(db).ActiveGenerationID(ctx, portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	vals, err := sqlcgen.New(db).ListPortfolioValuationProjections(ctx, mustBin(active))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range vals {
		if item.Amount.Valid && compactDecimal(item.Amount.String) == "1250.5" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing reported equity valuation %+v", vals)
	}
}

func setupPortfolioJob(t *testing.T, db *sql.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	user, err := authSvc.CreateUser(context.Background(), "pe-"+uuid.NewString()[:8], "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	items, err := portfolio.NewService(db).List(context.Background(), user.ID)
	if err != nil || len(items) == 0 {
		t.Fatal(err)
	}
	job, err := jobs.NewService(db).EnqueueRebuild(context.Background(), items[0].ID, &user.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	return items[0].ID, job.ID
}

func insertMappedUSDListing(t *testing.T, db *sql.DB, portfolioID, userID uuid.UUID) uuid.UUID {
	t.Helper()
	listingID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	mappingID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(
		`INSERT INTO asset_listings (id, platform_id, external_ref, symbol, name, status, first_seen_at, last_seen_at, created_at, updated_at)
		 VALUES (?, ?, ?, 'USD', 'US Dollar', 'active', ?, ?, ?, ?)`,
		listingID[:], catalog.PlatformBinance[:], "usd-"+listingID.String(), now, now, now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO asset_listing_mappings (id, portfolio_id, asset_listing_id, current_revision_id, version, created_at, updated_at)
		 VALUES (?, ?, ?, NULL, 1, ?, ?)`,
		mappingID[:], portfolioID[:], listingID[:], now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO asset_listing_mapping_revisions (id, portfolio_id, mapping_id, asset_id, state, created_by_user_id, created_at)
		 VALUES (?, ?, ?, ?, 'confirmed', ?, ?)`,
		revisionID[:], portfolioID[:], mappingID[:], catalog.AssetUSD[:], userID[:], now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE asset_listing_mappings SET current_revision_id = ? WHERE id = ?`, revisionID[:], mappingID[:]); err != nil {
		t.Fatal(err)
	}
	return listingID
}
