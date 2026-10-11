package seeds

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
)

// Seed contains source SQL and the ownership needed to initialize valuations and Connections.
type Seed struct {
	SQL        string
	Live       []LiveIdentity
	Portfolios []uuid.UUID
}

// Build generates the walkthrough anchored at the current time.
func Build() (Seed, error) { return BuildAt(time.Now()) }

// BuildAt generates deterministic source facts. No derived values are inserted.
func BuildAt(now time.Time) (Seed, error) {
	if now.IsZero() || now.Year() < 1972 || now.Year() > 9998 {
		return Seed{}, fmt.Errorf("seed anchor is outside the supported calendar")
	}
	anchor := now.UTC().Truncate(30 * time.Minute)
	b := newBuilder()
	b.comment("Deterministic offline walkthrough; synthetic values, not recorded market history")
	b.comment("Scratch login: alice / Alice=123456")
	b.comment("Apply initializes Valuation Points from normalized Account Reports before commit.")
	created := anchor.AddDate(0, -3, -10)
	b.user(userSpec{ID: UserAlice, Username: "alice", Hash: alicePasswordHash, CreatedAt: created})
	portfolios := []portfolioSpec{
		{ID: PortfolioAliceMain, User: UserAlice, Name: "default", Timezone: "Asia/Shanghai", Interval: 1800, Undeletable: true, CreatedAt: created},
		{ID: PortfolioAliceEdge, User: UserAlice, Name: "edge-cases", Timezone: "Europe/Berlin", Interval: 1800, CreatedAt: created},
	}
	definitions := fake.DemoIdentities()
	seen := map[string]bool{}
	for _, d := range definitions {
		if err := fake.ValidateDemoIdentity(d); err != nil {
			return Seed{}, err
		}
		if seen[d.ID] {
			return Seed{}, fmt.Errorf("duplicate demo identity %q", d.ID)
		}
		seen[d.ID] = true
	}
	ids := make([]uuid.UUID, 0, len(portfolios))
	for _, p := range portfolios {
		b.portfolio(&p)
		ids = append(ids, p.ID)
		for _, d := range definitions {
			if portfolioID(d.Portfolio) == p.ID {
				created := p.CreatedAt
				if d.Portfolio == "large" {
					created = anchor.Add(-2 * time.Hour)
				}
				b.demoIdentity(d, anchor, created)
			}
		}
		loc, err := time.LoadLocation(p.Timezone)
		if err != nil {
			return Seed{}, fmt.Errorf("seed timezone: %w", err)
		}
		for _, at := range cycleTimes(anchor, loc) {
			if p.Name == "edge-cases" && sameLocalDay(at, anchor.AddDate(0, 0, -4), loc) {
				continue
			}
			var reports []reportSpec
			for _, d := range definitions {
				if portfolioID(d.Portfolio) != p.ID || (d.Portfolio == "large" && at.Before(anchor.Add(-2*time.Hour))) {
					continue
				}
				batch, err := fake.DemoBatch(d, anchor, at.Add(2*time.Second), fake.DemoAccounts(d, anchor, at))
				if err != nil {
					return Seed{}, fmt.Errorf("seed %s: %w", d.ID, err)
				}
				for _, price := range batch.Prices {
					b.price(p.ID, d.Platform, price.CanonicalAssetSymbol, price.Price, price.RetrievedAt)
				}
				for _, report := range batch.Reports {
					r := reportSpec{Account: accountID(d.ID, string(report.AccountType)), AccountType: report.AccountType, Platform: d.Platform, MarginMode: batch.MarginMode}
					if report.Status == adapters.StatusSucceeded {
						r.Report = &report
					} else {
						r.Class, r.Detail = report.ErrorClass, report.ErrorDetail
					}
					reports = append(reports, r)
				}
			}
			b.cycle(p.ID, at, reports)
		}
	}
	b.demoEvents(anchor)
	sql := b.SQL()
	if b.err != nil {
		return Seed{}, b.err
	}
	return Seed{SQL: sql, Live: b.live, Portfolios: ids}, nil
}

func (b *builder) demoIdentity(d fake.DemoIdentity, anchor, created time.Time) {
	key := fake.DemoKey(d.ID, anchor)
	if d.Connection == "unconfigured" || d.Profile == "archived" {
		key = ""
	}
	collected := anchor
	if d.Profile == "missing" || d.Profile == "unconfigured" || d.Profile == "normalization" {
		collected = time.Time{}
	}
	if d.Connection == "disabled" || d.Connection == "invalid" || d.Profile == "stale" || d.Profile == "stale_incomplete" || d.Profile == "unsupported" {
		collected = anchor.AddDate(0, 0, -2).Add(-2 * time.Hour)
	}
	b.identity(&identitySpec{ID: identityID(d.ID), Portfolio: portfolioID(d.Portfolio), Platform: d.Platform, Structure: d.Structure, MarginMode: d.Mode, Name: d.Name, APIKey: key, Connection: d.Connection, Archived: d.Profile == "archived", ArchivedAt: anchor.AddDate(0, 0, -2), CreatedAt: created, CollectedAt: collected})
	accounts := map[adapters.AccountType]uuid.UUID{}
	selected := map[adapters.AccountType]bool{}
	for _, kind := range adapters.AccountTypeIDs(d.Structure) {
		accounts[kind] = accountID(d.ID, string(kind))
	}
	for _, kind := range fake.DemoAccounts(d, anchor, anchor) {
		selected[kind] = true
	}
	b.accounts(identityID(d.ID), accounts, selected, created)
	b.audit(&auditSpec{Portfolio: portfolioID(d.Portfolio), Actor: UserAlice, Action: "identity.create", ResourceType: "identity", ResourceID: identityID(d.ID), At: created, After: map[string]any{"displayName": d.Name, "platform": d.Platform, "accountStructure": d.Structure}})
}
