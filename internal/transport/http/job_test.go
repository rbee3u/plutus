package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/projection"
	"github.com/rbee3u/plutus/internal/worker"
)

func TestProjectionJobsAndCursorPagination(t *testing.T) {
	db := mysql.OpenTestDB(t)
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	jobSvc := jobs.NewService(db)
	engine := Engine(&EngineOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:       StaticReady{},
		Auth:        authSvc,
		Portfolios:  portfolios,
		Idempotency: idempotency.NewStore(db),
		Jobs:        jobSvc,
	})
	loop := worker.New(worker.Options{
		Jobs:       jobSvc,
		Projection: projection.NewService(db),
		Adapters:   adapters.NewRegistry(),
		Poll:       10 * time.Millisecond,
		Lease:      time.Minute,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		ID:         "test-worker",
	})

	password := "correct horse battery staple"
	userA := "ja-" + uuid.NewString()[:8]
	userB := "jb-" + uuid.NewString()[:8]
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
	portfolioID, err := uuid.Parse(listed.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	first, err := jobSvc.EnqueueRebuild(context.Background(), portfolioID, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	again, err := jobSvc.EnqueueRebuild(context.Background(), portfolioID, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != again.ID {
		t.Fatalf("dedupe failed: %s vs %s", first.ID, again.ID)
	}

	if err := runUntilStatus(t, loop, jobSvc, portfolioID, first.ID, "succeeded"); err != nil {
		t.Fatal(err)
	}
	active, err := jobSvc.ActiveGenerationID(context.Background(), portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	if active == uuid.Nil {
		t.Fatal("active generation was not switched")
	}
	got := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID.String()+"/jobs/"+first.ID.String(), nil, sessionA, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get job %d %s", got.Code, got.Body.String())
	}
	var jobBody struct {
		Status   string `json:"status"`
		Attempts []struct {
			Outcome    string  `json:"outcome"`
			SafeDetail *string `json:"safeDetail"`
		} `json:"attempts"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &jobBody); err != nil {
		t.Fatal(err)
	}
	if jobBody.Status != "succeeded" || len(jobBody.Attempts) != 1 || jobBody.Attempts[0].Outcome != "succeeded" {
		t.Fatalf("job body %+v", jobBody)
	}
	if jobBody.Attempts[0].SafeDetail == nil || *jobBody.Attempts[0].SafeDetail == "" {
		t.Fatal("missing projection diagnostic")
	}

	foreign := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID.String()+"/jobs/"+first.ID.String(), nil, sessionB, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-user job %d %s", foreign.Code, foreign.Body.String())
	}

	var ids []string
	ids = append(ids, first.ID.String())
	for range 2 {
		queued, err := jobSvc.EnqueueRebuild(context.Background(), portfolioID, nil, "test")
		if err != nil {
			t.Fatal(err)
		}
		if err := runUntilStatus(t, loop, jobSvc, portfolioID, queued.ID, "succeeded"); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, queued.ID.String())
	}

	page1 := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID.String()+"/jobs?limit=2", nil, sessionA, "")
	if page1.Code != http.StatusOK {
		t.Fatalf("page1 %d %s", page1.Code, page1.Body.String())
	}
	var listedJobs struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Page struct {
			HasMore    bool    `json:"hasMore"`
			NextCursor *string `json:"nextCursor"`
			Kind       string  `json:"kind"`
			Limit      int     `json:"limit"`
		} `json:"page"`
	}
	if err := json.Unmarshal(page1.Body.Bytes(), &listedJobs); err != nil {
		t.Fatal(err)
	}
	if !listedJobs.Page.HasMore || listedJobs.Page.NextCursor == nil || listedJobs.Page.Kind != "cursor" || listedJobs.Page.Limit != 2 || len(listedJobs.Items) != 2 {
		t.Fatalf("page1 %+v", listedJobs)
	}
	seen := map[string]bool{listedJobs.Items[0].ID: true, listedJobs.Items[1].ID: true}

	page2 := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID.String()+"/jobs?limit=2&cursor="+url.QueryEscape(*listedJobs.Page.NextCursor), nil, sessionA, "")
	if page2.Code != http.StatusOK {
		t.Fatalf("page2 %d %s", page2.Code, page2.Body.String())
	}
	if err := json.Unmarshal(page2.Body.Bytes(), &listedJobs); err != nil {
		t.Fatal(err)
	}
	if len(listedJobs.Items) != 1 {
		t.Fatalf("page2 items %+v", listedJobs.Items)
	}
	if seen[listedJobs.Items[0].ID] {
		t.Fatalf("cursor duplicated %s", listedJobs.Items[0].ID)
	}
	seen[listedJobs.Items[0].ID] = true
	if len(seen) != 3 {
		t.Fatalf("cursor skipped, seen %v want %v", seen, ids)
	}

	bad := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID.String()+"/jobs?cursor=not-a-cursor", nil, sessionA, "")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor %d %s", bad.Code, bad.Body.String())
	}
}

func TestHTTPProjectionRebuildEnqueue(t *testing.T) {
	db := mysql.OpenTestDB(t)
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	jobSvc := jobs.NewService(db)
	engine := Engine(&EngineOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:       StaticReady{},
		Auth:        authSvc,
		Portfolios:  portfolios,
		Idempotency: idempotency.NewStore(db),
		Jobs:        jobSvc,
	})

	password := "correct horse battery staple"
	userA := "jr-" + uuid.NewString()[:8]
	userB := "jrb-" + uuid.NewString()[:8]
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
	path := "/api/v1/portfolios/" + portfolioID + "/projection-rebuilds"

	first := doJSON(t, engine, http.MethodPost, path, map[string]any{"reason": "manual rebuild"}, sessionA, "rebuild-1")
	if first.Code != http.StatusAccepted {
		t.Fatalf("first %d %s", first.Code, first.Body.String())
	}
	var jobBody struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Kind   string `json:"kind"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &jobBody); err != nil {
		t.Fatal(err)
	}
	if jobBody.Status != "queued" || jobBody.Kind != "projection" {
		t.Fatalf("job %+v", jobBody)
	}
	wantLoc := "/api/v1/portfolios/" + portfolioID + "/jobs/" + jobBody.ID
	if first.Header().Get("Location") != wantLoc {
		t.Fatalf("location %s want %s", first.Header().Get("Location"), wantLoc)
	}

	replay := doJSON(t, engine, http.MethodPost, path, map[string]any{"reason": "manual rebuild"}, sessionA, "rebuild-1")
	if replay.Code != http.StatusAccepted {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}
	var replayBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &replayBody); err != nil {
		t.Fatal(err)
	}
	if replayBody.ID != jobBody.ID {
		t.Fatalf("idempotent replay %s vs %s", replayBody.ID, jobBody.ID)
	}

	conflict := doJSON(t, engine, http.MethodPost, path, map[string]any{"reason": "other"}, sessionA, "rebuild-1")
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict %d %s", conflict.Code, conflict.Body.String())
	}

	merged := doJSON(t, engine, http.MethodPost, path, nil, sessionA, "")
	if merged.Code != http.StatusAccepted {
		t.Fatalf("merge %d %s", merged.Code, merged.Body.String())
	}
	var mergedBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(merged.Body.Bytes(), &mergedBody); err != nil {
		t.Fatal(err)
	}
	if mergedBody.ID != jobBody.ID {
		t.Fatalf("active merge %s vs %s", mergedBody.ID, jobBody.ID)
	}

	foreign := doJSON(t, engine, http.MethodPost, path, nil, sessionB, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-user rebuild %d %s", foreign.Code, foreign.Body.String())
	}
}

func runUntilStatus(t *testing.T, loop *worker.Worker, svc *jobs.Service, portfolioID, jobID uuid.UUID, status string) error {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := loop.ProcessOnce(context.Background()); err != nil {
			return err
		}
		item, err := svc.Get(context.Background(), portfolioID, jobID)
		if err != nil {
			return err
		}
		if item.Status == status {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return context.DeadlineExceeded
}
