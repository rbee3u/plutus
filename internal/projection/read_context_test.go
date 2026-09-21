package projection

import (
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

func TestApplyLatestValuationContext(t *testing.T) {
	t.Parallel()

	ctx := Context{Completeness: "missing", Coverage: "missing", Freshness: "unknown"}
	applyLatestValuationContext(&ctx, nil)
	if ctx.Completeness != "missing" || ctx.CutoffAt != nil {
		t.Fatalf("empty rows mutated context %+v", ctx)
	}

	older := time.Date(2026, 9, 20, 16, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 9, 21, 1, 58, 0, 0, time.UTC)
	applyLatestValuationContext(&ctx, []sqlcgen.PortfolioValuationProjection{
		{EvaluatedAt: older, Completeness: "partial", FreshnessState: "stale"},
		{EvaluatedAt: latest, Completeness: "complete", FreshnessState: "fresh"},
	})
	if ctx.Completeness != "complete" || ctx.Coverage != "complete" || ctx.Freshness != "fresh" {
		t.Fatalf("latest status %+v", ctx)
	}
	if ctx.CutoffAt == nil || !ctx.CutoffAt.Equal(latest) {
		t.Fatalf("cutoff %v", ctx.CutoffAt)
	}
}
