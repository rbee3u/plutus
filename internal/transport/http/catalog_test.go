package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/observation"
	"github.com/rbee3u/plutus/internal/portfolio"
)

func TestCatalogListingMappingAndPins(t *testing.T) {
	db := mysql.OpenTestDB(t)
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	engine := Engine(&EngineOptions{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:        StaticReady{},
		Auth:         authSvc,
		Portfolios:   portfolio.NewService(db),
		Access:       access.NewService(db, 15*time.Minute, 30*time.Minute, nil),
		Catalog:      catalog.NewService(db),
		Observations: observation.NewService(db),
		Idempotency:  idempotency.NewStore(db),
		Jobs:         jobs.NewService(db),
	})

	password := "correct horse battery staple"
	userA := "ct-" + uuid.NewString()[:8]
	userB := "cu-" + uuid.NewString()[:8]
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
	listingID := insertTestListing(t, db)
	instrumentID := insertTestInstrument(t, db)
	base := "/api/v1/portfolios/" + portfolioID + "/catalog"
	listingPath := base + "/asset-listings/" + listingID.String() + "/mapping"
	listingPinPath := base + "/pins/listing/" + listingID.String()
	instrumentPinPath := base + "/pins/instrument/" + instrumentID.String()

	empty := doJSON(t, engine, http.MethodGet, base+"/asset-listings?q="+listingID.String(), nil, sessionA, "")
	if empty.Code != http.StatusOK {
		t.Fatalf("empty search %d %s", empty.Code, empty.Body.String())
	}

	listings := doJSON(t, engine, http.MethodGet, base+"/asset-listings?q=USDT&mappingState=unmapped", nil, sessionA, "")
	if listings.Code != http.StatusOK {
		t.Fatalf("listings %d %s", listings.Code, listings.Body.String())
	}
	var listingBody struct {
		Items []struct {
			ID           string  `json:"id"`
			MappingState string  `json:"mappingState"`
			Pinned       bool    `json:"pinned"`
			PlatformCode *string `json:"platformCode"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listings.Body.Bytes(), &listingBody); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range listingBody.Items {
		if item.ID == listingID.String() {
			found = true
			if item.MappingState != "unmapped" || item.Pinned || item.PlatformCode == nil || *item.PlatformCode != "binance" {
				t.Fatalf("unmapped listing %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("inserted listing missing")
	}

	foreign := doJSON(t, engine, http.MethodGet, base+"/asset-listings", nil, sessionB, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-user listings %d %s", foreign.Code, foreign.Body.String())
	}

	missing := jsonWithMatch(t, engine, http.MethodPut, base+"/asset-listings/"+uuid.NewString()+"/mapping", map[string]any{
		"assetId": catalog.AssetUSD.String(),
		"state":   "confirmed",
	}, sessionA, "*", "map-missing")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing listing %d %s", missing.Code, missing.Body.String())
	}

	invalidAsset := jsonWithMatch(t, engine, http.MethodPut, listingPath, map[string]any{
		"assetId": uuid.NewString(),
		"state":   "confirmed",
	}, sessionA, "*", "map-bad-asset")
	if invalidAsset.Code != http.StatusBadRequest {
		t.Fatalf("invalid asset %d %s", invalidAsset.Code, invalidAsset.Body.String())
	}

	created := jsonWithMatch(t, engine, http.MethodPut, listingPath, map[string]any{
		"assetId": catalog.AssetUSD.String(),
		"state":   "confirmed",
		"reason":  "seed mapping",
	}, sessionA, "*", "map-create")
	if created.Code != http.StatusCreated {
		t.Fatalf("create mapping %d %s", created.Code, created.Body.String())
	}
	if created.Header().Get("ETag") != `"listing-mapping:1"` {
		t.Fatalf("create etag %s", created.Header().Get("ETag"))
	}
	var createdBody struct {
		Mapping struct {
			State   string `json:"state"`
			Version int    `json:"version"`
			AssetID string `json:"assetId"`
		} `json:"mapping"`
		Job struct {
			Kind string `json:"kind"`
		} `json:"job"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatal(err)
	}
	if createdBody.Mapping.State != "confirmed" || createdBody.Mapping.Version != 1 || createdBody.Mapping.AssetID != catalog.AssetUSD.String() || createdBody.Job.Kind != "projection" {
		t.Fatalf("created body %+v", createdBody)
	}

	replay := jsonWithMatch(t, engine, http.MethodPut, listingPath, map[string]any{
		"assetId": catalog.AssetUSD.String(),
		"state":   "confirmed",
		"reason":  "seed mapping",
	}, sessionA, "*", "map-create")
	if replay.Code != http.StatusCreated {
		t.Fatalf("idempotent create %d %s", replay.Code, replay.Body.String())
	}

	starAgain := jsonWithMatch(t, engine, http.MethodPut, listingPath, map[string]any{
		"assetId": catalog.AssetUSD.String(),
		"state":   "provisional",
	}, sessionA, "*", "map-star-again")
	if starAgain.Code != http.StatusConflict {
		t.Fatalf("star exists %d %s", starAgain.Code, starAgain.Body.String())
	}

	revised := jsonWithMatch(t, engine, http.MethodPut, listingPath, map[string]any{
		"assetId": catalog.AssetUSD.String(),
		"state":   "provisional",
	}, sessionA, `"listing-mapping:1"`, "map-revise")
	if revised.Code != http.StatusOK {
		t.Fatalf("revise mapping %d %s", revised.Code, revised.Body.String())
	}
	if revised.Header().Get("ETag") != `"listing-mapping:2"` {
		t.Fatalf("revise etag %s", revised.Header().Get("ETag"))
	}

	stale := jsonWithMatch(t, engine, http.MethodPut, listingPath, map[string]any{
		"assetId": catalog.AssetUSD.String(),
		"state":   "confirmed",
	}, sessionA, `"listing-mapping:1"`, "map-stale")
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale mapping %d %s", stale.Code, stale.Body.String())
	}

	confirmed := doJSON(t, engine, http.MethodGet, base+"/asset-listings?q=USDT&mappingState=provisional", nil, sessionA, "")
	if confirmed.Code != http.StatusOK {
		t.Fatalf("provisional search %d %s", confirmed.Code, confirmed.Body.String())
	}

	invalidKind := jsonWithMatch(t, engine, http.MethodPut, base+"/pins/account/"+listingID.String(), map[string]any{}, sessionA, "*", "pin-kind")
	if invalidKind.Code != http.StatusBadRequest {
		t.Fatalf("invalid subject %d %s", invalidKind.Code, invalidKind.Body.String())
	}

	pinCreated := jsonWithMatch(t, engine, http.MethodPut, listingPinPath, map[string]any{"reason": "track listing"}, sessionA, "*", "pin-listing")
	if pinCreated.Code != http.StatusCreated {
		t.Fatalf("create listing pin %d %s", pinCreated.Code, pinCreated.Body.String())
	}
	if pinCreated.Header().Get("ETag") != `"catalog-pin:1"` {
		t.Fatalf("pin etag %s", pinCreated.Header().Get("ETag"))
	}
	var pinBody struct {
		Pin struct {
			Enabled     bool   `json:"enabled"`
			SubjectKind string `json:"subjectKind"`
			Version     int    `json:"version"`
		} `json:"pin"`
		Job struct {
			Kind string `json:"kind"`
		} `json:"job"`
	}
	if err := json.Unmarshal(pinCreated.Body.Bytes(), &pinBody); err != nil {
		t.Fatal(err)
	}
	if !pinBody.Pin.Enabled || pinBody.Pin.SubjectKind != "listing" || pinBody.Pin.Version != 1 || pinBody.Job.Kind != "projection" {
		t.Fatalf("pin body %+v", pinBody)
	}

	pinnedListings := doJSON(t, engine, http.MethodGet, base+"/asset-listings?q=USDT", nil, sessionA, "")
	if pinnedListings.Code != http.StatusOK {
		t.Fatalf("pinned listings %d %s", pinnedListings.Code, pinnedListings.Body.String())
	}
	if err := json.Unmarshal(pinnedListings.Body.Bytes(), &listingBody); err != nil {
		t.Fatal(err)
	}
	found = false
	for _, item := range listingBody.Items {
		if item.ID == listingID.String() {
			found = true
			if !item.Pinned {
				t.Fatal("listing should be pinned")
			}
		}
	}
	if !found {
		t.Fatal("pinned listing missing")
	}

	instrumentPin := jsonWithMatch(t, engine, http.MethodPut, instrumentPinPath, map[string]any{}, sessionA, "*", "pin-instrument")
	if instrumentPin.Code != http.StatusCreated {
		t.Fatalf("create instrument pin %d %s", instrumentPin.Code, instrumentPin.Body.String())
	}

	instruments := doJSON(t, engine, http.MethodGet, base+"/instruments?pinned=true", nil, sessionA, "")
	if instruments.Code != http.StatusOK {
		t.Fatalf("instruments %d %s", instruments.Code, instruments.Body.String())
	}
	var instrumentBody struct {
		Items []struct {
			ID     string `json:"id"`
			Pinned bool   `json:"pinned"`
			Kind   string `json:"kind"`
		} `json:"items"`
	}
	if err := json.Unmarshal(instruments.Body.Bytes(), &instrumentBody); err != nil {
		t.Fatal(err)
	}
	found = false
	for _, item := range instrumentBody.Items {
		if item.ID == instrumentID.String() {
			found = true
			if !item.Pinned || item.Kind != "linear_perpetual" {
				t.Fatalf("instrument %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("pinned instrument missing")
	}

	disabled := jsonWithMatch(t, engine, http.MethodDelete, listingPinPath, nil, sessionA, `"catalog-pin:1"`, "")
	if disabled.Code != http.StatusOK {
		t.Fatalf("disable pin %d %s", disabled.Code, disabled.Body.String())
	}
	if disabled.Header().Get("ETag") != `"catalog-pin:2"` {
		t.Fatalf("disable etag %s", disabled.Header().Get("ETag"))
	}
	var disabledBody struct {
		Pin struct {
			Enabled bool `json:"enabled"`
			Version int  `json:"version"`
		} `json:"pin"`
	}
	if err := json.Unmarshal(disabled.Body.Bytes(), &disabledBody); err != nil {
		t.Fatal(err)
	}
	if disabledBody.Pin.Enabled || disabledBody.Pin.Version != 2 {
		t.Fatalf("disabled pin %+v", disabledBody)
	}

	missingPin := jsonWithMatch(t, engine, http.MethodDelete, base+"/pins/listing/"+uuid.NewString(), nil, sessionA, `"catalog-pin:1"`, "")
	if missingPin.Code != http.StatusNotFound {
		t.Fatalf("missing pin %d %s", missingPin.Code, missingPin.Body.String())
	}
}

func insertTestInstrument(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = db.Exec(
		`INSERT INTO instruments (id, platform_id, external_ref, symbol, kind, status, first_seen_at, last_seen_at, created_at, updated_at)
		 VALUES (?, ?, ?, 'BTCUSDT', 'linear_perpetual', 'active', ?, ?, ?, ?)`,
		id[:], catalog.PlatformBinance[:], "inst-"+id.String(), now, now, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
