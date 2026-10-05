package httpserver

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
	"github.com/rbee3u/plutus/internal/valuation"
)

func toAPIPortfolio(item *portfolio.Portfolio) api.Portfolio {
	return api.Portfolio{
		Id:                        item.ID,
		Name:                      item.Name,
		Timezone:                  item.Timezone,
		CollectionIntervalSeconds: item.CollectionIntervalSeconds,
		Undeletable:               item.Undeletable,
		Version:                   int(item.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		CreatedAt:                 item.CreatedAt,
		UpdatedAt:                 item.UpdatedAt,
	}
}

func toAPIIdentity(item *identity.Identity) api.Identity {
	accounts := make([]api.Account, 0, len(item.Accounts))
	for i := range item.Accounts {
		accounts = append(accounts, toAPIAccount(&item.Accounts[i]))
	}
	return api.Identity{
		Id:               item.ID,
		PortfolioId:      item.PortfolioID,
		Platform:         string(item.Platform),
		DisplayName:      item.DisplayName,
		AccountStructure: string(item.AccountStructure),
		MarginMode:       strPtr(string(item.MarginMode)),
		ExternalRef:      item.ExternalRef,
		Status:           api.IdentityStatus(item.Status),
		ArchivedAt:       item.ArchivedAt,
		Connection: api.Connection{
			Status:                api.ConnectionStatus(item.Connection.Status),
			CredentialsConfigured: item.Connection.CredentialsConfigured,
			CredentialsUpdatedAt:  item.Connection.CredentialsUpdatedAt,
			LastValidatedAt:       item.Connection.LastValidatedAt,
			LastCollectedAt:       item.Connection.LastCollectedAt,
			LastErrorClass:        item.Connection.LastErrorClass,
			LastErrorDetail:       item.Connection.LastErrorDetail,
		},
		Accounts:  accounts,
		Version:   int(item.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func toAPIAccount(item *identity.Account) api.Account {
	return api.Account{
		Id:          item.ID,
		IdentityId:  item.IdentityID,
		AccountType: string(item.AccountType),
		DisplayName: item.DisplayName,
		Selected:    item.Selected,
		Version:     int(item.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
	}
}

func toAPIOutcome(item *identity.Identity, outcome *identity.ValidationOutcome) api.ValidationOutcome {
	return api.ValidationOutcome{
		Ok:          outcome.OK,
		ExternalRef: strPtr(outcome.ExternalRef),
		ErrorClass:  strPtr(outcome.ErrorClass),
		ErrorDetail: strPtr(outcome.ErrorDetail),
		ValidatedAt: outcome.ValidatedAt,
		Identity:    toAPIIdentity(item),
	}
}

func toAPIPoint(p *valuation.Point) api.ValuationPointSummary {
	return api.ValuationPointSummary{
		Id:                      p.ID,
		JobId:                   p.JobID,
		EvaluatedAt:             p.EvaluatedAt,
		TotalUsd:                p.TotalUSD,
		Completeness:            api.Completeness(p.Completeness()),
		AccountCount:            p.AccountCount,
		StaleAccountCount:       p.StaleAccountCount,
		IncompleteAccountCount:  p.IncompleteAccountCount,
		BoundaryContributionUsd: p.BoundaryContributionUSD,
		BoundaryWithdrawalUsd:   p.BoundaryWithdrawalUSD,
		RecalculatedAt:          p.RecalculatedAt,
	}
}

func toAPIPointPtr(p *valuation.Point) *api.ValuationPointSummary {
	if p == nil {
		return nil
	}
	out := toAPIPoint(p)
	return &out
}

func toAPIPoints(points []valuation.Point) []api.ValuationPointSummary {
	out := make([]api.ValuationPointSummary, 0, len(points))
	for i := range points {
		out = append(out, toAPIPoint(&points[i]))
	}
	return out
}

func toAPIContext(c *valuation.Context) api.ReadContext {
	return api.ReadContext{PortfolioId: c.PortfolioID, Timezone: c.Timezone, PointId: c.PointID, JobId: c.JobID, EvaluatedAt: c.EvaluatedAt}
}

func toAPIIssues(issues []valuation.Issue) []api.Issue {
	out := make([]api.Issue, 0, len(issues))
	for i := range issues {
		item := &issues[i]
		out = append(out, api.Issue{Code: item.Code, Detail: item.Detail, AccountId: item.AccountID, AccountName: strPtr(item.AccountName), Symbol: strPtr(item.Symbol)})
	}
	return out
}

func toAPIAccountValuation(v *valuation.AccountValuation) api.AccountValuation {
	omitted := make([]api.OmittedBalance, 0, len(v.Omitted))
	for i := range v.Omitted {
		omitted = append(omitted, api.OmittedBalance{Symbol: v.Omitted[i].Symbol, AssetSymbol: strPtr(v.Omitted[i].AssetSymbol), Equity: v.Omitted[i].Equity, Reason: v.Omitted[i].Reason})
	}
	return api.AccountValuation{
		ReportId:               uuidPtr(v.ReportID),
		AccountId:              v.AccountID,
		AccountName:            v.AccountName,
		AccountType:            v.AccountType,
		Selected:               v.Selected,
		IdentityId:             v.IdentityID,
		IdentityName:           v.IdentityName,
		IdentityStatus:         strPtr(v.IdentityStatus),
		ConnectionStatus:       strPtr(v.ConnectionStatus),
		Platform:               v.Platform,
		Status:                 api.AccountValuationStatus(v.Status),
		ErrorClass:             v.ErrorClass,
		ErrorDetail:            v.ErrorDetail,
		Method:                 api.AccountValuationMethod(v.Method),
		Freshness:              api.Freshness(v.Freshness),
		Completeness:           api.Completeness(v.Completeness()),
		ValueUsd:               v.ValueUSD,
		ReportObservedAt:       v.ReportObservedAt,
		Equity:                 v.Equity,
		AdjustedEquity:         v.AdjustedEquity,
		AvailableEquity:        v.AvailableEquity,
		InitialMarginRatio:     v.InitialMarginRatio,
		MaintenanceMarginRatio: v.MaintenanceMarginRatio,
		UnifiedMarginRatio:     v.UnifiedMarginRatio,
		Omitted:                omitted,
	}
}

func toAPIAccountValuations(items []valuation.AccountValuation) []api.AccountValuation {
	out := make([]api.AccountValuation, 0, len(items))
	for i := range items {
		out = append(out, toAPIAccountValuation(&items[i]))
	}
	return out
}

func toAPIGroupTotals(groups []valuation.GroupTotal) []api.GroupTotal {
	out := make([]api.GroupTotal, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		out = append(out, api.GroupTotal{Key: g.Key, Label: g.Label, Platform: strPtr(g.Platform), ValueUsd: g.ValueUSD, AccountCount: g.AccountCount, RowCount: g.RowCount, Completeness: api.Completeness(g.Completeness)})
	}
	return out
}

func toAPIBalanceRows(rows []valuation.BalanceRow) []api.BalanceRow {
	out := make([]api.BalanceRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, api.BalanceRow{
			AccountId: r.AccountID, AccountName: r.AccountName, AccountType: r.AccountType,
			IdentityId: r.IdentityID, IdentityName: r.IdentityName, Platform: r.Platform,
			ListingId: r.ListingID, Symbol: r.Symbol, AssetSymbol: r.AssetSymbol,
			Equity: r.Equity, AvailableToWithdraw: r.AvailableToWithdraw, AvailableToTrade: r.AvailableToTrade,
			Liability: r.Liability, Interest: r.Interest, UnrealizedPnl: r.UnrealizedPnl,
			PriceUsd: r.PriceUSD, EquityUsd: r.EquityUSD, NoPrice: r.NoPrice,
			Freshness: api.Freshness(r.Freshness), ReportObservedAt: r.ReportObservedAt,
		})
	}
	return out
}

func toAPIPositionRows(rows []valuation.PositionRow) []api.PositionRow {
	out := make([]api.PositionRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, api.PositionRow{
			AccountId: r.AccountID, AccountName: r.AccountName, AccountType: r.AccountType,
			IdentityId: r.IdentityID, IdentityName: r.IdentityName, Platform: r.Platform,
			PlatformPositionRef: r.PlatformPositionRef, InstrumentId: r.InstrumentID, Symbol: r.Symbol, Kind: r.Kind,
			BaseSymbol: r.BaseSymbol, QuoteSymbol: r.QuoteSymbol, SettlementSymbol: r.SettlementSymbol,
			ExpiryAt: r.ExpiryAt, ContractSize: r.ContractSize,
			PositionMode: enumPtr[api.PositionRowPositionMode](r.PositionMode), MarginMode: enumPtr[api.PositionRowMarginMode](r.MarginMode),
			MarginCurrency: r.MarginCurrency, MarginBasis: enumPtr[api.PositionRowMarginBasis](r.MarginBasis),
			PortfolioMargin: r.PortfolioMargin, Size: r.Size, SizeSymbol: r.SizeSymbol, SettleMargin: r.SettleMargin, Issues: toAPIIssues(r.Issues),
			MarginSource: enumPtr[api.PositionRowMarginSource](r.MarginSource), MarginUnavailableReason: r.MarginUnavailableReason,
			Side: api.PositionRowSide(r.Side), Quantity: r.Quantity, QuantityUnit: api.PositionRowQuantityUnit(r.QuantityUnit),
			EntryPrice: r.EntryPrice, MarkPrice: r.MarkPrice, LiquidationPrice: r.LiquidationPrice,
			UnrealizedPnl: r.UnrealizedPnl, Leverage: r.Leverage, Margin: r.Margin, NotionalUsd: r.NotionalUSD,
			Freshness: api.Freshness(r.Freshness), ReportObservedAt: r.ReportObservedAt,
		})
	}
	return out
}

func toAPIBoundaryEvents(events []valuation.BoundaryEvent) []api.BoundaryEvent {
	out := make([]api.BoundaryEvent, 0, len(events))
	for i := range events {
		e := &events[i]
		out = append(out, api.BoundaryEvent{
			PointId: e.PointID, EvaluatedAt: e.EvaluatedAt, AccountId: e.AccountID, AccountName: e.AccountName,
			IdentityId: uuidPtr(e.IdentityID), IdentityName: strPtr(e.IdentityName), Platform: strPtr(e.Platform),
			Direction: api.BoundaryEventDirection(e.Direction), ValueUsd: e.ValueUSD, Method: e.Method,
		})
	}
	return out
}

func toAPIPeriodClose(c *valuation.PeriodClose) api.PeriodClose {
	return api.PeriodClose{
		PeriodKind: api.PeriodKind(c.PeriodKind), Label: c.Label, PeriodStart: c.PeriodStart, PeriodEnd: c.PeriodEnd,
		Point: toAPIPointPtr(c.Point), Completeness: api.Completeness(c.Completeness), Reasons: nonNilStrings(c.Reasons),
	}
}

func toAPIValueChange(v *valuation.ValueChange) api.ValueChange {
	return api.ValueChange{
		PeriodKind: v.PeriodKind, Label: v.Label, From: v.From, To: v.To,
		StartPoint: toAPIPointPtr(v.StartPoint), EndPoint: toAPIPointPtr(v.EndPoint),
		ContributionsUsd: v.ContributionsUSD, WithdrawalsUsd: v.WithdrawalsUSD,
		BoundaryContributionsUsd: v.BoundaryContributionsUSD, BoundaryWithdrawalsUsd: v.BoundaryWithdrawalsUSD,
		AmountUsd: v.AmountUSD, Completeness: api.Completeness(v.Completeness), Reasons: nonNilStrings(v.Reasons),
		BoundaryEvents: toAPIBoundaryEvents(v.BoundaryEvents),
	}
}

func toAPIFlow(f *capital.Flow) api.CapitalFlow {
	return api.CapitalFlow{
		Id: f.ID, PortfolioId: f.PortfolioID, Direction: api.CapitalFlowDirection(f.Direction), AssetSymbol: f.AssetSymbol,
		Quantity: f.Quantity, ValueUsd: f.ValueUSD, EffectiveAt: f.EffectiveAt, AccountId: f.AccountID, AccountName: f.AccountName,
		Note: f.Note, Version: int(f.Version), CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt, // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
	}
}

func toAPIListing(l *catalog.Listing) api.AssetListing {
	return api.AssetListing{
		Id: l.ID, Platform: l.Platform, Symbol: l.Symbol, AssetSymbol: l.AssetSymbol, Name: l.Name,
		MappingSource: api.AssetListingMappingSource(l.MappingSource), FirstSeenAt: l.FirstSeenAt, LastSeenAt: l.LastSeenAt, Version: int(l.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		Ignored: l.Ignored, IgnoreReason: l.IgnoreReason, IgnoredAt: l.IgnoredAt, ExclusionVersion: int(l.ExclusionVersion), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
	}
}

func toAPIInstrument(i *catalog.Instrument) api.Instrument {
	return api.Instrument{
		Id: i.ID, Platform: i.Platform, Symbol: i.Symbol, Kind: i.Kind, BaseSymbol: i.BaseSymbol, QuoteSymbol: i.QuoteSymbol,
		SettlementSymbol: i.SettlementSymbol, ExpiryAt: i.ExpiryAt, ContractSize: i.ContractSize, FirstSeenAt: i.FirstSeenAt, LastSeenAt: i.LastSeenAt,
	}
}

func toAPIJob(j *jobs.Job) api.Job {
	return api.Job{
		Id: j.ID, PortfolioId: j.PortfolioID, Kind: api.JobKind(j.Kind), Status: api.JobStatus(j.Status), Trigger: api.JobTrigger(j.Trigger),
		AttemptCount: int(j.AttemptCount), MaxAttempts: int(j.MaxAttempts), LastErrorClass: j.LastErrorClass, LastDetail: j.LastDetail,
		CreatedAt: j.CreatedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

func toAPICycle(j *jobs.Job, point *api.ValuationPointSummary) api.CollectionCycle {
	return api.CollectionCycle{
		Id: j.ID, PortfolioId: j.PortfolioID, Status: api.JobStatus(j.Status), Trigger: api.CollectionCycleTrigger(j.Trigger),
		AttemptCount: int(j.AttemptCount), MaxAttempts: int(j.MaxAttempts), LastErrorClass: j.LastErrorClass, LastDetail: j.LastDetail,
		CreatedAt: j.CreatedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt, Point: point,
	}
}

func toAPICyclePoint(jobID uuid.UUID, p *jobs.CyclePoint) *api.ValuationPointSummary {
	if p == nil {
		return nil
	}
	completeness := api.Complete
	if p.IncompleteAccountCount > 0 {
		completeness = api.Incomplete
	}
	return &api.ValuationPointSummary{
		Id: p.ID, JobId: jobID, EvaluatedAt: p.EvaluatedAt, TotalUsd: p.TotalUSD, Completeness: completeness,
		AccountCount: p.AccountCount, StaleAccountCount: p.StaleAccountCount, IncompleteAccountCount: p.IncompleteAccountCount,
		BoundaryContributionUsd: p.BoundaryContributionUSD, BoundaryWithdrawalUsd: p.BoundaryWithdrawalUSD, RecalculatedAt: p.RecalculatedAt,
	}
}

func toAPIAuditEvent(item *audit.Event) api.AuditEvent {
	out := api.AuditEvent{
		Action:       item.Action,
		After:        item.After,
		Before:       item.Before,
		Id:           item.ID,
		Metadata:     item.Metadata,
		OccurredAt:   item.OccurredAt,
		PortfolioId:  item.PortfolioID,
		ResourceType: item.ResourceType,
	}
	if item.ActorUserID != nil {
		out.ActorUserId = item.ActorUserID
	}
	resourceID := item.ResourceID
	out.ResourceId = &resourceID
	if item.RequestID != "" {
		out.RequestId = &item.RequestID
	}
	return out
}

func toAPIDualPositionRows(rows []valuation.DualPositionRow) []api.DualPositionRow {
	out := make([]api.DualPositionRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, api.DualPositionRow{AccountId: r.AccountID, AccountName: r.AccountName, AccountType: r.AccountType, IdentityId: r.IdentityID, IdentityName: r.IdentityName, Platform: r.Platform, PlatformPositionRef: r.PlatformPositionRef, ProductRef: r.ProductRef, OrderRef: r.OrderRef, BaseSymbol: r.BaseSymbol, QuoteSymbol: r.QuoteSymbol, InvestSymbol: r.InvestSymbol, Principal: r.Principal, Direction: r.Direction, TargetPrice: r.TargetPrice, ExpiryAt: r.ExpiryAt, AnnualizedRate: r.AnnualizedRate, RateKind: r.RateKind, Status: r.Status, Freshness: api.Freshness(r.Freshness), ReportObservedAt: r.ReportObservedAt, SubscribedAt: r.SubscribedAt, Issues: issueSlicePointer(r.Issues)})
	}
	return out
}

func enumPtr[T ~string](value *string) *T {
	if value == nil {
		return nil
	}
	out := T(*value)
	return &out
}
