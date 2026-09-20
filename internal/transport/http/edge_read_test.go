package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/rbee3u/plutus/db/seeds"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/projection"
)

func TestEdgeCaseReadModelsStayIncomplete(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := context.Background()
	ensureEdgeCaseFacts(t, db)

	authSvc := auth.NewService(db, false, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	proj := projection.NewService(db)
	jobSvc := jobs.NewService(db)
	engine := Engine(&EngineOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:       StaticReady{},
		Auth:        authSvc,
		Portfolios:  portfolios,
		Idempotency: idempotency.NewStore(db),
		Jobs:        jobSvc,
		Projection:  proj,
	})

	job, err := jobSvc.EnqueueRebuild(ctx, seeds.PortfolioAliceMain, nil, "edge-cases-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Rebuild(ctx, projection.Request{PortfolioID: seeds.PortfolioAliceMain, JobID: job.ID}); err != nil {
		t.Fatal(err)
	}

	session := loginSession(t, engine, "alice", "Alice=123456")
	foreign := loginSession(t, engine, "bob", "Bob=123456")
	portfolioID := seeds.PortfolioAliceMain.String()
	seriesWindow := "from=2026-08-31T00:00:00Z&to=2026-09-20T00:00:00Z&limit=50&maxPoints=200"
	periodWindow := "from=2026-08-31T00:00:00Z&to=2026-09-20T00:00:00Z&limit=50"

	denied := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/overview", nil, foreign, "")
	if denied.Code != http.StatusNotFound {
		t.Fatalf("foreign overview %d %s", denied.Code, denied.Body.String())
	}

	overview := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/overview", nil, session, "")
	if overview.Code != http.StatusOK {
		t.Fatalf("overview %d %s", overview.Code, overview.Body.String())
	}
	var overviewBody struct {
		Valuation struct {
			Amount       *string `json:"amount"`
			Completeness string  `json:"completeness"`
		} `json:"valuation"`
		Context struct {
			Timezone     string `json:"timezone"`
			Completeness string `json:"completeness"`
			Coverage     string `json:"coverage"`
		} `json:"context"`
		Issues []struct {
			Code string `json:"code"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(overview.Body.Bytes(), &overviewBody); err != nil {
		t.Fatal(err)
	}
	if overviewBody.Context.Timezone != "Asia/Shanghai" {
		t.Fatalf("timezone %+v", overviewBody.Context)
	}
	if overviewBody.Valuation.Completeness == "complete" && overviewBody.Context.Coverage == "complete" && len(overviewBody.Issues) == 0 {
		t.Fatalf("edge-cases overview looks complete %+v", overviewBody)
	}
	rejectInventedZero(t, overviewBody.Valuation.Completeness, overviewBody.Valuation.Amount, "overview valuation")
	if len(overviewBody.Issues) == 0 {
		t.Fatal("expected projection issues for missing price, coverage, or stale facts")
	}

	exposures := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/exposures", nil, session, "")
	if exposures.Code != http.StatusOK {
		t.Fatalf("exposures %d %s", exposures.Code, exposures.Body.String())
	}
	var exposureBody struct {
		Items []struct {
			AssetCode     *string `json:"assetCode"`
			LongQuantity  string  `json:"longQuantity"`
			ShortQuantity string  `json:"shortQuantity"`
			NetQuantity   string  `json:"netQuantity"`
			NetValue      *string `json:"netValue"`
			Completeness  string  `json:"completeness"`
		} `json:"items"`
		Issues []struct {
			Code string `json:"code"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(exposures.Body.Bytes(), &exposureBody); err != nil {
		t.Fatal(err)
	}
	opposing := false
	nullIncomplete := false
	for _, item := range exposureBody.Items {
		if !isZeroDecimal(item.LongQuantity) && !isZeroDecimal(item.ShortQuantity) {
			opposing = true
		}
		rejectInventedZero(t, item.Completeness, item.NetValue, "exposure net value")
		if item.Completeness != "complete" && item.NetValue == nil {
			nullIncomplete = true
		}
	}
	if !opposing {
		t.Fatalf("expected long and short exposure to both remain visible %+v", exposureBody.Items)
	}
	if !nullIncomplete && len(exposureBody.Issues) == 0 {
		t.Fatal("expected incomplete exposure values to stay null or issue-bearing")
	}

	valuations := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/valuations?"+seriesWindow, nil, session, "")
	if valuations.Code != http.StatusOK {
		t.Fatalf("valuations %d %s", valuations.Code, valuations.Body.String())
	}
	var valuationBody struct {
		Items []struct {
			Amount       *string `json:"amount"`
			Completeness string  `json:"completeness"`
		} `json:"items"`
	}
	if err := json.Unmarshal(valuations.Body.Bytes(), &valuationBody); err != nil {
		t.Fatal(err)
	}
	for _, item := range valuationBody.Items {
		rejectInventedZero(t, item.Completeness, item.Amount, "valuation point")
	}

	changes := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/value-changes?periodKind=day&"+seriesWindow, nil, session, "")
	if changes.Code != http.StatusOK {
		t.Fatalf("value changes %d %s", changes.Code, changes.Body.String())
	}
	var changeBody struct {
		Items []struct {
			PeriodStartDate string  `json:"periodStartDate"`
			Amount          *string `json:"amount"`
			State           string  `json:"state"`
		} `json:"items"`
		Issues []struct {
			Code string `json:"code"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(changes.Body.Bytes(), &changeBody); err != nil {
		t.Fatal(err)
	}
	sawShanghaiDay := false
	sawIncomparable := false
	for _, item := range changeBody.Items {
		if item.PeriodStartDate == "2026-09-01" {
			sawShanghaiDay = true
		}
		if item.State == "incomparable" || item.State == "incomplete" {
			sawIncomparable = true
			if item.Amount != nil && isZeroDecimal(*item.Amount) {
				t.Fatalf("incomparable value change used zero %+v", item)
			}
		}
	}
	if !sawShanghaiDay {
		t.Fatalf("expected Asia/Shanghai day 2026-09-01 from 2026-08-31 16:00 UTC %+v", changeBody.Items)
	}
	if !sawIncomparable && len(changeBody.Issues) == 0 {
		t.Fatal("expected incomparable or incomplete value change around the coverage change")
	}

	summaries := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/period-summaries?periodKind=day&"+periodWindow, nil, session, "")
	if summaries.Code != http.StatusOK {
		t.Fatalf("period summaries %d %s", summaries.Code, summaries.Body.String())
	}
	var summaryBody struct {
		Items []struct {
			PeriodStartDate string  `json:"periodStartDate"`
			ValuationClose  *string `json:"valuationClose"`
			Completeness    string  `json:"completeness"`
		} `json:"items"`
	}
	if err := json.Unmarshal(summaries.Body.Bytes(), &summaryBody); err != nil {
		t.Fatal(err)
	}
	sawSummaryDay := false
	for _, item := range summaryBody.Items {
		if item.PeriodStartDate == "2026-09-01" {
			sawSummaryDay = true
		}
		rejectInventedZero(t, item.Completeness, item.ValuationClose, "period summary close")
	}
	if !sawSummaryDay {
		t.Fatalf("expected period summary for 2026-09-01 %+v", summaryBody.Items)
	}

	bad := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/valuations?cursor=not-a-cursor", nil, session, "")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor %d %s", bad.Code, bad.Body.String())
	}
	var problem struct {
		Detail  string  `json:"detail"`
		TraceID *string `json:"traceId"`
		Code    string  `json:"code"`
	}
	if err := json.Unmarshal(bad.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Detail == "" || problem.Code == "" || problem.TraceID == nil || *problem.TraceID == "" {
		t.Fatalf("problem details %+v", problem)
	}
}

func ensureEdgeCaseFacts(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	var tokenX int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM assets WHERE code = 'TOKENX'").Scan(&tokenX); err != nil {
		t.Fatal(err)
	}
	if tokenX > 0 {
		return
	}
	var alice int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username_normalized = 'alice'").Scan(&alice); err != nil {
		t.Fatal(err)
	}
	if alice > 0 {
		t.Skip("test database already has alice without TOKENX; refusing to wipe for edge-cases")
	}
	sqlText, err := seeds.EdgeCasesSQL()
	if err != nil {
		t.Fatal(err)
	}
	if err := mysql.ExecSQLStatements(ctx, db, sqlText); err != nil {
		t.Fatalf("edge-cases seed: %v", err)
	}
}

func rejectInventedZero(t *testing.T, completeness string, amount *string, label string) {
	t.Helper()
	if completeness != "complete" && amount != nil && isZeroDecimal(*amount) {
		t.Fatalf("%s used zero for incomplete amount %q", label, *amount)
	}
}

func isZeroDecimal(value string) bool {
	n, err := strconv.ParseFloat(value, 64)
	return err == nil && n == 0
}
