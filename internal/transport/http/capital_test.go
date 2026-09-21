package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/observation"
	"github.com/rbee3u/plutus/internal/portfolio"
)

func TestCapitalBaselineAndFlowRevisions(t *testing.T) {
	db := mysql.OpenTestDB(t)
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	accessSvc := access.NewService(db, 15*time.Minute, 30*time.Minute, nil)
	engine := Engine(&EngineOptions{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:        StaticReady{},
		Auth:         authSvc,
		Portfolios:   portfolios,
		Access:       accessSvc,
		Catalog:      catalog.NewService(db),
		Observations: observation.NewService(db),
		Capital:      capital.NewService(db),
		Idempotency:  idempotency.NewStore(db),
		Jobs:         jobs.NewService(db),
	})

	password := "correct horse battery staple"
	userA := "ca-" + uuid.NewString()[:8]
	userB := "cb-" + uuid.NewString()[:8]
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
	basePath := "/api/v1/portfolios/" + portfolioID + "/capital-baseline"
	flowPath := "/api/v1/portfolios/" + portfolioID + "/capital-flows"
	assetID := catalog.AssetUSD.String()
	effectiveAt := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

	missing := doJSON(t, engine, http.MethodGet, basePath, nil, sessionA, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing baseline %d %s", missing.Code, missing.Body.String())
	}
	foreign := doJSON(t, engine, http.MethodGet, basePath, nil, sessionB, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-user baseline %d %s", foreign.Code, foreign.Body.String())
	}

	created := jsonWithMatch(t, engine, http.MethodPut, basePath, map[string]any{
		"effectiveAt": effectiveAt,
		"assetId":     assetID,
		"quantity":    "100.50",
		"note":        "opening",
	}, sessionA, "*", "cap-base-1")
	if created.Code != http.StatusCreated {
		t.Fatalf("create baseline %d %s", created.Code, created.Body.String())
	}
	if created.Header().Get("ETag") != `"capital-baseline:1"` {
		t.Fatalf("create etag %s", created.Header().Get("ETag"))
	}
	var createdBody struct {
		Baseline struct {
			Quantity string `json:"quantity"`
			Version  int    `json:"version"`
		} `json:"baseline"`
		Job struct {
			Kind string `json:"kind"`
		} `json:"job"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatal(err)
	}
	if createdBody.Baseline.Quantity != "100.5" || createdBody.Baseline.Version != 1 || createdBody.Job.Kind != "projection" {
		t.Fatalf("created body %+v", createdBody)
	}
	replay := jsonWithMatch(t, engine, http.MethodPut, basePath, map[string]any{
		"effectiveAt": effectiveAt,
		"assetId":     assetID,
		"quantity":    "100.50",
		"note":        "opening",
	}, sessionA, "*", "cap-base-1")
	if replay.Code != http.StatusCreated {
		t.Fatalf("idempotent create %d %s", replay.Code, replay.Body.String())
	}

	starAgain := jsonWithMatch(t, engine, http.MethodPut, basePath, map[string]any{
		"effectiveAt": effectiveAt,
		"assetId":     assetID,
		"quantity":    "200",
	}, sessionA, "*", "cap-base-star")
	if starAgain.Code != http.StatusConflict {
		t.Fatalf("star after create %d %s", starAgain.Code, starAgain.Body.String())
	}

	stale := jsonWithMatch(t, engine, http.MethodPut, basePath, map[string]any{
		"effectiveAt": effectiveAt,
		"assetId":     assetID,
		"quantity":    "200",
	}, sessionA, `"capital-baseline:9"`, "cap-base-stale")
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale baseline %d %s", stale.Code, stale.Body.String())
	}

	updated := jsonWithMatch(t, engine, http.MethodPut, basePath, map[string]any{
		"effectiveAt": effectiveAt,
		"assetId":     assetID,
		"quantity":    "-25",
	}, sessionA, `"capital-baseline:1"`, "cap-base-2")
	if updated.Code != http.StatusOK {
		t.Fatalf("update baseline %d %s", updated.Code, updated.Body.String())
	}
	if updated.Header().Get("ETag") != `"capital-baseline:2"` {
		t.Fatalf("update etag %s", updated.Header().Get("ETag"))
	}

	revisions := doJSON(t, engine, http.MethodGet, basePath+"/revisions", nil, sessionA, "")
	if revisions.Code != http.StatusOK {
		t.Fatalf("baseline revisions %d %s", revisions.Code, revisions.Body.String())
	}
	var revList struct {
		Items []struct {
			Quantity string `json:"quantity"`
		} `json:"items"`
	}
	if err := json.Unmarshal(revisions.Body.Bytes(), &revList); err != nil {
		t.Fatal(err)
	}
	if len(revList.Items) != 2 || revList.Items[0].Quantity != "-25" {
		t.Fatalf("revisions %+v", revList)
	}

	zeroFlow := doJSON(t, engine, http.MethodPost, flowPath, map[string]any{
		"direction":   "contribution",
		"quantity":    "0",
		"assetId":     assetID,
		"effectiveAt": effectiveAt,
	}, sessionA, "cap-flow-zero")
	if zeroFlow.Code != http.StatusBadRequest {
		t.Fatalf("zero flow %d %s", zeroFlow.Code, zeroFlow.Body.String())
	}
	emptyFlow := doJSON(t, engine, http.MethodPost, flowPath, map[string]any{
		"direction":   "contribution",
		"quantity":    "",
		"assetId":     assetID,
		"effectiveAt": effectiveAt,
	}, sessionA, "cap-flow-empty")
	if emptyFlow.Code != http.StatusBadRequest {
		t.Fatalf("empty flow %d %s", emptyFlow.Code, emptyFlow.Body.String())
	}

	flow := doJSON(t, engine, http.MethodPost, flowPath, map[string]any{
		"direction":   "contribution",
		"quantity":    "25.50",
		"assetId":     assetID,
		"effectiveAt": effectiveAt,
		"note":        "seed",
	}, sessionA, "cap-flow-1")
	if flow.Code != http.StatusCreated {
		t.Fatalf("create flow %d %s", flow.Code, flow.Body.String())
	}
	var flowBody struct {
		Flow struct {
			ID       string `json:"id"`
			Quantity string `json:"quantity"`
			Version  int    `json:"version"`
		} `json:"flow"`
	}
	if err := json.Unmarshal(flow.Body.Bytes(), &flowBody); err != nil {
		t.Fatal(err)
	}
	if flowBody.Flow.Quantity != "25.5" || flowBody.Flow.Version != 1 {
		t.Fatalf("flow %+v", flowBody)
	}
	if flow.Header().Get("ETag") != `"capital-flow:1"` {
		t.Fatalf("flow etag %s", flow.Header().Get("ETag"))
	}
	itemPath := flowPath + "/" + flowBody.Flow.ID

	patched := patchWithMatch(t, engine, itemPath, map[string]any{
		"direction": "withdrawal",
		"quantity":  "10",
	}, sessionA, `"capital-flow:1"`)
	if patched.Code != http.StatusOK {
		t.Fatalf("patch flow %d %s", patched.Code, patched.Body.String())
	}
	if patched.Header().Get("ETag") != `"capital-flow:2"` {
		t.Fatalf("patch etag %s", patched.Header().Get("ETag"))
	}

	listedFlows := doJSON(t, engine, http.MethodGet, flowPath, nil, sessionA, "")
	if listedFlows.Code != http.StatusOK {
		t.Fatalf("list flows %d %s", listedFlows.Code, listedFlows.Body.String())
	}
	var flowList struct {
		Items []struct {
			Direction string `json:"direction"`
		} `json:"items"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	if err := json.Unmarshal(listedFlows.Body.Bytes(), &flowList); err != nil {
		t.Fatal(err)
	}
	if flowList.Page.Total != 1 || flowList.Items[0].Direction != "withdrawal" {
		t.Fatalf("active flows %+v", flowList)
	}

	deleted := deleteWithMatch(t, engine, itemPath, sessionA, `"capital-flow:2"`)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete flow %d %s", deleted.Code, deleted.Body.String())
	}
	if deleted.Header().Get("ETag") != `"capital-flow:3"` {
		t.Fatalf("delete etag %s", deleted.Header().Get("ETag"))
	}

	afterDelete := doJSON(t, engine, http.MethodGet, flowPath, nil, sessionA, "")
	if afterDelete.Code != http.StatusOK {
		t.Fatalf("list after delete %d %s", afterDelete.Code, afterDelete.Body.String())
	}
	var afterList struct {
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	if err := json.Unmarshal(afterDelete.Body.Bytes(), &afterList); err != nil {
		t.Fatal(err)
	}
	if afterList.Page.Total != 0 {
		t.Fatalf("deleted still listed %+v", afterList)
	}

	got := doJSON(t, engine, http.MethodGet, itemPath, nil, sessionA, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get deleted flow %d %s", got.Code, got.Body.String())
	}
	var gotFlow struct {
		State   string `json:"state"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &gotFlow); err != nil {
		t.Fatal(err)
	}
	if gotFlow.State != "deleted" || gotFlow.Version != 3 {
		t.Fatalf("deleted flow %+v", gotFlow)
	}

	flowRevs := doJSON(t, engine, http.MethodGet, itemPath+"/revisions", nil, sessionA, "")
	if flowRevs.Code != http.StatusOK {
		t.Fatalf("flow revisions %d %s", flowRevs.Code, flowRevs.Body.String())
	}
	var flowRevList struct {
		Items []struct {
			State string `json:"state"`
		} `json:"items"`
	}
	if err := json.Unmarshal(flowRevs.Body.Bytes(), &flowRevList); err != nil {
		t.Fatal(err)
	}
	if len(flowRevList.Items) != 3 || flowRevList.Items[0].State != "deleted" {
		t.Fatalf("flow revisions %+v", flowRevList)
	}

	crossFlow := doJSON(t, engine, http.MethodGet, itemPath, nil, sessionB, "")
	if crossFlow.Code != http.StatusNotFound {
		t.Fatalf("cross-user flow %d %s", crossFlow.Code, crossFlow.Body.String())
	}
}
