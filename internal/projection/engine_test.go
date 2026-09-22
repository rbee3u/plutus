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
	if _, err := db.Exec(`UPDATE identities SET created_at = ? WHERE id = ?`, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), ident.ID[:]); err != nil {
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
	amount := "1250.50"
	observed := time.Date(2026, 9, 18, 16, 0, 0, 0, time.UTC)
	snapshot, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID:     ident.ID,
		ConnectionID:   conn.ID,
		AdapterName:    "test",
		AdapterVersion: "1",
		StartedAt:      observed,
	})
	if err != nil {
		t.Fatal(err)
	}
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

func TestSnapshotReplacesOmittedPositionAndCarriesFailedScope(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := context.Background()
	portfolioID, userID, ident, conn, account := setupObservedIdentity(t, db)
	instrumentID := insertPerpetual(t, db)
	obs := observation.NewService(db)
	firstAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	secondAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	failedAt := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	first, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID: ident.ID, ConnectionID: conn.ID, AdapterName: "test", AdapterVersion: "1", StartedAt: firstAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := obs.Create(ctx, portfolioID, &observation.CreateInput{
		IdentitySnapshotID: first.ID, AccountID: &account.ID, Kind: "position", SourceObservedAt: firstAt, Completeness: "complete",
		Position: &observation.Position{InstrumentID: instrumentID, PlatformPositionRef: "pos-x", Side: "long", Quantity: "2"},
	}); err != nil {
		t.Fatal(err)
	}
	second, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID: ident.ID, ConnectionID: conn.ID, AdapterName: "test", AdapterVersion: "1", StartedAt: secondAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := obs.Create(ctx, portfolioID, &observation.CreateInput{
		IdentitySnapshotID: second.ID, AccountID: &account.ID, Kind: "position", SourceObservedAt: secondAt, Completeness: "complete",
		Position: &observation.Position{InstrumentID: instrumentID, PlatformPositionRef: "pos-y", Side: "short", Quantity: "1"},
	}); err != nil {
		t.Fatal(err)
	}
	third, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID: ident.ID, ConnectionID: conn.ID, AdapterName: "test", AdapterVersion: "1", StartedAt: failedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := obs.RecordFailure(ctx, portfolioID, &observation.FailureInput{
		IdentitySnapshotID: third.ID, AccountID: &account.ID, ScopeKind: "position", Phase: "collect",
		ErrorClass: "temporary_platform_failure", ErrorCode: "position_unavailable", SafeDetail: "position list failed", OccurredAt: failedAt,
	}); err != nil {
		t.Fatal(err)
	}
	rebuildAt(t, db, portfolioID, userID, failedAt.Add(time.Hour))
	active := activeGeneration(t, db, portfolioID)
	positions, err := sqlcgen.New(db).ListLatestPositionProjections(ctx, mustBin(active))
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 || positions[0].PlatformPositionRef != "pos-y" || positions[0].FreshnessState != "stale" {
		t.Fatalf("positions %+v", positions)
	}
}

func TestArchiveDropsCurrentTotalsAndKeepsEarlierCutoff(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := context.Background()
	portfolioID, userID, ident, conn, account := setupObservedIdentity(t, db)
	listingID := insertMappedUSDListing(t, db, portfolioID, userID)
	obs := observation.NewService(db)
	observed := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	snapshot, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID: ident.ID, ConnectionID: conn.ID, AdapterName: "test", AdapterVersion: "1", StartedAt: observed,
	})
	if err != nil {
		t.Fatal(err)
	}
	total := "10"
	if _, err := obs.Create(ctx, portfolioID, &observation.CreateInput{
		IdentitySnapshotID: snapshot.ID, AccountID: &account.ID, Kind: "balance", SourceObservedAt: observed, Completeness: "complete",
		Balance: &observation.Balance{AssetListingID: listingID, TotalQuantity: &total},
	}); err != nil {
		t.Fatal(err)
	}
	accessSvc := access.NewService(db, 15*time.Minute, 30*time.Minute, nil)
	if _, err := accessSvc.CreateIdentity(ctx, portfolioID, &access.CreateIdentityInput{
		PlatformID: catalog.PlatformOKX, ExternalRef: "uid-empty", DisplayName: "Empty", AccountMode: catalog.AccountModeOKXMCM, Kind: "main",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := accessSvc.ArchiveIdentity(ctx, portfolioID, ident.ID, ident.Version); err != nil {
		t.Fatal(err)
	}
	listed, err := accessSvc.ListIdentities(ctx, portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range listed {
		if item.ID == ident.ID {
			t.Fatal("archived identity is still listed")
		}
	}
	asOf := time.Now().UTC().Add(time.Hour)
	rebuildAt(t, db, portfolioID, userID, asOf)
	vals, err := sqlcgen.New(db).ListPortfolioValuationProjections(ctx, mustBin(activeGeneration(t, db, portfolioID)))
	if err != nil {
		t.Fatal(err)
	}
	var historical, current *sqlcgen.PortfolioValuationProjection
	for i := range vals {
		item := &vals[i]
		if item.EvaluatedAt.Equal(observed) {
			historical = item
		}
		if item.EvaluatedAt.Equal(asOf) {
			current = item
		}
	}
	if historical == nil || !historical.Amount.Valid || compactDecimal(historical.Amount.String) != "10" || historical.Completeness != "complete" {
		t.Fatalf("historical %+v", historical)
	}
	if current == nil || current.Completeness == "complete" {
		t.Fatalf("current %+v", current)
	}
	balances, err := sqlcgen.New(db).ListLatestBalanceProjections(ctx, mustBin(activeGeneration(t, db, portfolioID)))
	if err != nil {
		t.Fatal(err)
	}
	if len(balances) != 0 {
		t.Fatalf("latest balances %+v", balances)
	}
}

func TestOmittedPositionIsNotCarried(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := context.Background()
	portfolioID, userID, ident, conn, account := setupObservedIdentity(t, db)
	instrumentID := insertPerpetual(t, db)
	obs := observation.NewService(db)
	firstAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	secondAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	first, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID: ident.ID, ConnectionID: conn.ID, AdapterName: "test", AdapterVersion: "1", StartedAt: firstAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := obs.Create(ctx, portfolioID, &observation.CreateInput{
		IdentitySnapshotID: first.ID, AccountID: &account.ID, Kind: "position", SourceObservedAt: firstAt, Completeness: "complete",
		Position: &observation.Position{InstrumentID: instrumentID, PlatformPositionRef: "pos-x", Side: "long", Quantity: "2"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := obs.CreateSnapshot(ctx, portfolioID, &observation.CreateSnapshotInput{
		IdentityID: ident.ID, ConnectionID: conn.ID, AdapterName: "test", AdapterVersion: "1", StartedAt: secondAt,
	}); err != nil {
		t.Fatal(err)
	}
	rebuildAt(t, db, portfolioID, userID, secondAt.Add(time.Hour))
	positions, err := sqlcgen.New(db).ListLatestPositionProjections(ctx, mustBin(activeGeneration(t, db, portfolioID)))
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 0 {
		t.Fatalf("closed position carried %+v", positions)
	}
}

func setupObservedIdentity(t *testing.T, db *sql.DB) (uuid.UUID, uuid.UUID, access.Identity, access.Connection, observation.Account) {
	t.Helper()
	ctx := context.Background()
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	user, err := authSvc.CreateUser(ctx, "sn-"+uuid.NewString()[:8], "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	items, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil || len(items) == 0 {
		t.Fatal(err)
	}
	accessSvc := access.NewService(db, 15*time.Minute, 30*time.Minute, nil)
	ident, err := accessSvc.CreateIdentity(ctx, items[0].ID, &access.CreateIdentityInput{
		PlatformID: catalog.PlatformBinance, ExternalRef: "uid-" + uuid.NewString()[:8], DisplayName: "Binance", AccountMode: catalog.AccountModeBinancePM, Kind: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE identities SET created_at = ? WHERE id = ?`, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), ident.ID[:]); err != nil {
		t.Fatal(err)
	}
	conn, err := accessSvc.CreateConnection(ctx, items[0].ID, &access.CreateConnectionInput{IdentityID: ident.ID, Label: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	account, err := observation.NewService(db).CreateAccount(ctx, items[0].ID, &observation.CreateAccountInput{
		IdentityID: ident.ID, ExternalRef: "unified", DisplayName: "Unified", AccountType: "unified", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	return items[0].ID, user.ID, ident, conn, account
}

func insertPerpetual(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(
		`INSERT INTO instruments (id, platform_id, external_ref, symbol, kind, status, first_seen_at, last_seen_at, created_at, updated_at)
		 VALUES (?, ?, ?, 'BTCUSDT', 'linear_perpetual', 'active', ?, ?, ?, ?)`,
		id[:], catalog.PlatformBinance[:], "inst-"+id.String(), now, now, now, now,
	); err != nil {
		t.Fatal(err)
	}
	return id
}

func rebuildAt(t *testing.T, db *sql.DB, portfolioID, userID uuid.UUID, asOf time.Time) {
	t.Helper()
	job, err := jobs.NewService(db).EnqueueRebuild(context.Background(), portfolioID, &userID, "test")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)
	svc.now = func() time.Time { return asOf }
	if err := svc.Rebuild(context.Background(), Request{PortfolioID: portfolioID, JobID: job.ID}); err != nil {
		t.Fatal(err)
	}
}

func activeGeneration(t *testing.T, db *sql.DB, portfolioID uuid.UUID) uuid.UUID {
	t.Helper()
	active, err := jobs.NewService(db).ActiveGenerationID(context.Background(), portfolioID)
	if err != nil || active == uuid.Nil {
		t.Fatalf("active %s %v", active, err)
	}
	return active
}
