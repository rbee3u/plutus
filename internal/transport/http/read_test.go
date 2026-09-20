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

	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/projection"
)

func TestOverviewBalancesPositionsReadModel(t *testing.T) {
	db := mysql.OpenTestDB(t)
	authSvc := auth.NewService(db, false, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	proj := projection.NewService(db)
	engine := Engine(&EngineOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:       StaticReady{},
		Auth:        authSvc,
		Portfolios:  portfolios,
		Idempotency: idempotency.NewStore(db),
		Jobs:        jobs.NewService(db),
		Projection:  proj,
	})

	password := "correct horse battery staple"
	userA := "ov-" + uuid.NewString()[:8]
	userB := "ovb-" + uuid.NewString()[:8]
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
	parsed, err := uuid.Parse(portfolioID)
	if err != nil {
		t.Fatal(err)
	}

	missing := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/overview", nil, sessionA, "")
	if missing.Code != http.StatusOK {
		t.Fatalf("missing overview %d %s", missing.Code, missing.Body.String())
	}
	var overview struct {
		Valuation struct {
			Amount       *string `json:"amount"`
			Completeness string  `json:"completeness"`
		} `json:"valuation"`
		Context struct {
			Completeness string `json:"completeness"`
		} `json:"context"`
		Issues []struct {
			Code string `json:"code"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(missing.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Valuation.Amount != nil || overview.Valuation.Completeness != "missing" || overview.Context.Completeness != "missing" {
		t.Fatalf("missing overview %+v", overview)
	}
	if len(overview.Issues) == 0 {
		t.Fatal("expected missing generation issue")
	}

	job, err := jobs.NewService(db).EnqueueRebuild(context.Background(), parsed, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Rebuild(context.Background(), projection.Request{PortfolioID: parsed, JobID: job.ID}); err != nil {
		t.Fatal(err)
	}

	ready := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/overview", nil, sessionA, "")
	if ready.Code != http.StatusOK {
		t.Fatalf("ready overview %d %s", ready.Code, ready.Body.String())
	}
	if err := json.Unmarshal(ready.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Context.Completeness == "" {
		t.Fatalf("ready overview %+v", overview)
	}

	balances := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/balances?limit=1&offset=0&groupBy=account", nil, sessionA, "")
	if balances.Code != http.StatusOK {
		t.Fatalf("balances %d %s", balances.Code, balances.Body.String())
	}
	var balancePage struct {
		Items []struct {
			AccountId string `json:"accountId"`
		} `json:"items"`
		Groups []struct {
			Grouping string `json:"grouping"`
			Key      string `json:"key"`
		} `json:"groups"`
		Page struct {
			Kind   string `json:"kind"`
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
			Total  int    `json:"total"`
		} `json:"page"`
	}
	if err := json.Unmarshal(balances.Body.Bytes(), &balancePage); err != nil {
		t.Fatal(err)
	}
	if balancePage.Page.Kind != "offset" || balancePage.Page.Limit != 1 {
		t.Fatalf("balance page %+v", balancePage)
	}

	badGroup := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/balances?groupBy=not-a-group", nil, sessionA, "")
	if badGroup.Code != http.StatusBadRequest {
		t.Fatalf("bad group %d %s", badGroup.Code, badGroup.Body.String())
	}

	positions := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/positions", nil, sessionA, "")
	if positions.Code != http.StatusOK {
		t.Fatalf("positions %d %s", positions.Code, positions.Body.String())
	}

	foreign := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/overview", nil, sessionB, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-user overview %d %s", foreign.Code, foreign.Body.String())
	}
	foreignBal := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/balances", nil, sessionB, "")
	if foreignBal.Code != http.StatusNotFound {
		t.Fatalf("cross-user balances %d %s", foreignBal.Code, foreignBal.Body.String())
	}
}
