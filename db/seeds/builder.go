package seeds

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/catalog"
)

//nolint:gosec // G101: deterministic Argon2id hashes of the documented scratch-login passwords.
const (
	alicePasswordHash = `$argon2id$v=19$m=65536,t=3,p=2$4ZKp/JFnZmxIWjXk18zPWA$3wK/YAxLVIGX9PZpBe9MxZGrZpC+pw3nVpWfK6q2s2I`
	bobPasswordHash   = `$argon2id$v=19$m=65536,t=3,p=2$2Iobo6Ik3eLKnVXVTJvdKg$6yClSIX/cdFRxSZXqB1LUHEDpU0ah/bdDgC+pIu4EW8`
	timestampLayout   = "2006-01-02 15:04:05.000000"
	rowsPerInsert     = 400
)

// LiveIdentity is a seeded Identity whose credentials are sealed and enabled after the SQL runs.
type LiveIdentity struct {
	PortfolioID uuid.UUID
	IdentityID  uuid.UUID
	Credentials adapters.Credentials
}

var tableOrder = []string{
	"users", "portfolios", "identities", "accounts", "asset_listings", "instruments", "jobs",
	"account_reports", "balances", "positions", "prices", "capital_flows", "audit_events",
}

var tableColumns = map[string]string{
	"users":           "id, username, username_normalized, status, password_hash, created_at, updated_at",
	"portfolios":      "id, user_id, name, timezone, collection_interval_seconds, undeletable, version, created_at, updated_at",
	"identities":      "id, portfolio_id, platform, display_name, account_structure, margin_mode, external_ref, status, archived_at, connection_status, credential_key_version, credentials_updated_at, last_validated_at, last_collected_at, last_error_class, last_error_detail, version, created_at, updated_at",
	"accounts":        "id, portfolio_id, identity_id, account_type, display_name, selected, version, created_at, updated_at",
	"asset_listings":  "id, platform, symbol, asset_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at",
	"instruments":     "id, platform, symbol, kind, base_symbol, quote_symbol, settlement_symbol, expiry_at, contract_size, first_seen_at, last_seen_at, created_at, updated_at",
	"jobs":            "id, portfolio_id, kind, status, `trigger`, dedupe_key, payload_json, requested_by_user_id, available_at, attempt_count, max_attempts, last_error_class, last_detail, created_at, updated_at, started_at, finished_at",
	"account_reports": "id, portfolio_id, job_id, account_id, status, observed_at, error_class, error_detail, equity, adjusted_equity, available_equity, initial_margin_ratio, maintenance_margin_ratio, unified_margin_ratio, value_usd, method, source_report_id, omitted_json, created_at",
	"balances":        "report_id, portfolio_id, listing_id, equity, available_to_withdraw, available_to_trade, liability, interest, unrealized_pnl",
	"positions":       "report_id, portfolio_id, platform_position_ref, instrument_id, side, quantity, quantity_unit, entry_price, mark_price, liquidation_price, unrealized_pnl, leverage, margin",
	"prices":          "platform, asset_symbol, observed_at, price, quote_symbol",
	"capital_flows":   "id, portfolio_id, direction, asset_symbol, quantity, value_usd, effective_at, account_id, note, created_by_user_id, version, created_at, updated_at",
	"audit_events":    "id, portfolio_id, actor_user_id, action, resource_type, resource_id, request_id, before_json, after_json, metadata_json, occurred_at",
}

type listingRow struct {
	id       uuid.UUID
	platform adapters.Platform
	symbol   string
	asset    string
	name     string
	source   string
	first    time.Time
	last     time.Time
}

type instrumentRow struct {
	id         uuid.UUID
	platform   adapters.Platform
	instrument adapters.Instrument
	first      time.Time
	last       time.Time
}

// builder accumulates rows per table and renders them as INSERT statements.
type builder struct {
	header      []string
	rows        map[string][][]string
	seq         map[uint16]uint64
	listings    map[string]*listingRow
	listingKeys []string
	instruments map[string]*instrumentRow
	instrKeys   []string
	prices      map[string]bool
	live        []LiveIdentity
	identities  map[uuid.UUID]identitySpec
}

func newBuilder() *builder {
	return &builder{
		rows:        map[string][][]string{},
		seq:         map[uint16]uint64{},
		listings:    map[string]*listingRow{},
		instruments: map[string]*instrumentRow{},
		prices:      map[string]bool{},
		identities:  map[uuid.UUID]identitySpec{},
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
	b.rows[table] = append(b.rows[table], row)
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
	for _, key := range b.listingKeys {
		l := b.listings[key]
		b.add("asset_listings", bin(l.id), q(string(l.platform)), q(l.symbol), q(l.asset), qn(l.name), q(l.source), ts(l.first), ts(l.last), "1", ts(l.first), ts(l.last))
	}
	for _, key := range b.instrKeys {
		i := b.instruments[key]
		b.add("instruments", bin(i.id), q(string(i.platform)), q(i.instrument.Symbol), q(i.instrument.Kind), qn(i.instrument.BaseSymbol), qn(i.instrument.QuoteSymbol), qn(i.instrument.SettlementSymbol), tsp(i.instrument.ExpiryAt), decp(i.instrument.ContractSize), ts(i.first), ts(i.last), ts(i.first), ts(i.last))
	}
	b.listingKeys = nil
	b.instrKeys = nil
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

func jsonLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	raw, err := json.Marshal(v)
	if err != nil {
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
}

func (b *builder) identity(i *identitySpec) {
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
		b.live = append(b.live, LiveIdentity{PortfolioID: i.Portfolio, IdentityID: i.ID, Credentials: adapters.Credentials{APIKey: i.APIKey, APISecret: "seed-secret-" + i.APIKey, Passphrase: "seed-passphrase"}})
	}
}

// accounts inserts one Account per Account Type of the Identity's structure with the given selection.
func (b *builder) accounts(identityID uuid.UUID, ids map[adapters.AccountType]uuid.UUID, selected map[adapters.AccountType]bool, createdAt time.Time) {
	spec := b.identities[identityID]
	for _, t := range adapters.AccountTypes(spec.Structure) {
		id, ok := ids[t.ID]
		if !ok {
			id = b.next(kindAccount)
		}
		b.add("accounts", bin(id), bin(spec.Portfolio), bin(identityID), q(string(t.ID)), q(t.Name), boolean(selected[t.ID]), "1", ts(createdAt), ts(createdAt))
	}
}

func (b *builder) listing(platform adapters.Platform, symbol, name string, seenAt time.Time) uuid.UUID {
	key := string(platform) + "|" + symbol
	row, ok := b.listings[key]
	if !ok {
		row = &listingRow{id: b.next(kindListing), platform: platform, symbol: symbol, asset: catalog.AssetSymbolFor(symbol), name: name, source: catalog.MappingAuto, first: seenAt, last: seenAt}
		b.listings[key] = row
		b.listingKeys = append(b.listingKeys, key)
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

// mapListing records a manual Asset override for one listing.
func (b *builder) mapListing(platform adapters.Platform, symbol, asset string) {
	key := string(platform) + "|" + symbol
	if row, ok := b.listings[key]; ok {
		row.asset = asset
		row.source = catalog.MappingManual
	}
}

func (b *builder) instrument(platform adapters.Platform, in *adapters.Instrument, seenAt time.Time) uuid.UUID {
	key := string(platform) + "|" + in.Symbol
	row, ok := b.instruments[key]
	if !ok {
		row = &instrumentRow{id: b.next(kindInstrument), platform: platform, instrument: *in, first: seenAt, last: seenAt}
		b.instruments[key] = row
		b.instrKeys = append(b.instrKeys, key)
	}
	if seenAt.Before(row.first) {
		row.first = seenAt
	}
	if seenAt.After(row.last) {
		row.last = seenAt
	}
	return row.id
}

func (b *builder) price(platform adapters.Platform, asset, price string, at time.Time) {
	if price == "" {
		return
	}
	key := string(platform) + "|" + asset + "|" + at.UTC().Format(timestampLayout)
	if b.prices[key] {
		return
	}
	b.prices[key] = true
	b.add("prices", q(string(platform)), q(asset), ts(at), price, q("USDT"))
}

// reportSpec is one Account's outcome within a cycle.
type reportSpec struct {
	Account  uuid.UUID
	Platform adapters.Platform
	Report   *adapters.AccountReport // nil means a failed report
	Class    string
	Detail   string
}

// cycle inserts one succeeded Collection Cycle job and its Account Reports.
// Valuation columns are left for valuation.Rebuild, which the seed command runs afterwards.
func (b *builder) cycle(portfolioID uuid.UUID, at time.Time, reports []reportSpec) {
	jobID := b.next(kindJob)
	started := at.Add(time.Second)
	finished := at.Add(4 * time.Second)
	b.add("jobs", bin(jobID), bin(portfolioID), q("collection_cycle"), q("succeeded"), q("schedule"), q("cycle"), "NULL", "NULL", ts(at), "1", "3", "NULL", "NULL", ts(at), ts(finished), ts(started), ts(finished))
	for i := range reports {
		r := &reports[i]
		reportID := b.next(kindReport)
		if r.Report == nil {
			class := r.Class
			if class == "" {
				class = adapters.ClassTemporary
			}
			b.add("account_reports", bin(reportID), bin(portfolioID), bin(jobID), bin(r.Account), q("failed"), "NULL", q(class), qn(r.Detail),
				"NULL", "NULL", "NULL", "NULL", "NULL", "NULL", "NULL", q("missing"), "NULL", "NULL", ts(finished))
			continue
		}
		rep := r.Report
		observed := rep.ObservedAt
		if observed.IsZero() {
			observed = at.Add(2 * time.Second)
		}
		b.add("account_reports", bin(reportID), bin(portfolioID), bin(jobID), bin(r.Account), q("ok"), ts(observed), "NULL", "NULL",
			decp(rep.Equity), decp(rep.AdjustedEquity), decp(rep.AvailableEquity), decp(rep.InitialMarginRatio), decp(rep.MaintenanceMarginRatio), decp(rep.UnifiedMarginRatio),
			"NULL", q("missing"), "NULL", "NULL", ts(finished))
		for j := range rep.Balances {
			bal := &rep.Balances[j]
			listingID := b.listing(r.Platform, bal.Symbol, bal.Name, observed)
			b.add("balances", bin(reportID), bin(portfolioID), bin(listingID), dec(bal.Equity), decp(bal.AvailableToWithdraw), decp(bal.AvailableToTrade), decp(bal.Liability), decp(bal.Interest), decp(bal.UnrealizedPnl))
		}
		for j := range rep.Positions {
			pos := &rep.Positions[j]
			instrumentID := b.instrument(r.Platform, &pos.Instrument, observed)
			b.add("positions", bin(reportID), bin(portfolioID), q(pos.PlatformPositionRef), bin(instrumentID), q(pos.Side), pos.Quantity, q(pos.QuantityUnit), decp(pos.EntryPrice), decp(pos.MarkPrice), decp(pos.LiquidationPrice), decp(pos.UnrealizedPnl), decp(pos.Leverage), decp(pos.Margin))
		}
	}
}

type flowSpec struct {
	ID        uuid.UUID
	Portfolio uuid.UUID
	User      uuid.UUID
	Direction string
	Asset     string
	Quantity  string
	ValueUSD  string // empty means unvalued
	At        time.Time
	Account   *uuid.UUID
	Note      string
}

func (b *builder) flow(f *flowSpec) {
	b.add("capital_flows", bin(f.ID), bin(f.Portfolio), q(f.Direction), q(f.Asset), f.Quantity, dec(f.ValueUSD), ts(f.At), binp(f.Account), qn(f.Note), bin(f.User), "1", ts(f.At), ts(f.At))
}

type auditSpec struct {
	Portfolio    uuid.UUID
	Actor        uuid.UUID
	Action       string
	ResourceType string
	ResourceID   uuid.UUID
	After        any
	At           time.Time
}

func (b *builder) audit(a *auditSpec) {
	id := b.next(kindAudit)
	b.add("audit_events", bin(id), bin(a.Portfolio), bin(a.Actor), q(a.Action), q(a.ResourceType), bin(a.ResourceID), q("seed-"+fmt.Sprintf("%04d", b.seq[kindAudit])), "NULL", jsonLiteral(a.After), "NULL", ts(a.At))
}
