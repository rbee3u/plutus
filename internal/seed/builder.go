package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/shared/textkey"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

const (
	alicePasswordHash = `$argon2id$v=19$m=65536,t=3,p=2$4ZKp/JFnZmxIWjXk18zPWA$3wK/YAxLVIGX9PZpBe9MxZGrZpC+pw3nVpWfK6q2s2I`
	timestampLayout   = "2006-01-02 15:04:05.000000"
	rowsPerInsert     = 400
)

// LiveIdentity is an explicit post-insert connection initialization plan.
type LiveIdentity struct {
	PortfolioID uuid.UUID
	IdentityID  uuid.UUID
	Credentials adapter.Credentials
	Status      string
}

var tableOrder = []string{"users", "portfolios", "identities", "accounts", "assets", "futures_contracts", "collection_cycles", "manual_revisions", "account_reports", "balances", "futures_positions", "earn_positions", "dual_positions", "price_observations", "capital_flows", "asset_exclusion_versions", "audit_events"}

var tableColumns = map[string]string{
	"users":                    "id, username, username_key, status, password_hash, created_at, updated_at",
	"portfolios":               "id, user_id, name, name_key, timezone, collection_interval_seconds, is_initial, version, created_at, updated_at",
	"identities":               "id, portfolio_id, platform, display_name, display_name_key, account_structure, last_observed_margin_mode, platform_login_ref, archived_at, connection_status, credentials_key_version, credentials_set_at, last_validation_succeeded_at, last_collection_finished_at, last_error_class, last_error_detail, version, created_at, updated_at",
	"accounts":                 "id, portfolio_id, identity_id, account_type, selected",
	"assets":                   "id, portfolio_id, platform, symbol, canonical_asset_symbol, mapping_version, first_observed_at, last_observed_at",
	"futures_contracts":        "id, portfolio_id, platform, symbol, terms_hash, kind, base_asset_symbol, quote_asset_symbol, settlement_asset_symbol, expiry_at, contract_size, first_observed_at, last_observed_at",
	"collection_cycles":        "id, portfolio_id, trigger_kind, status, attempt_count, available_at, created_at, started_at, collection_finished_at, finished_at, manual_reported_at",
	"account_reports":          "id, portfolio_id, cycle_id, account_id, status, collected_at, source_observed_at, recorded_at, margin_mode, error_class, error_detail, reported_equity_usd, adjusted_equity_usd, available_equity_usd, initial_margin_requirement_usd, maintenance_margin_requirement_usd, reported_initial_margin_ratio, reported_maintenance_margin_ratio, reported_margin_ratio, spot_margin_unrealized_pnl_usd, manual_revision_number",
	"balances":                 "report_id, portfolio_id, asset_id, equity, available_to_withdraw, available_to_trade, liability, interest, unrealized_pnl",
	"futures_positions":        "report_id, portfolio_id, position_key, futures_contract_id, side, source_quantity, source_quantity_unit, entry_price, mark_price, liquidation_price, unrealized_pnl, leverage, margin, position_mode, margin_setting, margin_asset_symbol, margin_basis",
	"earn_positions":           "report_id, portfolio_id, position_key, product_ref, product_name, product_type, granularity, amount_basis, status, subscribed_at, maturity_at, details_json",
	"dual_positions":           "report_id, portfolio_id, position_key, product_ref, order_ref, base_asset_symbol, quote_asset_symbol, principal_asset_symbol, principal, direction, target_price, expiry_at, annualized_rate, rate_kind, status, subscribed_at",
	"price_observations":       "id, portfolio_id, platform, requested_canonical_asset_symbol, source_market_ref, quote_canonical_asset_symbol, price, retrieved_at",
	"manual_revisions":         "portfolio_id, account_id, revision_number, changed_at",
	"capital_flows":            "id, portfolio_id, direction, canonical_asset_symbol, quantity, value_usd, effective_at, account_id, note, version, created_at, updated_at, pricing_method, pricing_attempted_at, calculation_version",
	"asset_exclusion_versions": "portfolio_id, asset_id, version, excluded, reason, effective_at",
	"audit_events":             "id, portfolio_id, actor_user_id, action, resource_type, resource_id, request_id, before_json, after_json, metadata_json, occurred_at",
}

type assetRow struct {
	portfolio            uuid.UUID
	id                   uuid.UUID
	platform             adapter.Platform
	symbol               string
	canonicalAssetSymbol string
	source               string
	first                time.Time
	last                 time.Time
}

type futuresContractRow struct {
	portfolio       uuid.UUID
	id              uuid.UUID
	hash            [32]byte
	symbol          string
	platform        adapter.Platform
	futuresContract adapter.FuturesContract
	first           time.Time
	last            time.Time
}

// builder accumulates rows per table and renders them as INSERT statements.
type builder struct {
	header              []string
	rows                map[string][][]string
	seq                 map[uint16]uint64
	assets              map[string]*assetRow
	assetKeys           []string
	futuresContracts    map[string]*futuresContractRow
	futuresContractKeys []string
	prices              map[string]bool
	live                []LiveIdentity
	identities          map[uuid.UUID]identitySpec
	portfolios          map[uuid.UUID]bool
	accountOwners       map[uuid.UUID]uuid.UUID
	accountTypes        map[uuid.UUID]adapter.AccountType
	accountIdentities   map[uuid.UUID]uuid.UUID
	manualAccounts      map[uuid.UUID]uuid.UUID
	keys                map[string]bool
	err                 error
}

func newBuilder() *builder {
	return &builder{
		rows:              map[string][][]string{},
		seq:               map[uint16]uint64{},
		assets:            map[string]*assetRow{},
		futuresContracts:  map[string]*futuresContractRow{},
		prices:            map[string]bool{},
		identities:        map[uuid.UUID]identitySpec{},
		portfolios:        map[uuid.UUID]bool{},
		accountOwners:     map[uuid.UUID]uuid.UUID{},
		accountTypes:      map[uuid.UUID]adapter.AccountType{},
		accountIdentities: map[uuid.UUID]uuid.UUID{}, manualAccounts: map[uuid.UUID]uuid.UUID{},
		keys: map[string]bool{},
	}
}

func (b *builder) comment(line string) {
	b.header = append(b.header, "-- "+line)
}

func (b *builder) next(kind uint16) uuid.UUID {
	b.seq[kind]++
	return SeedID(kind, b.seq[kind])
}

func (b *builder) add(table string, row ...string) {
	if _, exists := tableColumns[table]; !exists {
		b.fail(fmt.Errorf("unknown seed table %q", table))
		return
	}
	columns := strings.Split(tableColumns[table], ", ")
	if len(columns) != len(row) {
		b.fail(fmt.Errorf("seed %s: %d columns but %d values", table, len(columns), len(row)))
		return
	}
	key := table + "|" + row[0]
	switch table {
	case "balances", "futures_positions", "earn_positions", "dual_positions":
		key += "|" + row[2]
	case "asset_exclusion_versions", "manual_revisions":
		key += "|" + row[1] + "|" + row[2]
	}
	if b.keys[key] {
		b.fail(fmt.Errorf("seed %s: duplicate primary key", table))
		return
	}
	b.keys[key] = true
	for i, col := range columns {
		switch col {
		case "equity", "reported_equity_usd", "adjusted_equity_usd", "available_equity_usd", "initial_margin_requirement_usd", "maintenance_margin_requirement_usd", "reported_initial_margin_ratio", "reported_maintenance_margin_ratio", "reported_margin_ratio", "spot_margin_unrealized_pnl_usd", "value_usd", "available_to_withdraw", "available_to_trade", "liability", "interest", "unrealized_pnl", "quantity", "entry_price", "mark_price", "liquidation_price", "leverage", "margin", "principal", "target_price", "annualized_rate", "contract_size", "price":
			if row[i] == "NULL" {
				if table == "balances" && col == "equity" {
					b.fail(fmt.Errorf("seed balance equity must be known"))
				}
				continue
			}
			if !decimal.Valid(row[i]) {
				b.fail(fmt.Errorf("seed %s.%s: invalid decimal", table, col))
			}
		}
	}
	b.rows[table] = append(b.rows[table], row)
}

func (b *builder) fail(err error) {
	if b.err == nil {
		b.err = err
	}
}

// SQL renders every table in foreign-key order.
func (b *builder) SQL() string {
	b.flushCatalog()
	var out strings.Builder
	for _, line := range b.header {
		out.WriteString(line)
		out.WriteByte('\n')
	}
	out.WriteByte('\n')
	for _, table := range tableOrder {
		rows := b.rows[table]
		for start := 0; start < len(rows); start += rowsPerInsert {
			end := min(start+rowsPerInsert, len(rows))
			out.WriteString("INSERT INTO ")
			out.WriteString(table)
			out.WriteString(" (")
			out.WriteString(tableColumns[table])
			out.WriteString(") VALUES\n")
			for i := start; i < end; i++ {
				out.WriteString("    (")
				out.WriteString(strings.Join(rows[i], ", "))
				out.WriteString(")")
				if i+1 < end {
					out.WriteString(",\n")
				}
			}
			out.WriteString(";\n\n")
		}
	}
	return out.String()
}

func (b *builder) flushCatalog() {
	for _, key := range b.assetKeys {
		asset := b.assets[key]
		version := "1"
		if asset.source == catalog.MappingManual {
			version = "2"
		}
		b.add("assets", bin(asset.id), bin(asset.portfolio), q(string(asset.platform)), q(asset.symbol), q(asset.canonicalAssetSymbol), version, ts(asset.first), ts(asset.last))
	}
	for _, key := range b.futuresContractKeys {
		contract := b.futuresContracts[key]
		terms := contract.futuresContract
		b.add("futures_contracts", bin(contract.id), bin(contract.portfolio), q(string(contract.platform)), q(contract.symbol), "UNHEX('"+hex.EncodeToString(contract.hash[:])+"')", q(terms.Kind), qn(terms.BaseAssetSymbol), qn(terms.QuoteAssetSymbol), qn(terms.SettlementAssetSymbol), tsp(terms.ExpiryAt), decp(terms.ContractSize), ts(contract.first), ts(contract.last))
	}
	b.assetKeys = nil
	b.futuresContractKeys = nil
}

// Literal helpers.

func bin(id uuid.UUID) string {
	return "UNHEX('" + strings.ToUpper(hex.EncodeToString(id[:])) + "')"
}

func binp(id *uuid.UUID) string {
	if id == nil {
		return "NULL"
	}
	return bin(*id)
}

func q(v string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `''`).Replace(v) + "'"
}

func qn(v string) string {
	if v == "" {
		return "NULL"
	}
	return q(v)
}

func ts(t time.Time) string {
	return "'" + t.UTC().Format(timestampLayout) + "'"
}

func tsp(t *time.Time) string {
	if t == nil {
		return "NULL"
	}
	return ts(*t)
}

func dec(v string) string {
	if v == "" {
		return "NULL"
	}
	return v
}

func decp(v *string) string {
	if v == nil || *v == "" {
		return "NULL"
	}
	return *v
}

func boolean(v bool) string {
	if v {
		return "TRUE"
	}
	return "FALSE"
}

func (b *builder) jsonLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	raw, err := json.Marshal(v)
	if err != nil {
		b.fail(fmt.Errorf("serialize seed observation: %w", err))
		return "NULL"
	}
	return q(string(raw))
}

// Entity helpers.

type userSpec struct {
	ID        uuid.UUID
	Username  string
	Hash      string
	CreatedAt time.Time
}

func (b *builder) user(u userSpec) {
	b.add("users", bin(u.ID), q(u.Username), q(textkey.Key(u.Username)), q("active"), q(u.Hash), ts(u.CreatedAt), ts(u.CreatedAt))
}

type portfolioSpec struct {
	ID          uuid.UUID
	User        uuid.UUID
	Name        string
	Timezone    string
	Interval    int
	Undeletable bool
	CreatedAt   time.Time
}

func (b *builder) portfolio(p *portfolioSpec) {
	b.portfolios[p.ID] = true
	b.add("portfolios", bin(p.ID), bin(p.User), q(p.Name), q(textkey.Key(p.Name)), q(p.Timezone), strconv.Itoa(p.Interval), boolean(p.Undeletable), "1", ts(p.CreatedAt), ts(p.CreatedAt))
}

type identitySpec struct {
	ID        uuid.UUID
	Portfolio uuid.UUID
	Platform  adapter.Platform
	Structure adapter.AccountStructure
	// MarginMode is the margin mode the identity last observed; empty for structures without one.
	MarginMode  adapter.MarginMode
	Name        string
	Archived    bool
	ArchivedAt  time.Time
	APIKey      string // non-empty: credentials are sealed and the connection enabled after seeding
	LastError   string
	LastDetail  string
	CollectedAt time.Time
	CreatedAt   time.Time
	Connection  string
}

func (b *builder) identity(i *identitySpec) {
	if !b.portfolios[i.Portfolio] || !adapter.ValidStructure(i.Platform, i.Structure) || !adapter.ValidMarginMode(i.Structure, i.MarginMode) {
		b.fail(fmt.Errorf("seed identity references an invalid portfolio or capability"))
		return
	}
	var archivedAt *time.Time
	if i.Archived {
		at := i.ArchivedAt
		archivedAt = &at
	}
	var collectedAt *time.Time
	if !i.CollectedAt.IsZero() {
		at := i.CollectedAt
		collectedAt = &at
	}
	// An identity with credentials was validated once, which bound it to the account its key reaches.
	externalRef := ""
	if i.APIKey != "" {
		externalRef = fake.ExternalRef(i.APIKey)
	}
	b.add("identities", bin(i.ID), bin(i.Portfolio), q(string(i.Platform)), q(i.Name), q(textkey.Key(i.Name)), q(string(i.Structure)), qn(string(i.MarginMode)), qn(externalRef), tsp(archivedAt),
		q("unconfigured"), "NULL", "NULL", "NULL", tsp(collectedAt), qn(i.LastError), qn(i.LastDetail), "1", ts(i.CreatedAt), ts(i.CreatedAt))
	b.identities[i.ID] = *i
	if i.APIKey != "" && !i.Archived {
		creds := adapter.Credentials{APIKey: i.APIKey, APISecret: "seed-secret-" + i.APIKey, Passphrase: "seed-passphrase"}
		if i.Platform == adapter.PlatformKraken {
			creds.Futures = &adapter.KeyPair{APIKey: i.APIKey, APISecret: "seed-futures-" + i.APIKey}
		}
		b.live = append(b.live, LiveIdentity{PortfolioID: i.Portfolio, IdentityID: i.ID, Credentials: creds, Status: i.Connection})
	}
}

// accounts inserts one account per account type of the identity's structure with the given selection.
func (b *builder) accounts(identityID uuid.UUID, ids map[adapter.AccountType]uuid.UUID, selected map[adapter.AccountType]bool, createdAt time.Time) {
	spec, exists := b.identities[identityID]
	if !exists {
		b.fail(fmt.Errorf("seed account references unknown identity %s", identityID))
		return
	}
	types := append(adapter.AccountTypes(spec.Structure), adapter.AccountTypeInfo{ID: adapter.TypeManual, Name: "Manual"})
	for _, t := range types {
		id, ok := ids[t.ID]
		if !ok {
			id = b.next(kindAccount)
		}
		b.accountOwners[id], b.accountTypes[id] = spec.Portfolio, t.ID
		b.accountIdentities[id] = identityID
		if t.ID == adapter.TypeManual {
			b.manualAccounts[identityID] = id
			b.add("manual_revisions", bin(spec.Portfolio), bin(id), "1", "NULL")
		}
		b.add("accounts", bin(id), bin(spec.Portfolio), bin(identityID), q(string(t.ID)), boolean(selected[t.ID] || t.ID == adapter.TypeManual))
	}
}

func (b *builder) asset(portfolio uuid.UUID, platform adapter.Platform, symbol string, observedAt time.Time) uuid.UUID {
	key := portfolio.String() + "|" + string(platform) + "|" + symbol
	row, ok := b.assets[key]
	if !ok {
		// Retain the historical ID namespace so terminology changes do not replace seeded assets.
		row = &assetRow{portfolio: portfolio, id: namedID("asset/" + key), platform: platform, symbol: symbol, canonicalAssetSymbol: catalog.CanonicalAssetSymbolFor(symbol), source: catalog.MappingAuto, first: observedAt, last: observedAt}
		b.assets[key] = row
		b.assetKeys = append(b.assetKeys, key)
	}
	if observedAt.Before(row.first) {
		row.first = observedAt
	}
	if observedAt.After(row.last) {
		row.last = observedAt
	}
	return row.id
}

func (b *builder) futuresContract(portfolio uuid.UUID, platform adapter.Platform, in *adapter.FuturesContract, observedAt time.Time) uuid.UUID {
	terms, raw, err := catalog.NormalizeFuturesTerms(in)
	if err != nil {
		b.fail(err)
		return uuid.Nil
	}
	hash := sha256.Sum256(raw)
	key := portfolio.String() + "|" + string(platform) + "|" + in.Symbol + "|" + hex.EncodeToString(hash[:])
	row, ok := b.futuresContracts[key]
	if !ok {
		row = &futuresContractRow{portfolio: portfolio, id: namedID("futures-terms/" + key), hash: hash, symbol: in.Symbol, platform: platform, futuresContract: terms, first: observedAt, last: observedAt}
		b.futuresContracts[key] = row
		b.futuresContractKeys = append(b.futuresContractKeys, key)
	}

	if observedAt.Before(row.first) {
		row.first = observedAt
	}
	if observedAt.After(row.last) {
		row.last = observedAt
	}
	return row.id
}

func (b *builder) price(portfolio uuid.UUID, platform adapter.Platform, canonicalAssetSymbol, price string, at time.Time) {
	if price == "" {
		return
	}
	key := portfolio.String() + "|" + string(platform) + "|" + canonicalAssetSymbol + "|" + at.UTC().Format(timestampLayout)
	if b.prices[key] {
		return
	}
	b.prices[key] = true
	b.add("price_observations", bin(namedID("price/"+key)), bin(portfolio), q(string(platform)), q(canonicalAssetSymbol), q(canonicalAssetSymbol+"USDT"), q("USDT"), price, ts(at))
}

// reportSpec is one account's outcome within a cycle.
type reportSpec struct {
	AccountType adapter.AccountType
	Account     uuid.UUID
	Platform    adapter.Platform
	MarginMode  adapter.MarginMode
	Report      *adapter.AccountReport // nil means a failed report
	Class       string
	Detail      string
}

// cycle inserts one succeeded collection cycle and its account reports.
// Apply initializes its valuation point before committing these source facts.
func (b *builder) cycle(portfolioID uuid.UUID, at time.Time, reports []reportSpec) {
	cycleID := namedID("cycle/" + portfolioID.String() + "/" + at.Format(timestampLayout))
	started := at.Add(time.Second)
	finished := at.Add(4 * time.Second)
	b.add("collection_cycles", bin(cycleID), bin(portfolioID), q("schedule"), q("succeeded"), "1", ts(at), ts(at), ts(started), ts(finished), ts(finished), ts(started))
	seen := map[uuid.UUID]bool{}
	for _, report := range reports {
		identityID := b.accountIdentities[report.Account]
		if seen[identityID] {
			continue
		}
		seen[identityID] = true
		accountID := b.manualAccounts[identityID]
		reports = append(reports, reportSpec{Account: accountID, AccountType: adapter.TypeManual, Platform: report.Platform, Report: &adapter.AccountReport{AccountType: adapter.TypeManual, Status: adapter.StatusSucceeded, CollectedAt: finished}})
	}
	for i := range reports {
		r := &reports[i]
		if b.accountOwners[r.Account] != portfolioID || b.accountTypes[r.Account] != r.AccountType {
			b.fail(fmt.Errorf("seed report references an invalid account boundary"))
			continue
		}
		reportID := namedID("report/" + cycleID.String() + "/" + r.Account.String())
		if r.Report == nil {
			class := r.Class
			if class == "" {
				class = adapter.ClassTemporary
			}
			b.add("account_reports", bin(reportID), bin(portfolioID), bin(cycleID), bin(r.Account), q("failed"), "NULL", "NULL", ts(finished), "NULL", q(class), qn(r.Detail), "NULL", "NULL", "NULL", "NULL", "NULL", "NULL", "NULL", "NULL", "NULL", "NULL")
			continue
		}
		rep := r.Report
		observed := rep.CollectedAt
		if observed.IsZero() {
			observed = at.Add(2 * time.Second)
		}
		if err := validateReport(rep); err != nil {
			b.fail(fmt.Errorf("seed account %s at %s: %w", r.Account, at.Format(time.RFC3339), err))
			continue
		}
		manualRevision := "NULL"
		if r.AccountType == adapter.TypeManual {
			manualRevision = "1"
		}
		b.add("account_reports", bin(reportID), bin(portfolioID), bin(cycleID), bin(r.Account), q(adapter.StatusSucceeded), ts(observed), tsp(rep.SourceObservedAt), ts(finished), qn(string(r.MarginMode)), "NULL", "NULL", decp(rep.Equity), decp(rep.AdjustedEquity), decp(rep.AvailableEquity), decp(rep.InitialMarginRequirementUSD), decp(rep.MaintenanceMarginRequirementUSD), decp(rep.ReportedInitialMarginRatio), decp(rep.ReportedMaintenanceMarginRatio), decp(rep.ReportedMarginRatio), decp(rep.SpotMarginUnrealizedPnlUSD), manualRevision)
		for j := range rep.Balances {
			bal := &rep.Balances[j]
			assetID := b.asset(portfolioID, r.Platform, bal.AssetSymbol, observed)
			b.add("balances", bin(reportID), bin(portfolioID), bin(assetID), dec(bal.Equity), decp(bal.AvailableToWithdraw), decp(bal.AvailableToTrade), decp(bal.Liability), decp(bal.Interest), decp(bal.UnrealizedPnl))
		}
		for i := range rep.DualPositions {
			pos := &rep.DualPositions[i]
			b.add("dual_positions", bin(reportID), bin(portfolioID), q(pos.PositionKey), qp(pos.ProductRef), qp(pos.OrderRef), qp(pos.BaseAssetSymbol), qp(pos.QuoteAssetSymbol), q(pos.PrincipalAssetSymbol), pos.Principal, qp(pos.Direction), decp(pos.TargetPrice), tsp(pos.ExpiryAt), decp(pos.AnnualizedRate), qp(pos.RateKind), q(pos.Status), tsp(pos.SubscribedAt))
		}
		for _, pos := range rep.EarnPositions {
			b.add("earn_positions", bin(reportID), bin(portfolioID), q(pos.PositionKey), qp(pos.ProductRef), qp(pos.ProductName), q(pos.Type), q(pos.Granularity), q(pos.AmountBasis), qp(pos.Status), tsp(pos.SubscribedAt), tsp(pos.MaturityAt), b.jsonLiteral(adapter.EarnDetails{Assets: pos.Assets, Rates: pos.Rates, Rewards: pos.Rewards, Details: pos.Details, Issues: pos.Problems}))
		}
		for j := range rep.Positions {
			pos := &rep.Positions[j]
			futuresContractID := b.futuresContract(portfolioID, r.Platform, &pos.FuturesContract, observed)
			b.add("futures_positions", bin(reportID), bin(portfolioID), q(pos.PositionKey), bin(futuresContractID), q(pos.Side), pos.SourceQuantity, q(pos.SourceQuantityUnit), decp(pos.EntryPrice), decp(pos.MarkPrice), decp(pos.LiquidationPrice), decp(pos.UnrealizedPnl), decp(pos.Leverage), decp(pos.Margin), qn(pos.PositionMode), qn(pos.MarginSetting), qn(pos.MarginAssetSymbol), qn(pos.MarginBasis))
		}
	}
}

type flowSpec struct {
	ID                   uuid.UUID
	Portfolio            uuid.UUID
	User                 uuid.UUID
	Direction            string
	CanonicalAssetSymbol string
	Quantity             string
	ValueUSD             string // empty means unvalued
	At                   time.Time
	Account              *uuid.UUID
	Note                 string
}

func (b *builder) flow(f *flowSpec) {
	method := "explicit"
	if f.ValueUSD == "" {
		method = "unavailable"
	}
	b.add("capital_flows", bin(f.ID), bin(f.Portfolio), q(f.Direction), q(f.CanonicalAssetSymbol), f.Quantity, dec(f.ValueUSD), ts(f.At), binp(f.Account), qn(f.Note), "1", ts(f.At), ts(f.At), q(method), ts(f.At), "1")
}

type auditSpec struct {
	Portfolio    uuid.UUID
	Actor        uuid.UUID
	Action       string
	ResourceType string
	ResourceID   uuid.UUID
	After        any
	Before       any
	System       bool
	At           time.Time
}

func (b *builder) audit(a *auditSpec) {
	id := namedID("audit/" + a.Portfolio.String() + "/" + a.Action + "/" + a.ResourceID.String() + "/" + a.At.Format(timestampLayout))
	actor := bin(a.Actor)
	if a.System {
		actor = "NULL"
	}
	b.add("audit_events", bin(id), bin(a.Portfolio), actor, q(a.Action), q(a.ResourceType), bin(a.ResourceID), q("seed-"+id.String()), b.jsonLiteral(a.Before), b.jsonLiteral(a.After), "NULL", ts(a.At))
}

func qp(value *string) string {
	if value == nil {
		return "NULL"
	}
	return q(*value)
}
