package valuation

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestEvaluationPreservesExactAmountsAndOmissions(t *testing.T) {
	source := uuid.New()
	price := big.NewRat(1, 3)
	input := preparedReport{account: AccountValuation{SourceReportID: &source, Freshness: FreshnessStale}, balances: []preparedBalance{
		{platformAssetSymbol: "A", equity: big.NewRat(1, 1), price: price},
		{platformAssetSymbol: "B", equity: big.NewRat(1, 1), price: price},
		{platformAssetSymbol: "C", equity: big.NewRat(1, 1), price: price},
		{platformAssetSymbol: "DEBT", equity: big.NewRat(-2, 1), price: big.NewRat(1, 10)},
		{platformAssetSymbol: "UNKNOWN", assetSymbol: "MAPPED", rawEquity: "2.00", equity: big.NewRat(2, 1)},
		{platformAssetSymbol: "INVALID", rawEquity: "invalid"},
	}}
	got := evaluateReport(input)
	if got.ValueUSD == nil || *got.ValueUSD != "0.8" || got.valueRat.Cmp(big.NewRat(4, 5)) != 0 || got.Method != MethodCalculated || got.Freshness != FreshnessStale {
		t.Fatalf("unexpected calculated value: %+v", got)
	}
	want := []Omitted{{PlatformAssetSymbol: "UNKNOWN", AssetSymbol: "MAPPED", Equity: "2", Reason: "no_price"}, {PlatformAssetSymbol: "INVALID", Equity: "invalid", Reason: "invalid_equity"}}
	if !reflect.DeepEqual(got.Omitted, want) || got.Completeness() != CompletenessIncomplete {
		t.Fatalf("omissions: %+v", got.Omitted)
	}
	if price.Cmp(big.NewRat(1, 3)) != 0 || input.balances[3].equity.Cmp(big.NewRat(-2, 1)) != 0 {
		t.Fatal("evaluation mutated prepared or cached numbers")
	}
}

func FuzzCalculatedValueConservation(f *testing.F) {
	f.Add(int64(0), int64(0), uint32(0))
	f.Add(int64(125), int64(-125), uint32(999))
	f.Add(int64(-200), int64(37), uint32(124))
	f.Add(int64(math.MinInt64), int64(math.MaxInt64), uint32(math.MaxUint32))
	f.Fuzz(func(t *testing.T, left, right int64, priceUnits uint32) {
		// Quantities have two decimal places and the positive price has three.
		// Integer arithmetic supplies an independent oracle without overflow.
		priceNumerator := int64(priceUnits) + 1
		sum := new(big.Int).Add(big.NewInt(left), big.NewInt(right))
		want := new(big.Rat).SetFrac(
			new(big.Int).Mul(sum, big.NewInt(priceNumerator)), big.NewInt(100000))
		price := big.NewRat(priceNumerator, 1000)
		a := preparedBalance{platformAssetSymbol: "A", equity: big.NewRat(left, 100), price: price}
		b := preparedBalance{platformAssetSymbol: "B", equity: big.NewRat(right, 100), price: price}
		zero := preparedBalance{platformAssetSymbol: "ZERO", equity: new(big.Rat), price: price}
		source := uuid.MustParse("01a0bdfc-1fa4-7a54-9fa1-24f4721a71af")
		input := preparedReport{account: AccountValuation{SourceReportID: &source, Freshness: FreshnessFresh}}
		for _, tc := range []struct {
			name     string
			balances []preparedBalance
		}{
			{"combined", []preparedBalance{{platformAssetSymbol: "SUM", equity: new(big.Rat).SetFrac(sum, big.NewInt(100)), price: price}}},
			{"split", []preparedBalance{a, b}},
			{"reversed with zero", []preparedBalance{b, zero, a}},
		} {
			input.balances = tc.balances
			got := evaluateReport(input)
			if got.ValueUSD == nil {
				t.Fatalf("%s: known balances produced an unknown value", tc.name)
			}
			value, ok := decimal.Parse(*got.ValueUSD)
			if !ok || value.Cmp(want) != 0 {
				t.Fatalf("%s: value = %q, want %s", tc.name, *got.ValueUSD, want.RatString())
			}
			if got.Method != MethodCalculated || got.Completeness() != CompletenessComplete {
				t.Fatalf("%s: expected a complete calculated value, got %+v", tc.name, got)
			}
		}

		input.balances = []preparedBalance{a, b, {platformAssetSymbol: "UNKNOWN", rawEquity: "1", equity: big.NewRat(1, 1)}}
		got := evaluateReport(input)
		if got.ValueUSD == nil {
			t.Fatal("a missing price erased the known subtotal")
		}
		value, ok := decimal.Parse(*got.ValueUSD)
		if !ok || value.Cmp(want) != 0 {
			t.Fatalf("a missing price changed the known subtotal to %q, want %s", *got.ValueUSD, want.RatString())
		}
		omitted := []Omitted{{PlatformAssetSymbol: "UNKNOWN", Equity: "1", Reason: "no_price"}}
		if got.Completeness() != CompletenessIncomplete || !reflect.DeepEqual(got.Omitted, omitted) {
			t.Fatalf("missing price must make the result incomplete and name the omission: %+v", got)
		}
	})
}

type rejectingPrices struct{ calls int }

func (p *rejectingPrices) Lookup(context.Context, string, string, time.Time) (*big.Rat, bool, error) {
	p.calls++
	return nil, false, errors.New("price lookup unavailable")
}

func (p *rejectingPrices) LookupPlatform(ctx context.Context, platform, symbol string, at time.Time) (*big.Rat, bool, error) {
	return p.Lookup(ctx, platform, symbol, at)
}

func TestPreparationDoesNotIntroduceUnusedPriceReads(t *testing.T) {
	report, account, identity := uuid.New(), uuid.New(), uuid.New()
	excluded, zero, invalid := uuid.New(), uuid.New(), uuid.New()
	row := sqlcgen.ListReportsByJobRow{ID: report[:], AccountID: account[:], IdentityID: identity[:], Status: "ok", ObservedAt: sql.NullTime{Time: time.Now(), Valid: true}}
	book := &rejectingPrices{}
	// With no database, any accidental SQL dependency also fails this test.
	service := &Service{}
	row.Equity = sql.NullString{String: "12.345", Valid: true}
	input, err := service.prepareReport(context.Background(), book, &row, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := evaluateReport(input)
	if got.Method != MethodReported || got.ValueUSD == nil || *got.ValueUSD != "12.345" {
		t.Fatalf("reported: %+v", got)
	}
	row.Equity = sql.NullString{}
	preload := &reportInputs{balances: map[uuid.UUID][]sqlcgen.ListBalancesByReportRow{report: {
		{PlatformAssetID: excluded[:], Equity: "10", PlatformAssetSymbol: "EXCLUDED"},
		{PlatformAssetID: zero[:], Equity: "0", PlatformAssetSymbol: "ZERO"},
		{PlatformAssetID: invalid[:], Equity: "invalid", PlatformAssetSymbol: "INVALID"},
	}}}
	input, err = service.prepareReport(context.Background(), book, &row, map[uuid.UUID]bool{excluded: true}, preload)
	if err != nil {
		t.Fatal(err)
	}
	got = evaluateReport(input)
	if got.ValueUSD == nil || *got.ValueUSD != "0" || len(got.Omitted) != 1 || got.Omitted[0].Reason != "invalid_equity" {
		t.Fatalf("calculated: %+v", got)
	}
	row.Status = "failed"
	input, err = service.prepareReport(context.Background(), book, &row, nil, &reportInputs{})
	if err != nil {
		t.Fatal(err)
	}
	got = evaluateReport(input)
	if got.Method != MethodMissing || got.ValueUSD != nil || got.SourceReportID != nil || got.Completeness() != CompletenessIncomplete {
		t.Fatalf("missing must remain unknown: %+v", got)
	}
	if book.calls != 0 {
		t.Fatalf("performed %d unused price reads", book.calls)
	}
}

func TestWriteAndRebuildUseEquivalentPreparedInputs(t *testing.T) {
	f := newRebuildFixture(t)
	accounts := f.accounts("spot", "funding", "earn")
	firstJob := f.job(f.at, "succeeded")
	source := f.report(firstJob, accounts["spot"], f.at, "ok", 2, nil)
	f.report(firstJob, accounts["funding"], f.at, "ok", 2, "2.25")
	f.report(firstJob, accounts["earn"], f.at, "failed", 2, nil)
	symbol := "PREPARED-" + uuid.NewString()[:8]
	platformAsset, missing := uuid.New(), uuid.New()
	for _, item := range []struct {
		id     uuid.UUID
		symbol string
	}{{platformAsset, symbol}, {missing, symbol + "-MISSING"}} {
		f.exec("INSERT INTO platform_assets (id,platform,symbol,asset_symbol,first_seen_at,last_seen_at,created_at,updated_at) VALUES (?,'binance',?,?,?,?,?,?)", item.id[:], item.symbol, item.symbol, f.at, f.at, f.at, f.at)
	}
	f.exec("INSERT INTO balances (report_id,portfolio_id,platform_asset_id,equity) VALUES (?,?,?,-3),(?,?,?,7)", source[:], f.portfolio[:], platformAsset[:], source[:], f.portfolio[:], missing[:])
	f.exec("INSERT INTO prices (platform,asset_symbol,observed_at,price,quote_asset_symbol) VALUES ('binance',?,?,0.125,'USD')", symbol, f.at)
	secondJob := f.job(f.at.Add(time.Minute), "succeeded")
	f.report(secondJob, accounts["spot"], f.at.Add(time.Minute), "failed", 2, nil)
	f.report(secondJob, accounts["funding"], f.at.Add(time.Minute), "ok", 2, "2.25")
	f.report(secondJob, accounts["earn"], f.at.Add(time.Minute), "failed", 2, nil)
	before := map[uuid.UUID][]AccountValuation{}
	for i, job := range []uuid.UUID{firstJob, secondJob} {
		point, err := f.service.WritePoint(f.ctx, f.portfolio, job, f.at.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if point.TotalUSD != "1.875" || point.IncompleteAccountCount != 2 {
			t.Fatalf("ordinary point: %+v", point)
		}
		before[job], err = f.service.Accounts(f.ctx, f.portfolio, job)
		if err != nil {
			t.Fatal(err)
		}
	}
	if count, err := f.service.Rebuild(f.ctx, f.portfolio); err != nil || count != 2 {
		t.Fatalf("rebuild: %d %v", count, err)
	}
	for _, job := range []uuid.UUID{firstJob, secondJob} {
		after, err := f.service.Accounts(f.ctx, f.portfolio, job)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before[job], after) {
			t.Fatalf("rebuilt Accounts differ for %s:\nbefore: %+v\nafter: %+v", job, before[job], after)
		}
		point, err := f.service.PointByJob(f.ctx, f.portfolio, job)
		if err != nil || decimal.ParseOrZero(point.TotalUSD).Cmp(big.NewRat(15, 8)) != 0 || point.IncompleteAccountCount != 2 {
			t.Fatalf("rebuilt point: %+v %v", point, err)
		}
	}
}
