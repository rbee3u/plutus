package collection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/db/migrations"
	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/valuation"
)

type earnScenario struct{ legacy, failed, empty bool }

func (a *earnScenario) Validate(context.Context, adapters.Credentials, adapters.AccountStructure) (adapters.ValidationResult, error) {
	return adapters.ValidationResult{ExternalRef: "earn-test", MarginMode: adapters.MarginOKXPM}, nil
}

func (a *earnScenario) Collect(_ context.Context, _ adapters.Credentials, _ adapters.AccountStructure, types []adapters.AccountType) (adapters.Batch, error) {
	at := time.Now().UTC()
	out := adapters.Batch{ObservedAt: at, MarginMode: adapters.MarginOKXPM}
	for _, kind := range types {
		if a.failed {
			out.Reports = append(out.Reports, adapters.FailedReport(kind, adapters.NewError(adapters.ClassTemporary, "Synthetic refusal")))
			continue
		}
		r := adapters.AccountReport{AccountType: kind, Status: adapters.StatusOK, ObservedAt: at, EarnDetailsCollected: !a.legacy}
		if !a.empty {
			r.Balances = []adapters.Balance{{Symbol: "USDT", Equity: "40"}, {Symbol: "USDC", Equity: "10"}}
			if !a.legacy {
				p := adapters.NewEarnPosition("multi", "on_chain", "position", "principal")
				p.Assets = []adapters.EarnAmount{{Symbol: "USDT", Amount: "15"}, {Symbol: "USDC", Amount: "10"}}
				p.AddReward("USDT", "100", "estimated")
				p.Problems = append(p.Problems, "Estimated APR could not be read.")
				q := adapters.NewEarnPosition("fixed", "fixed", "position", "principal")
				q.Assets = []adapters.EarnAmount{{Symbol: "USDT", Amount: "25"}}
				r.EarnPositions = []adapters.EarnPosition{p, q}
			}
		}
		out.Reports = append(out.Reports, r)
	}
	return out, nil
}

func TestEarnLegacyDetailsStaleEmptyAndIsolation(t *testing.T) {
	h := newHarness(t)
	a := &earnScenario{legacy: true}
	h.registry.Register(adapters.PlatformOKX, a)
	id := h.createIdentity("Earn details", adapters.PlatformOKX, adapters.StructureOKXUnified, "earn")
	id = h.storeCredentials(id, "test")
	if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, id.ID, id.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
		t.Fatal(err)
	}
	first, job := h.runCycle()
	account := h.accountsByType(job, id.ID)["earn"]
	pctx := valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	legacy, err := h.valuation.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || legacy.EarnDetailsCollected || len(legacy.EarnPositions) != 0 || len(legacy.EarnBalanceSummaries) != 1 || len(legacy.EarnBalanceSummaries[0].Assets) != 2 {
		t.Fatalf("legacy: %+v %v", legacy, err)
	}
	// Apply the appended migration to populated tables, preserving old report facts.
	if _, err := h.collection.db.ExecContext(h.ctx, "ALTER TABLE account_reports DROP COLUMN earn_details_collected"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.collection.db.ExecContext(h.ctx, "DROP TABLE earn_positions"); err != nil {
		t.Fatal(err)
	}
	source, err := migrations.FS.ReadFile("20261005010000_earn_positions.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(source), "-- +goose Down")
	if err := mysql.ExecSQLStatements(h.ctx, h.collection.db, up); err != nil {
		t.Fatal(err)
	}
	legacy, err = h.valuation.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || legacy.EarnDetailsCollected || *legacy.Account.ValueUSD != "50" {
		t.Fatalf("populated migration: %+v %v", legacy, err)
	}
	a.legacy = false
	second, job := h.runCycle()
	account = h.accountsByType(job, id.ID)["earn"]
	if first.Point.TotalUSD != "50" || second.Point.TotalUSD != "50" || second.Point.BoundaryContributionUSD != "0" {
		t.Fatalf("descriptive detail changed value: %+v", second.Point)
	}
	view, err := h.valuation.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || !view.EarnDetailsCollected || len(view.EarnPositions) != 2 || len(view.EarnBalanceSummaries) != 0 {
		t.Fatalf("detail: %+v %v", view, err)
	}
	page, err := h.valuation.FlatEarnPositions(h.ctx, pctx, &valuation.ListFilter{Symbol: "uSdT", Limit: 1, Offset: 1})
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].PlatformPositionRef != "fixed" {
		t.Fatalf("sort/filter/page: %+v %v", page, err)
	}
	page, err = h.valuation.FlatEarnPositions(h.ctx, pctx, &valuation.ListFilter{Symbol: "sDc", Limit: 50})
	if err != nil || page.Total != 1 || len(page.Items[0].Assets) != 2 || len(page.Items[0].Issues) != 1 {
		t.Fatalf("multiasset row: %+v %v", page, err)
	}
	if _, err := h.valuation.AccountEarnPositions(h.ctx, valuation.PortfolioContext{ID: uuid.New(), Timezone: "UTC"}, account.AccountID); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("scope leak: %v", err)
	}
	a.failed = true
	stale, job := h.runCycle()
	if stale.Point.TotalUSD != "50" {
		t.Fatal("failure removed funds")
	}
	view, err = h.valuation.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || len(view.EarnPositions) != 2 || view.EarnPositions[0].Freshness != valuation.FreshnessStale || !view.EarnPositions[0].ReportObservedAt.Equal(*account.ReportObservedAt) {
		t.Fatalf("stale: %+v %v", view, err)
	}
	balances, err := h.valuation.AccountBalances(h.ctx, pctx, account.AccountID)
	if err != nil || *view.Account.SourceReportID != *balances.Account.SourceReportID {
		t.Fatal("report mismatch")
	}
	if _, err := h.valuation.Rebuild(h.ctx, h.portfolioID); err != nil {
		t.Fatal(err)
	}
	point, err := h.valuation.PointByJob(h.ctx, h.portfolioID, job)
	if err != nil || point.TotalUSD != "50" {
		t.Fatal("rebuild changed funds")
	}
	a.failed = false
	a.empty = true
	last, _ := h.runCycle()
	view, err = h.valuation.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || !view.EarnDetailsCollected || len(view.EarnPositions) != 0 || len(view.EarnBalanceSummaries) != 0 || last.Point.TotalUSD != "0" {
		t.Fatalf("empty: %+v %v", view, err)
	}
}
