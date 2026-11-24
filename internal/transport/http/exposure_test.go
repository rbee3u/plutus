package httpserver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

func TestExposureJSONPreservesUnknownAndPartialValues(t *testing.T) {
	accountID := uuid.New()
	row := financial.ExposureRow{
		CanonicalAssetSymbol: "LINK",
		PlatformSymbols:      []string{"LINK"},
		State:                financial.ExposureUnknown,
		Freshness:            valuation.FreshnessFresh,
		AccountCount:         1,
		UnknownAccountCount:  1,
		Issues: []valuation.Issue{{
			Code:        "exposure_unknown_position",
			AccountID:   &accountID,
			AccountName: "Trading",
			Symbol:      "LINK",
			Detail:      "position inputs unavailable",
		}},
	}
	data, err := json.Marshal(toAPIExposureRow(&row))
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, field := range []string{`"netQuantity":null`, `"priceUsd":null`, `"valueUsd":null`, `"priceSource":null`, `"state":"unknown"`} {
		if !strings.Contains(encoded, field) {
			t.Fatalf("missing %s in %s", field, encoded)
		}
	}

	list := financial.ExposureList{
		Context: financial.Context{Timezone: "UTC"},
		Summary: financial.ExposureSummary{Completeness: valuation.CompletenessIncomplete, Freshness: valuation.FreshnessFresh},
		Coverage: financial.ExposureCoverage{
			Complete:            false,
			MissingAccountIDs:   []uuid.UUID{},
			MissingAccountCount: 0,
		},
		Items:  []financial.ExposureRow{row},
		Issues: []valuation.Issue{},
	}
	data, err = json.Marshal(toAPIExposureList(list))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"missingAccountIds":[]`) {
		t.Fatalf("empty coverage IDs were not preserved: %s", data)
	}
	if !strings.Contains(string(data), `"items":[{`) {
		t.Fatalf("row missing: %s", data)
	}
	for _, field := range []string{
		`"stablecoinValueUsd":null`,
		`"nonStablecoinValueUsd":null`,
		`"totalValueUsd":null`,
		`"unpricedAssetCount":0`,
		`"unpricedAssetSymbols":[]`,
		`"stablecoinSymbols":[]`,
	} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("missing %s in %s", field, data)
		}
	}
}

func TestExposureSummaryJSONPreservesValueComposition(t *testing.T) {
	list := financial.ExposureList{
		Context: financial.Context{Timezone: "UTC"},
		Summary: financial.ExposureSummary{
			AssetCount:            3,
			LongCount:             2,
			ShortCount:            1,
			StablecoinValueUSD:    new("1000"),
			NonStablecoinValueUSD: new("-200.5"),
			TotalValueUSD:         new("799.5"),
			UnpricedAssetCount:    1,
			UnpricedAssetSymbols:  []string{"LINK"},
			StablecoinSymbols:     []string{"USDT"},
			Completeness:          valuation.CompletenessIncomplete,
			Freshness:             valuation.FreshnessFresh,
		},
		Coverage: financial.ExposureCoverage{Complete: true, MissingAccountIDs: []uuid.UUID{}},
		Items:    []financial.ExposureRow{},
		Issues:   []valuation.Issue{},
	}
	data, err := json.Marshal(toAPIExposureList(list))
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, field := range []string{
		`"stablecoinValueUsd":"1000"`,
		`"nonStablecoinValueUsd":"-200.5"`,
		`"totalValueUsd":"799.5"`,
		`"unpricedAssetCount":1`,
		`"unpricedAssetSymbols":["LINK"]`,
		`"stablecoinSymbols":["USDT"]`,
	} {
		if !strings.Contains(encoded, field) {
			t.Fatalf("missing %s in %s", field, encoded)
		}
	}
}

func TestExposureDetailJSONPreservesContributionNulls(t *testing.T) {
	observed := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	detail := financial.ExposureDetail{
		Context:              financial.Context{Timezone: "UTC"},
		CanonicalAssetSymbol: "BTC",
		PlatformSymbols:      []string{"BTC"},
		State:                financial.ExposurePartial,
		Freshness:            valuation.FreshnessStale,
		AccountCount:         2,
		UnknownAccountCount:  1,
		Coverage:             financial.ExposureCoverage{MissingAccountIDs: []uuid.UUID{}, Complete: true},
		Contributions: []financial.ExposureContribution{{
			Platform:         "bybit",
			PlatformLabel:    "Bybit",
			IdentityID:       uuid.New(),
			AccountID:        uuid.New(),
			AccountName:      "Unified",
			AccountType:      "unified",
			AccountStructure: "bybit_uta_2_0",
			AssetSymbol:      "BTC",
			State:            financial.ExposureUnknown,
			Freshness:        valuation.FreshnessStale,
			ReportObservedAt: &observed,
			Issues: []valuation.Issue{{
				Code:   "exposure_unknown_position",
				Symbol: "BTC",
				Detail: "position inputs unavailable",
			}},
		}},
		Issues: []valuation.Issue{},
	}
	data, err := json.Marshal(toAPIExposureDetail(detail))
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, field := range []string{`"quantity":null`, `"netQuantity":null`, `"state":"partial"`, `"freshness":"stale"`} {
		if !strings.Contains(encoded, field) {
			t.Fatalf("missing %s in %s", field, encoded)
		}
	}
}
