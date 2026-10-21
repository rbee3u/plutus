package integration

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/capital"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/portfolio"
)

func TestFuturesContractsReuseObservedTermsAndPreserveObservationBounds(t *testing.T) {
	h := newHarness(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	terms := adapter.FuturesContract{Symbol: " btc-usdt ", Kind: adapter.KindLinearPerpetual, BaseAssetSymbol: "btc", QuoteAssetSymbol: "usdt", SettlementAssetSymbol: "USDT", ContractSize: new("1.0")}
	put := func(at time.Time) uuid.UUID {
		t.Helper()
		id, err := mysql.WithTx(h.ctx, h.db, func(q *sqlcgen.Queries) (uuid.UUID, error) {
			return h.catalog.EnsureFuturesContract(h.ctx, q, h.portfolioID, adapter.PlatformBinance, &terms, at)
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := put(at)
	terms.ContractSize = new("2")
	second := put(at.Add(time.Hour))
	terms.ContractSize = new("1.000")
	third := put(at.Add(2 * time.Hour))
	if first == second || first != third {
		t.Fatalf("A -> B -> A contracts: %s %s %s", first, second, third)
	}
	put(at.Add(-time.Hour))
	var count int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM futures_contracts").Scan(&count); err != nil || count != 2 {
		t.Fatalf("contracts=%d %v", count, err)
	}
	var from, to time.Time
	if err := h.db.QueryRowContext(h.ctx, "SELECT first_observed_at,last_observed_at FROM futures_contracts WHERE id=?", first[:]).Scan(&from, &to); err != nil || !from.Equal(at.Add(-time.Hour)) || !to.Equal(at.Add(2*time.Hour)) {
		t.Fatalf("bounds=%v %v %v", from, to, err)
	}
}

func TestIdempotencyCommitsBusinessAndResponseTogether(t *testing.T) {
	h := newHarness(t)
	store := h.services.Idempotency
	var calls atomic.Int32
	create := func(ctx context.Context) (idempotency.Record, error) {
		calls.Add(1)
		p, err := h.services.Portfolios.Create(ctx, h.userID, portfolio.CreateInput{Name: "atomic-create"})
		if err != nil {
			return idempotency.Record{}, err
		}
		body, err := json.Marshal(map[string]any{"ID": p.ID, "Version": p.Version})
		return idempotency.Record{Status: 201, Body: body}, err
	}
	var wg sync.WaitGroup
	results := make(chan idempotency.Record, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			record, err := store.Run(h.ctx, h.userID, "POST", "/portfolios", "same-key", []byte(`{"name":"atomic-create"}`), create)
			results <- record
			errs <- err
		})
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("create called %d times", calls.Load())
	}
	var body []byte
	var created struct {
		ID      uuid.UUID `json:"ID"`
		Version uint64    `json:"Version"`
	}
	for record := range results {
		if body != nil && !equalJSON(body, record.Body) {
			t.Fatal("replay changed response")
		}
		body = record.Body
		if err := json.Unmarshal(record.Body, &created); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Run(h.ctx, h.userID, "POST", "/portfolios", "same-key", []byte(`{}`), create); !errors.Is(err, idempotency.ErrConflict) {
		t.Fatalf("hash conflict: %v", err)
	}
	if err := h.services.Portfolios.Delete(h.ctx, h.userID, created.ID, created.Version); err != nil {
		t.Fatal(err)
	}
	drainPortfolioDeletion(t, h, created.ID)
	if record, err := store.Run(h.ctx, h.userID, "POST", "/portfolios", "same-key", []byte(`{"name":"atomic-create"}`), create); err != nil || !equalJSON(record.Body, body) || calls.Load() != 1 {
		t.Fatalf("replay after delete: %v", err)
	}
	// Invalid success serialization must roll back the portfolio and the key.
	_, err := store.Run(h.ctx, h.userID, "POST", "/portfolios", "rollback", nil, func(ctx context.Context) (idempotency.Record, error) {
		_, err := h.services.Portfolios.Create(ctx, h.userID, portfolio.CreateInput{Name: "must-rollback"})
		return idempotency.Record{Status: 201, Body: []byte(`invalid-json`)}, err
	})
	if err == nil {
		t.Fatal("invalid response committed")
	}
	var count int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM portfolios WHERE name='must-rollback'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("business write leaked: %d %v", count, err)
	}
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM idempotency_records WHERE idempotency_key='rollback'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("key leaked: %d %v", count, err)
	}
	if _, err := h.db.ExecContext(h.ctx, "UPDATE idempotency_records SET expires_at=? WHERE idempotency_key='same-key'", time.Now().UTC().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Run(h.ctx, h.userID, "POST", "/portfolios", "same-key", []byte(`{}`), create); err != nil || calls.Load() != 2 {
		t.Fatalf("expired key not reusable: %v", err)
	}
}

func TestNameKeysAndSymbolByteLimits(t *testing.T) {
	h := newHarness(t)
	if _, err := h.services.Portfolios.Create(h.ctx, h.userID, portfolio.CreateInput{Name: "Straße"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.services.Portfolios.Create(h.ctx, h.userID, portfolio.CreateInput{Name: "STRASSE"}); !errors.Is(err, portfolio.ErrConflict) {
		t.Fatalf("case-fold duplicate: %v", err)
	}
	a, err := catalog.NormalizeSymbol(" e\u0301th ")
	if err != nil || a != "ÉTH" {
		t.Fatalf("NFC symbol=%q %v", a, err)
	}
}

func equalJSON(a, b []byte) bool {
	var left, right any
	return json.Unmarshal(a, &left) == nil && json.Unmarshal(b, &right) == nil && reflect.DeepEqual(left, right)
}

func TestIdempotencyRejectsKeysOutsideStorageDomain(t *testing.T) {
	store := idempotency.NewStore(nil)
	for _, key := range []string{"中文", "trailing ", "embedded space", "control\tkey", string(make([]byte, 256))} {
		_, err := store.Run(t.Context(), uuid.New(), "POST", "/portfolios", key, nil, func(context.Context) (idempotency.Record, error) {
			t.Fatal("invalid key reached business mutation")
			return idempotency.Record{}, nil
		})
		if !errors.Is(err, idempotency.ErrInvalidKey) {
			t.Fatalf("invalid key: %v", err)
		}
	}
}

func TestMappingRollsBackWhenAuditCannotBeWritten(t *testing.T) {
	h := newHarness(t)
	item := manualEnabled(t, h, &manualTestAdapter{usd: "0", btc: "30000"})
	state := manualSave(t, h, item, "BTC", "1")
	h.runCycle()
	asset, err := h.catalog.GetAsset(h.ctx, h.portfolioID, state.Items[0].AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.ExecContext(h.ctx, "CREATE TRIGGER reject_mapping_audit BEFORE INSERT ON audit_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected audit failure'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = h.db.ExecContext(context.Background(), "DROP TRIGGER IF EXISTS reject_mapping_audit") })
	_, err = h.catalog.PutMapping(h.ctx, asset.ID, catalog.MappingInput{ActorUserID: h.userID, PortfolioID: h.portfolioID, MappingVersion: asset.MappingVersion, CanonicalAssetSymbol: "ETH"})
	if err == nil {
		t.Fatal("injected failure ignored")
	}
	after, err := h.catalog.GetAsset(h.ctx, h.portfolioID, asset.ID)
	if err != nil || after.CanonicalAssetSymbol != asset.CanonicalAssetSymbol || after.MappingVersion != asset.MappingVersion {
		t.Fatalf("mapping committed without audit: %+v %v", after, err)
	}
	var count int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM audit_events WHERE resource_id=? AND action='catalog.asset.map'", asset.ID[:]).Scan(&count); err != nil || count != 0 {
		t.Fatalf("mapping audit leaked: %d %v", count, err)
	}
}

func TestProductConstraintsMatchNormalizedTerms(t *testing.T) {
	h := newHarness(t)
	item := manualEnabled(t, h, &manualTestAdapter{usd: "0"})
	var account uuid.UUID
	for _, a := range item.Accounts {
		if a.AccountType == "dual_investment" {
			account = a.ID
		}
	}
	collectionCycle, _, err := h.cycles.Enqueue(h.ctx, h.portfolioID, cycle.TriggerUser)
	if err != nil {
		t.Fatal(err)
	}
	report := uuid.New()
	if _, err := h.db.ExecContext(h.ctx, "INSERT INTO account_reports (portfolio_id,id,cycle_id,account_id,status,collected_at,recorded_at) VALUES (?,?,?,?,'succeeded',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))", h.portfolioID[:], report[:], collectionCycle.ID[:], account[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.ExecContext(h.ctx, `INSERT INTO dual_positions (portfolio_id,report_id,position_key,base_asset_symbol,quote_asset_symbol,principal_asset_symbol,principal,direction,target_price,annualized_rate,rate_kind,status) VALUES (?,?,'review','BTC','USDT','BTC',1,'sell_high',30000,0.1,'apr','active')`, h.portfolioID[:], report[:]); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"target_price=0", "annualized_rate=-1", "rate_kind=NULL", "annualized_rate=NULL", "status='unknown'", "base_asset_symbol='USDT'", "principal_asset_symbol='ETH'", "direction='buy_low'"} {
		if _, err := h.db.ExecContext(h.ctx, "UPDATE dual_positions SET "+change+" WHERE report_id=?", report[:]); err == nil { // #nosec G202 -- The SET clauses are fixed test cases, never external input.
			t.Fatalf("accepted contradictory terms: %s", change)
		}
	}
	if _, err := h.db.ExecContext(h.ctx, "UPDATE dual_positions SET target_price=NULL,annualized_rate=NULL,rate_kind=NULL,base_asset_symbol=NULL,quote_asset_symbol=NULL,direction=NULL WHERE report_id=?", report[:]); err != nil {
		t.Fatalf("unknown optional terms rejected: %v", err)
	}
	at := time.Now().UTC()
	terms := adapter.FuturesContract{Symbol: "BTC-USDT", Kind: adapter.KindLinearPerpetual, ExpiryAt: &at}
	if _, _, err := catalog.NormalizeFuturesTerms(&terms); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("perpetual expiry: %v", err)
	}
}

func TestActiveLoginBindingIsLocalAndReleasedOnlyByArchive(t *testing.T) {
	h := newHarness(t)
	first := h.createIdentity("first", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	first = h.storeCredentials(first, "same-login")
	action := identity.ActionInput{ActorUserID: h.userID}
	first, _, err := h.identities.Enable(h.ctx, h.portfolioID, first.ID, first.Version, action)
	if err != nil {
		t.Fatal(err)
	}
	second := h.createIdentity("second", adapter.PlatformBinance, adapter.StructureBinancePM, "portfolio_margin")
	second = h.storeCredentials(second, "same-login")
	if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, second.ID, second.Version, action); !errors.Is(err, identity.ErrLoginConflict) {
		t.Fatalf("duplicate login: %v", err)
	}
	first, err = h.identities.ClearCredentials(h.ctx, h.portfolioID, first.ID, first.Version, action)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, second.ID, second.Version, action); !errors.Is(err, identity.ErrLoginConflict) {
		t.Fatalf("clearing credentials released binding: %v", err)
	}
	other, err := h.services.Portfolios.Create(h.ctx, h.userID, portfolio.CreateInput{Name: "independent"})
	if err != nil {
		t.Fatal(err)
	}
	copyHarness := *h
	copyHarness.portfolioID = other.ID
	third := copyHarness.createIdentity("third", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	third = copyHarness.storeCredentials(third, "same-login")
	if _, _, err := h.identities.Enable(h.ctx, other.ID, third.ID, third.Version, action); err != nil {
		t.Fatalf("same login across portfolios: %v", err)
	}
	if _, err := h.identities.Archive(h.ctx, h.portfolioID, first.ID, first.Version, action); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, second.ID, second.Version, action); err != nil {
		t.Fatalf("archive did not release binding: %v", err)
	}
}

func TestFinancialReferencesRejectForeignPortfolio(t *testing.T) {
	h := architectureHarness(t)
	_, a, _ := populatedPortfolio(t, h, "scope-a")
	_, b, _ := populatedPortfolio(t, h, "scope-b")
	for _, statement := range []string{
		"UPDATE balances SET asset_id=(SELECT id FROM assets WHERE portfolio_id=? LIMIT 1) WHERE portfolio_id=? LIMIT 1",
		"UPDATE futures_positions SET futures_contract_id=(SELECT id FROM futures_contracts WHERE portfolio_id=? LIMIT 1) WHERE portfolio_id=? LIMIT 1",
		"UPDATE manual_revision_balances SET asset_id=(SELECT id FROM assets WHERE portfolio_id=? LIMIT 1) WHERE portfolio_id=? LIMIT 1",
		"UPDATE balance_valuations SET pricing_method='market_price',value_usd=1,price_observation_id=(SELECT id FROM price_observations WHERE portfolio_id=? LIMIT 1) WHERE portfolio_id=? LIMIT 1",
		"UPDATE boundary_events SET account_id=(SELECT id FROM accounts WHERE portfolio_id=? LIMIT 1) WHERE portfolio_id=? LIMIT 1",
	} {
		if _, err := h.db.ExecContext(h.ctx, statement, b.ID[:], a.ID[:]); err == nil {
			t.Fatalf("foreign reference accepted: %s", statement)
		}
	}
	for _, statement := range []string{
		"UPDATE futures_positions SET source_quantity=0 WHERE portfolio_id=? LIMIT 1",
		"UPDATE futures_positions SET source_quantity=-1 WHERE portfolio_id=? LIMIT 1",
		"UPDATE account_reports SET status='failed',collected_at=NULL,source_observed_at=NULL,error_class='temporary',reported_equity_usd=1 WHERE portfolio_id=? LIMIT 1",
	} {
		if _, err := h.db.ExecContext(h.ctx, statement, a.ID[:]); err == nil {
			t.Fatalf("invalid financial state accepted: %s", statement)
		}
	}
	for _, change := range []string{"pricing_method='market_price',price_observation_id=NULL", "pricing_method='unavailable',value_usd=1", "pricing_method='zero_equity',value_usd=1,price_observation_id=NULL"} {
		if _, err := h.db.ExecContext(h.ctx, "UPDATE balance_valuations SET "+change+" WHERE portfolio_id=? LIMIT 1", a.ID[:]); err == nil { // #nosec G202 -- SET clauses are fixed test cases, never input.
			t.Fatalf("contradictory evidence accepted: %s", change)
		}
	}
}

func TestMaintenanceKeepsLiveSessionsAndUnexpiredReplay(t *testing.T) {
	h := newHarness(t)
	for _, key := range []string{"live", "expired"} {
		if _, err := h.services.Idempotency.Run(h.ctx, h.userID, "POST", "/maintenance", key, nil, func(context.Context) (idempotency.Record, error) {
			return idempotency.Record{Status: 201, Body: []byte(`{}`)}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.db.ExecContext(h.ctx, "UPDATE idempotency_records SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE idempotency_key='expired'"); err != nil {
		t.Fatal(err)
	}
	for _, revoked := range []bool{false, true} {
		id := uuid.New()
		var revokedAt any
		if revoked {
			revokedAt = time.Now().UTC()
		}
		if _, err := h.db.ExecContext(h.ctx, "INSERT INTO sessions(id,user_id,token_hash,csrf_secret,created_at,last_seen_at,idle_expires_at,absolute_expires_at,revoked_at) VALUES(?,?,UNHEX(SHA2(?,256)),REPEAT('a',32),UTC_TIMESTAMP(6),UTC_TIMESTAMP(6),DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR),DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", id[:], h.userID[:], id.String(), revokedAt); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.services.Auth.PruneExpired(h.ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.services.Idempotency.PruneExpired(h.ctx); err != nil {
		t.Fatal(err)
	}
	var sessions, records int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM sessions WHERE user_id=?", h.userID[:]).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM idempotency_records WHERE user_id=?", h.userID[:]).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || records != 1 {
		t.Fatalf("maintenance retained sessions=%d records=%d", sessions, records)
	}
}

func TestInitialManualRevisionCannotContainBalances(t *testing.T) {
	h := newHarness(t)
	item := manualEnabled(t, h, &manualTestAdapter{usd: "0"})
	state := manualSave(t, h, item, "USD", "-2")
	if state.Version != 2 || len(state.Items) != 1 || state.Items[0].Quantity != "-2" {
		t.Fatalf("signed revision 2: %+v", state)
	}
	_, err := h.db.ExecContext(h.ctx, `INSERT INTO manual_revision_balances
  (portfolio_id,account_id,revision_number,asset_id,quantity,changed_at)
  VALUES (?,?,1,?,1,UTC_TIMESTAMP(6))`, h.portfolioID[:], state.AccountID[:], state.Items[0].AssetID[:])
	if err == nil || !strings.Contains(err.Error(), "chk_manual_revision_balances_revision") {
		t.Fatalf("initial revision accepted a balance or failed for another reason: %v", err)
	}
}

func TestCapitalParityCannotContradictQuantity(t *testing.T) {
	h := newHarness(t)
	for _, symbol := range []string{"USD", "USDT", "USDC"} {
		flow, err := h.services.Capital.Create(h.ctx, h.portfolioID, &capital.Input{
			ActorUserID: h.userID, Direction: new(capital.DirectionContribution),
			CanonicalAssetSymbol: &symbol, Quantity: new("2.5"), EffectiveAt: new(time.Now().UTC()),
		})
		if err != nil {
			t.Fatal(err)
		}
		if flow.PricingMethod != "usd_parity" || flow.ValueUSD == nil || *flow.ValueUSD != "2.5" {
			t.Fatalf("parity flow: %+v", flow)
		}
		if _, err := h.db.ExecContext(h.ctx, "UPDATE capital_flows SET value_usd=3 WHERE id=?", flow.ID[:]); err == nil || !strings.Contains(err.Error(), "chk_capital_flows_parity") {
			t.Fatalf("contradictory parity accepted or failed for another reason: %v", err)
		}
		// An explicit override remains valid and must retain its distinct method.
		changed, err := h.services.Capital.Patch(h.ctx, h.portfolioID, flow.ID, flow.Version, &capital.Input{ActorUserID: h.userID, ValueUSD: new("3")})
		if err != nil || changed.PricingMethod != "explicit" || changed.ValueUSD == nil || *changed.ValueUSD != "3" {
			t.Fatalf("explicit override: %+v %v", changed, err)
		}

		edited, err := h.services.Capital.Patch(h.ctx, h.portfolioID, flow.ID, changed.Version, &capital.Input{ActorUserID: h.userID, Quantity: new("9"), EffectiveAt: new(time.Now().UTC().Add(-time.Hour))})
		if err != nil || edited.PricingMethod != "explicit" || edited.ValueUSD == nil || *edited.ValueUSD != "3" {
			t.Fatalf("explicit override lost on input edit: %+v %v", edited, err)
		}
		cleared, err := h.services.Capital.Patch(h.ctx, h.portfolioID, flow.ID, edited.Version, &capital.Input{ActorUserID: h.userID, ClearValue: true})
		if err != nil || cleared.PricingMethod != "usd_parity" || cleared.ValueUSD == nil || *cleared.ValueUSD != "9" {
			t.Fatalf("cleared override was not repriced: %+v %v", cleared, err)
		}
	}
}
