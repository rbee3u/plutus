package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/observation"
	"github.com/rbee3u/plutus/internal/portfolio"
)

func TestAccountsCatalogAndObservationCorrection(t *testing.T) {
	db := mysql.OpenTestDB(t)
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	accessSvc := access.NewService(db, 15*time.Minute, 30*time.Minute, nil)
	catalogSvc := catalog.NewService(db)
	obsSvc := observation.NewService(db)
	jobSvc := jobs.NewService(db)
	engine := Engine(&EngineOptions{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:        StaticReady{},
		Auth:         authSvc,
		Portfolios:   portfolios,
		Access:       accessSvc,
		Catalog:      catalogSvc,
		Observations: obsSvc,
		Idempotency:  idempotency.NewStore(db),
		Jobs:         jobSvc,
		Audit:        audit.NewService(db),
	})

	password := "correct horse battery staple"
	userA := "fa-" + uuid.NewString()[:8]
	userB := "fb-" + uuid.NewString()[:8]
	if _, err := authSvc.CreateUser(context.Background(), userA, password); err != nil {
		t.Fatal(err)
	}
	if _, err := authSvc.CreateUser(context.Background(), userB, password); err != nil {
		t.Fatal(err)
	}
	sessionA := loginSession(t, engine, userA, password)
	sessionB := loginSession(t, engine, userB, password)

	list := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios", nil, sessionA, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	portfolioID := listed.Items[0].ID
	parsedPortfolio := uuid.MustParse(portfolioID)

	assets := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/catalog/assets?q=USD", nil, sessionA, "")
	if assets.Code != http.StatusOK {
		t.Fatalf("assets %d %s", assets.Code, assets.Body.String())
	}
	var assetList struct {
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	if err := json.Unmarshal(assets.Body.Bytes(), &assetList); err != nil {
		t.Fatal(err)
	}
	if assetList.Page.Total < 1 || assetList.Items[0].Code != "USD" {
		t.Fatalf("assets %+v", assetList)
	}

	ident := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+portfolioID+"/identities", map[string]any{
		"platformId":  catalog.PlatformBinance.String(),
		"externalRef": "acct-" + uuid.NewString()[:8],
		"displayName": "Binance",
		"accountMode": "binance_pm",
		"kind":        "main",
	}, sessionA, "")
	if ident.Code != http.StatusCreated {
		t.Fatalf("identity %d %s", ident.Code, ident.Body.String())
	}
	var identBody struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(ident.Body.Bytes(), &identBody); err != nil {
		t.Fatal(err)
	}
	identityID := uuid.MustParse(identBody.ID)
	identETag := ident.Header().Get("ETag")

	renamed := patchWithMatch(t, engine, "/api/v1/portfolios/"+portfolioID+"/identities/"+identBody.ID, map[string]any{
		"externalRef": "moved-before-facts",
	}, sessionA, identETag)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename before facts %d %s", renamed.Code, renamed.Body.String())
	}
	identETag = `"identity:2"`

	conn := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+portfolioID+"/connections", map[string]any{
		"identityId": identBody.ID,
		"label":      "primary",
	}, sessionA, "")
	if conn.Code != http.StatusCreated {
		t.Fatalf("connection %d %s", conn.Code, conn.Body.String())
	}
	var connBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(conn.Body.Bytes(), &connBody); err != nil {
		t.Fatal(err)
	}

	account, err := obsSvc.CreateAccount(context.Background(), parsedPortfolio, &observation.CreateAccountInput{
		IdentityID:  identityID,
		ExternalRef: "unified",
		DisplayName: "Unified",
		AccountType: "unified",
		Status:      "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	accounts := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/accounts", nil, sessionA, "")
	if accounts.Code != http.StatusOK {
		t.Fatalf("accounts %d %s", accounts.Code, accounts.Body.String())
	}
	var accountList struct {
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	if err := json.Unmarshal(accounts.Body.Bytes(), &accountList); err != nil {
		t.Fatal(err)
	}
	if accountList.Page.Total != 1 {
		t.Fatalf("account total %d", accountList.Page.Total)
	}

	listingID := insertTestListing(t, db)
	snapshot, err := obsSvc.CreateSnapshot(context.Background(), parsedPortfolio, &observation.CreateSnapshotInput{
		IdentityID:     identityID,
		ConnectionID:   uuid.MustParse(connBody.ID),
		AdapterName:    "test",
		AdapterVersion: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := obsSvc.RecordFailure(context.Background(), parsedPortfolio, &observation.FailureInput{
		IdentitySnapshotID: snapshot.ID,
		ScopeKind:          "price",
		Phase:              "collect",
		ErrorClass:         "temporary_platform_failure",
		ErrorCode:          "timeout",
		SafeDetail:         "price book timed out",
		Retriable:          true,
	}); err != nil {
		t.Fatal(err)
	}
	total := "12.5"
	obs, err := obsSvc.Create(context.Background(), parsedPortfolio, &observation.CreateInput{
		IdentitySnapshotID: snapshot.ID,
		AccountID:          &account.ID,
		Kind:               "balance",
		Completeness:       "complete",
		Balance: &observation.Balance{
			AssetListingID: listingID,
			TotalQuantity:  &total,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	frozen := patchWithMatch(t, engine, "/api/v1/portfolios/"+portfolioID+"/identities/"+identBody.ID, map[string]any{
		"externalRef": "should-fail",
	}, sessionA, identETag)
	if frozen.Code != http.StatusConflict {
		t.Fatalf("frozen %d %s", frozen.Code, frozen.Body.String())
	}
	display := patchWithMatch(t, engine, "/api/v1/portfolios/"+portfolioID+"/identities/"+identBody.ID, map[string]any{
		"displayName": "Still editable",
	}, sessionA, identETag)
	if display.Code != http.StatusOK {
		t.Fatalf("display %d %s", display.Code, display.Body.String())
	}

	corrected := patchWithMatch(t, engine, "/api/v1/portfolios/"+portfolioID+"/observations/"+obs.ID.String(), map[string]any{
		"totalQuantity": "15.00",
	}, sessionA, obs.ETag())
	if corrected.Code != http.StatusOK {
		t.Fatalf("correct %d %s", corrected.Code, corrected.Body.String())
	}
	var body struct {
		Observation struct {
			Version int `json:"version"`
			Balance *struct {
				TotalQuantity *string `json:"totalQuantity"`
			} `json:"balance"`
		} `json:"observation"`
		Job struct {
			Kind   string `json:"kind"`
			Status string `json:"status"`
		} `json:"job"`
	}
	if err := json.Unmarshal(corrected.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Observation.Version != 2 || body.Observation.Balance == nil || body.Observation.Balance.TotalQuantity == nil || *body.Observation.Balance.TotalQuantity != "15" {
		t.Fatalf("corrected %+v", body)
	}
	if body.Job.Kind != "projection" {
		t.Fatalf("job %+v", body.Job)
	}

	stale := patchWithMatch(t, engine, "/api/v1/portfolios/"+portfolioID+"/observations/"+obs.ID.String(), map[string]any{
		"totalQuantity": "16.00",
	}, sessionA, obs.ETag())
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale %d %s", stale.Code, stale.Body.String())
	}

	foreign := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/accounts", nil, sessionB, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-user %d %s", foreign.Code, foreign.Body.String())
	}

	listedObs := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/observations?kind=balance", nil, sessionA, "")
	if listedObs.Code != http.StatusOK {
		t.Fatalf("list observations %d %s", listedObs.Code, listedObs.Body.String())
	}
	var obsList struct {
		Items []struct {
			ID      string `json:"id"`
			Kind    string `json:"kind"`
			Summary string `json:"summary"`
			Version int    `json:"version"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listedObs.Body.Bytes(), &obsList); err != nil {
		t.Fatal(err)
	}
	if len(obsList.Items) != 1 || obsList.Items[0].ID != obs.ID.String() || obsList.Items[0].Kind != "balance" || obsList.Items[0].Summary != "Balance" || obsList.Items[0].Version != 2 {
		t.Fatalf("list observations %+v", obsList)
	}

	got := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/observations/"+obs.ID.String(), nil, sessionA, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get observation %d %s", got.Code, got.Body.String())
	}
	if got.Header().Get("ETag") != `"observation:2"` {
		t.Fatalf("get etag %s", got.Header().Get("ETag"))
	}
	missingObs := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/observations/"+uuid.NewString(), nil, sessionA, "")
	if missingObs.Code != http.StatusNotFound {
		t.Fatalf("missing observation %d %s", missingObs.Code, missingObs.Body.String())
	}
	foreignObs := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/observations/"+obs.ID.String(), nil, sessionB, "")
	if foreignObs.Code != http.StatusNotFound {
		t.Fatalf("cross-user observation %d %s", foreignObs.Code, foreignObs.Body.String())
	}

	audits := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/audit-events?action=observation.correct", nil, sessionA, "")
	if audits.Code != http.StatusOK {
		t.Fatalf("audit %d %s", audits.Code, audits.Body.String())
	}
	if !strings.Contains(audits.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("audit missing no-store")
	}
	var auditList struct {
		Items []struct {
			Action       string         `json:"action"`
			ResourceType string         `json:"resourceType"`
			After        map[string]any `json:"after"`
		} `json:"items"`
	}
	if err := json.Unmarshal(audits.Body.Bytes(), &auditList); err != nil {
		t.Fatal(err)
	}
	if len(auditList.Items) != 1 || auditList.Items[0].Action != "observation.correct" || auditList.Items[0].ResourceType != "observation" || auditList.Items[0].After == nil {
		t.Fatalf("audit %+v", auditList)
	}
	badCursor := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/audit-events?cursor=nope", nil, sessionA, "")
	if badCursor.Code != http.StatusBadRequest {
		t.Fatalf("bad audit cursor %d %s", badCursor.Code, badCursor.Body.String())
	}
	foreignAudit := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/audit-events", nil, sessionB, "")
	if foreignAudit.Code != http.StatusNotFound {
		t.Fatalf("cross-user audit %d %s", foreignAudit.Code, foreignAudit.Body.String())
	}
}

func insertTestListing(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = db.Exec(
		`INSERT INTO asset_listings (id, platform_id, external_ref, symbol, name, status, first_seen_at, last_seen_at, created_at, updated_at)
		 VALUES (?, ?, ?, 'USDT', 'Tether', 'active', ?, ?, ?, ?)`,
		id[:], catalog.PlatformBinance[:], "ext-"+id.String(), now, now, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
