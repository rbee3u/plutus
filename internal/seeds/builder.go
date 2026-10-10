package seeds

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

//nolint:gosec // G101: deterministic Argon2id hashes of the documented scratch-login passwords.
const (
	alicePasswordHash = `$argon2id$v=19$m=65536,t=3,p=2$4ZKp/JFnZmxIWjXk18zPWA$3wK/YAxLVIGX9PZpBe9MxZGrZpC+pw3nVpWfK6q2s2I`
	timestampLayout   = "2006-01-02 15:04:05.000000"
	rowsPerInsert     = 400
)

// LiveIdentity is an explicit post-insert Connection initialization plan.
type LiveIdentity struct {
	PortfolioID uuid.UUID
	IdentityID  uuid.UUID
	Credentials adapters.Credentials
	Status      string
}

var tableOrder = []string{
	"users", "portfolios", "identities", "accounts", "assets", "futures_contracts", "jobs",
	"account_reports", "balances", "positions", "earn_positions", "dual_positions", "prices", "capital_flows",
	"portfolio_asset_exclusions", "portfolio_asset_exclusion_events", "audit_events",
}

var tableColumns = map[string]string{
	"users":                            "id, username, username_normalized, status, password_hash, created_at, updated_at",
	"portfolios":                       "id, user_id, name, timezone, collection_interval_seconds, undeletable, version, created_at, updated_at",
	"identities":                       "id, portfolio_id, platform, display_name, account_structure, margin_mode, external_ref, status, archived_at, connection_status, credential_key_version, credentials_updated_at, last_validated_at, last_collected_at, last_error_class, last_error_detail, version, created_at, updated_at",
	"accounts":                         "id, portfolio_id, identity_id, account_type, display_name, selected, version, created_at, updated_at",
	"assets":                           "id, platform, symbol, canonical_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at",
	"futures_contracts":                "id, platform, symbol, kind, base_asset_symbol, quote_asset_symbol, settlement_asset_symbol, expiry_at, contract_size, first_seen_at, last_seen_at, created_at, updated_at",
	"jobs":                             "id, portfolio_id, kind, status, `trigger`, dedupe_key, payload_json, requested_by_user_id, available_at, attempt_count, max_attempts, last_error_class, last_detail, created_at, updated_at, started_at, finished_at",
	"account_reports":                  "id, portfolio_id, job_id, account_id, scope_version, earn_details_collected, margin_mode, status, observed_at, error_class, error_detail, equity, adjusted_equity, available_equity, initial_margin_ratio, maintenance_margin_ratio, unified_margin_ratio, value_usd, method, source_report_id, omitted_json, created_at",
	"balances":                         "report_id, portfolio_id, asset_id, equity, available_to_withdraw, available_to_trade, liability, interest, unrealized_pnl",
	"positions":                        "report_id, portfolio_id, platform_position_ref, futures_contract_id, side, quantity, quantity_unit, entry_price, mark_price, liquidation_price, unrealized_pnl, leverage, margin, position_mode, margin_mode, margin_asset_symbol, margin_basis",
	"earn_positions":                   "report_id, portfolio_id, platform_position_ref, observation_json",
	"dual_positions":                   "report_id, portfolio_id, platform_position_ref, product_ref, order_ref, base_asset_symbol, quote_asset_symbol, invest_asset_symbol, principal, direction, target_price, expiry_at, annualized_rate, rate_kind, status, subscribed_at",
	"prices":                           "platform, canonical_asset_symbol, observed_at, price, quote_canonical_asset_symbol",
	"capital_flows":                    "id, portfolio_id, direction, canonical_asset_symbol, quantity, value_usd, effective_at, account_id, note, created_by_user_id, version, created_at, updated_at",
	"audit_events":                     "id, portfolio_id, actor_user_id, action, resource_type, resource_id, request_id, before_json, after_json, metadata_json, occurred_at",
	"portfolio_asset_exclusions":       "portfolio_id, asset_id, ignored, reason, version, actor_user_id, updated_at",
	"portfolio_asset_exclusion_events": "portfolio_id, asset_id, version, ignored, reason, actor_user_id, effective_at",
}

type assetRow struct {
	id              uuid.UUID
	platform        adapters.Platform
	symbol          string
	canonicalSymbol string
	name            string
	source          string
	first           time.Time
	last            time.Time
}

type futuresContractRow struct {
	id              uuid.UUID
	platform        adapters.Platform
	futuresContract adapters.FuturesContract
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
	accountTypes        map[uuid.UUID]adapters.AccountType
	keys                map[string]bool
	err                 error
}

func newBuilder() *builder {
	return &builder{
		rows:             map[string][][]string{},
		seq:              map[uint16]uint64{},
		assets:           map[string]*assetRow{},
		futuresContracts: map[string]*futuresContractRow{},
		prices:           map[string]bool{},
		identities:       map[uuid.UUID]identitySpec{},
		portfolios:       map[uuid.UUID]bool{},
		accountOwners:    map[uuid.UUID]uuid.UUID{},
		accountTypes:     map[uuid.UUID]adapters.AccountType{},
		keys:             map[string]bool{},
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
	case "balances", "positions", "earn_positions", "dual_positions":
		key += "|" + row[2]
	case "prices", "portfolio_asset_exclusion_events":
		key += "|" + row[1] + "|" + row[2]
	case "portfolio_asset_exclusions":
		key += "|" + row[1]
	}
	if b.keys[key] {
		b.fail(fmt.Errorf("seed %s: duplicate primary key", table))
		return
	}
	b.keys[key] = true
	for i, col := range columns {
		switch col {
		case "equity", "adjusted_equity", "available_equity", "initial_margin_ratio", "maintenance_margin_ratio", "unified_margin_ratio", "value_usd", "available_to_withdraw", "available_to_trade", "liability", "interest", "unrealized_pnl", "quantity", "entry_price", "mark_price", "liquidation_price", "leverage", "margin", "principal", "target_price", "annualized_rate", "contract_size", "price":
			if row[i] == "NULL" {
				if table == "balances" && col == "equity" {
					b.fail(fmt.Errorf("seed Balance equity must be known"))
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
		b.add("assets", bin(asset.id), q(string(asset.platform)), q(asset.symbol), q(asset.canonicalSymbol), qn(asset.name), q(asset.source), ts(asset.first), ts(asset.last), version, ts(asset.first), ts(asset.last))
	}
	for _, key := range b.futuresContractKeys {
		contract := b.futuresContracts[key]
		b.add("futures_contracts", bin(contract.id), q(string(contract.platform)), q(contract.futuresContract.Symbol), q(contract.futuresContract.Kind), qn(contract.futuresContract.BaseAssetSymbol), qn(contract.futuresContract.QuoteAssetSymbol), qn(contract.futuresContract.SettlementAssetSymbol), tsp(contract.futuresContract.ExpiryAt), decp(contract.futuresContract.ContractSize), ts(contract.first), ts(contract.last), ts(contract.first), ts(contract.last))
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
	b.add("users", bin(u.ID), q(u.Username), q(strings.ToLower(u.Username)), q("active"), q(u.Hash), ts(u.CreatedAt), ts(u.CreatedAt))
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
	b.add("portfolios", bin(p.ID), bin(p.User), q(p.Name), q(p.Timezone), strconv.Itoa(p.Interval), boolean(p.Undeletable), "1", ts(p.CreatedAt), ts(p.CreatedAt))
}

type identitySpec struct {
	ID        uuid.UUID
	Portfolio uuid.UUID
	Platform  adapters.Platform
	Structure adapters.AccountStructure
	// MarginMode is the Margin Mode the Identity last observed; empty for structures without one.
	MarginMode  adapters.MarginMode
	Name        string
	Archived    bool
	ArchivedAt  time.Time
	APIKey      string // non-empty: credentials are sealed and the Connection enabled after seeding
	LastError   string
	LastDetail  string
	CollectedAt time.Time
	CreatedAt   time.Time
	Connection  string
}

func (b *builder) identity(i *identitySpec) {
	if !b.portfolios[i.Portfolio] || !adapters.ValidStructure(i.Platform, i.Structure) || !adapters.ValidMarginMode(i.Structure, i.MarginMode) {
		b.fail(fmt.Errorf("seed Identity references an invalid Portfolio or capability"))
		return
	}
	status := "enabled"
	var archivedAt *time.Time
	if i.Archived {
		status = "archived"
		at := i.ArchivedAt
		archivedAt = &at
	}
	var collectedAt *time.Time
	if !i.CollectedAt.IsZero() {
		at := i.CollectedAt
		collectedAt = &at
	}
	// An Identity with credentials was validated once, which bound it to the account its key reaches.
	externalRef := ""
	if i.APIKey != "" {
		externalRef = fake.ExternalRef(i.APIKey)
	}
	b.add("identities", bin(i.ID), bin(i.Portfolio), q(string(i.Platform)), q(i.Name), q(string(i.Structure)), qn(string(i.MarginMode)), qn(externalRef), q(status), tsp(archivedAt),
		q("unconfigured"), "0", "NULL", "NULL", tsp(collectedAt), qn(i.LastError), qn(i.LastDetail), "1", ts(i.CreatedAt), ts(i.CreatedAt))
	b.identities[i.ID] = *i
	if i.APIKey != "" && !i.Archived {
		creds := adapters.Credentials{APIKey: i.APIKey, APISecret: "seed-secret-" + i.APIKey, Passphrase: "seed-passphrase"}
		if i.Platform == adapters.PlatformKraken {
			creds.Futures = &adapters.KeyPair{APIKey: i.APIKey, APISecret: "seed-futures-" + i.APIKey}
		}
		b.live = append(b.live, LiveIdentity{PortfolioID: i.Portfolio, IdentityID: i.ID, Credentials: creds, Status: i.Connection})
	}
}

// accounts inserts one Account per Account Type of the Identity's structure with the given selection.
func (b *builder) accounts(identityID uuid.UUID, ids map[adapters.AccountType]uuid.UUID, selected map[adapters.AccountType]bool, createdAt time.Time) {
	spec, exists := b.identities[identityID]
	if !exists {
		b.fail(fmt.Errorf("seed Account references unknown Identity %s", identityID))
		return
	}
	types := append(adapters.AccountTypes(spec.Structure), adapters.AccountTypeInfo{ID: adapters.TypeManual, Name: "Manual"})
	for _, t := range types {
		id, ok := ids[t.ID]
		if !ok {
			id = b.next(kindAccount)
		}
		b.accountOwners[id], b.accountTypes[id] = spec.Portfolio, t.ID
		b.add("accounts", bin(id), bin(spec.Portfolio), bin(identityID), q(string(t.ID)), q(t.Name), boolean(selected[t.ID] || t.ID == adapters.TypeManual), "1", ts(createdAt), ts(createdAt))
	}
}

func (b *builder) asset(platform adapters.Platform, symbol, name string, seenAt time.Time) uuid.UUID {
	key := string(platform) + "|" + symbol
	row, ok := b.assets[key]
	if !ok {
		// Retain the historical ID namespace so terminology changes do not replace seeded Assets.
		row = &assetRow{id: namedID("listing/" + key), platform: platform, symbol: symbol, canonicalSymbol: catalog.CanonicalAssetSymbolFor(symbol), name: name, source: catalog.MappingAuto, first: seenAt, last: seenAt}
		b.assets[key] = row
		b.assetKeys = append(b.assetKeys, key)
	}
	if seenAt.Before(row.first) {
		row.first = seenAt
	}
	if seenAt.After(row.last) {
		row.last = seenAt
	}
	if row.name == "" {
		row.name = name
	}
	return row.id
}

func (b *builder) futuresContract(platform adapters.Platform, in *adapters.FuturesContract, seenAt time.Time) uuid.UUID {
	key := string(platform) + "|" + in.Symbol
	row, ok := b.futuresContracts[key]
	if !ok {
		// Preserve the historical namespace of seeded Futures Contract IDs.
		row = &futuresContractRow{id: namedID("instrument/" + key), platform: platform, futuresContract: *in, first: seenAt, last: seenAt}
		b.futuresContracts[key] = row
		b.futuresContractKeys = append(b.futuresContractKeys, key)
	}
	if seenAt.Before(row.first) {
		row.first = seenAt
	}
	if seenAt.After(row.last) {
		row.last = seenAt
	}
	return row.id
}

func (b *builder) price(platform adapters.Platform, canonicalAssetSymbol, price string, at time.Time) {
	if price == "" {
		return
	}
	key := string(platform) + "|" + canonicalAssetSymbol + "|" + at.UTC().Format(timestampLayout)
	if b.prices[key] {
		return
	}
	b.prices[key] = true
	b.add("prices", q(string(platform)), q(canonicalAssetSymbol), ts(at), price, q("USDT"))
}

// reportSpec is one Account's outcome within a cycle.
type reportSpec struct {
	AccountType adapters.AccountType
	Account     uuid.UUID
	Platform    adapters.Platform
	MarginMode  adapters.MarginMode
	Report      *adapters.AccountReport // nil means a failed report
	Class       string
	Detail      string
	Scope       uint16
}

// cycle inserts one succeeded Collection Cycle job and its Account Reports.
// Valuation columns are left for valuation.Rebuild, which the seed command runs afterwards.
func (b *builder) cycle(portfolioID uuid.UUID, at time.Time, reports []reportSpec) {
	jobID := namedID("cycle/" + portfolioID.String() + "/" + at.Format(timestampLayout))
	started := at.Add(time.Second)
	finished := at.Add(4 * time.Second)
	b.add("jobs", bin(jobID), bin(portfolioID), q("collection_cycle"), q("succeeded"), q("schedule"), q("cycle"), "NULL", "NULL", ts(at), "1", "3", "NULL", "NULL", ts(at), ts(finished), ts(started), ts(finished))
	for i := range reports {
		r := &reports[i]
		if b.accountOwners[r.Account] != portfolioID || b.accountTypes[r.Account] != r.AccountType {
			b.fail(fmt.Errorf("seed report references an invalid Account boundary"))
			continue
		}
		reportID := namedID("report/" + jobID.String() + "/" + r.Account.String())
		scope := r.Scope
		if scope == 0 {
			scope = adapters.ReportScope(r.Platform, r.AccountType)
		}
		if r.Report == nil {
			class := r.Class
			if class == "" {
				class = adapters.ClassTemporary
			}
			b.add("account_reports", bin(reportID), bin(portfolioID), bin(jobID), bin(r.Account), strconv.Itoa(int(scope)), "FALSE", "NULL", q("failed"), "NULL", q(class), qn(r.Detail),
				"NULL", "NULL", "NULL", "NULL", "NULL", "NULL", "NULL", q("missing"), "NULL", "NULL", ts(finished))
			continue
		}
		rep := r.Report
		observed := rep.ObservedAt
		if observed.IsZero() {
			observed = at.Add(2 * time.Second)
		}
		if err := validateReport(rep); err != nil {
			b.fail(fmt.Errorf("seed Account %s at %s: %w", r.Account, at.Format(time.RFC3339), err))
			continue
		}
		b.add("account_reports", bin(reportID), bin(portfolioID), bin(jobID), bin(r.Account), strconv.Itoa(int(scope)), boolean(rep.EarnDetailsCollected), qn(string(r.MarginMode)), q("ok"), ts(observed), "NULL", "NULL",
			decp(rep.Equity), decp(rep.AdjustedEquity), decp(rep.AvailableEquity), decp(rep.InitialMarginRatio), decp(rep.MaintenanceMarginRatio), decp(rep.UnifiedMarginRatio),
			"NULL", q("missing"), "NULL", "NULL", ts(finished))
		for j := range rep.Balances {
			bal := &rep.Balances[j]
			assetID := b.asset(r.Platform, bal.AssetSymbol, bal.Name, observed)
			b.add("balances", bin(reportID), bin(portfolioID), bin(assetID), dec(bal.Equity), decp(bal.AvailableToWithdraw), decp(bal.AvailableToTrade), decp(bal.Liability), decp(bal.Interest), decp(bal.UnrealizedPnl))
		}
		for i := range rep.DualPositions {
			pos := &rep.DualPositions[i]
			b.add("dual_positions", bin(reportID), bin(portfolioID), q(pos.PlatformPositionRef), qp(pos.ProductRef), qp(pos.OrderRef), qp(pos.BaseAssetSymbol), qp(pos.QuoteAssetSymbol), q(pos.InvestAssetSymbol), pos.Principal, qp(pos.Direction), decp(pos.TargetPrice), tsp(pos.ExpiryAt), decp(pos.AnnualizedRate), qp(pos.RateKind), q(pos.Status), tsp(pos.SubscribedAt))
		}
		for _, pos := range rep.EarnPositions {
			b.add("earn_positions", bin(reportID), bin(portfolioID), q(pos.PlatformPositionRef), b.jsonLiteral(pos))
		}
		for j := range rep.Positions {
			pos := &rep.Positions[j]
			futuresContractID := b.futuresContract(r.Platform, &pos.FuturesContract, observed)
			b.add("positions", bin(reportID), bin(portfolioID), q(pos.PlatformPositionRef), bin(futuresContractID), q(pos.Side), pos.Quantity, q(pos.QuantityUnit), decp(pos.EntryPrice), decp(pos.MarkPrice), decp(pos.LiquidationPrice), decp(pos.UnrealizedPnl), decp(pos.Leverage), decp(pos.Margin), qn(pos.PositionMode), qn(pos.MarginMode), qn(pos.MarginAssetSymbol), qn(pos.MarginBasis))
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
	b.add("capital_flows", bin(f.ID), bin(f.Portfolio), q(f.Direction), q(f.CanonicalAssetSymbol), f.Quantity, dec(f.ValueUSD), ts(f.At), binp(f.Account), qn(f.Note), bin(f.User), "1", ts(f.At), ts(f.At))
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
