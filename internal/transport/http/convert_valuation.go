package httpserver

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIPoint(p *valuation.Point) api.ValuationPointSummary {
	return api.ValuationPointSummary{
		CycleId:                 p.CycleID,
		CalculationVersion:      int64(p.CalculationVersion), // #nosec G115 -- Application algorithm versions are positive, code-owned constants far below MaxInt64.
		EvaluatedAt:             p.EvaluatedAt,
		TotalUsd:                p.TotalUSD,
		Completeness:            api.Completeness(p.Completeness()),
		AccountCount:            p.AccountCount,
		StaleAccountCount:       p.StaleAccountCount,
		IncompleteAccountCount:  p.IncompleteAccountCount,
		BoundaryContributionUsd: p.BoundaryContributionUSD,
		BoundaryWithdrawalUsd:   p.BoundaryWithdrawalUSD,
		DetailState:             api.ValuationPointSummaryDetailState(p.DetailState),
	}
}

func toAPIPointPtr(p *valuation.Point) *api.ValuationPointSummary {
	if p == nil {
		return nil
	}
	return new(toAPIPoint(p))
}

func toAPIPoints(points []valuation.Point) []api.ValuationPointSummary {
	out := make([]api.ValuationPointSummary, 0, len(points))
	for i := range points {
		out = append(out, toAPIPoint(&points[i]))
	}
	return out
}

func toAPIContext(c *financial.Context) api.ReadContext {
	return api.ReadContext{
		PortfolioId: c.PortfolioID,
		Timezone:    c.Timezone,
		CycleId:     c.CycleID,
		EvaluatedAt: c.EvaluatedAt,
	}
}

func toAPIIssues(issues []valuation.Issue) []api.Issue {
	out := make([]api.Issue, 0, len(issues))
	for i := range issues {
		item := &issues[i]
		var omitted *api.OmittedBalance
		if item.OmittedBalance != nil {
			omitted = new(toAPIOmittedBalance(item.OmittedBalance))
		}
		out = append(out, api.Issue{
			Code:             item.Code,
			FlowId:           item.FlowID,
			Detail:           item.Detail,
			AccountId:        item.AccountID,
			AccountName:      nonEmptyStringPtr(item.AccountName),
			Symbol:           nonEmptyStringPtr(item.Symbol),
			OmittedBalance:   omitted,
			ReportObservedAt: item.ReportObservedAt,
		})
	}
	return out
}

func toAPIOmittedBalance(item *valuation.Omitted) api.OmittedBalance {
	return api.OmittedBalance{AssetSymbol: item.AssetSymbol, CanonicalAssetSymbol: nonEmptyStringPtr(item.CanonicalAssetSymbol), Equity: item.Equity, Reason: item.Reason}
}

func toAPIAccountValuation(v *valuation.AccountValuation) api.AccountValuation {
	omitted := make([]api.OmittedBalance, 0, len(v.Omitted))
	for i := range v.Omitted {
		omitted = append(omitted, toAPIOmittedBalance(&v.Omitted[i]))
	}
	return api.AccountValuation{
		ReportId:               nonNilUUIDPtr(v.ReportID),
		SourceReportId:         v.SourceReportID,
		AccountId:              v.AccountID,
		AccountName:            v.AccountName,
		AccountType:            v.AccountType,
		Selected:               v.Selected,
		IdentityId:             v.IdentityID,
		IdentityName:           v.IdentityName,
		IdentityStatus:         nonEmptyStringPtr(v.IdentityStatus),
		ConnectionStatus:       nonEmptyStringPtr(v.ConnectionStatus),
		Platform:               v.Platform,
		Status:                 api.AccountValuationStatus(v.Status),
		ErrorClass:             v.ErrorClass,
		ErrorDetail:            v.ErrorDetail,
		Method:                 api.AccountValuationMethod(v.Method),
		Freshness:              api.Freshness(v.Freshness),
		Completeness:           api.Completeness(v.Completeness()),
		ValueUsd:               v.ValueUSD,
		ReportObservedAt:       v.ReportObservedAt,
		ManualModifiedAt:       v.ManualModifiedAt,
		ReportedEquityUsd:      v.Equity,
		AdjustedEquityUsd:      v.AdjustedEquity,
		AvailableEquityUsd:     v.AvailableEquity,
		InitialMarginRatio:     v.InitialMarginRatio,
		MaintenanceMarginRatio: v.MaintenanceMarginRatio,
		MarginRatio:            v.MarginRatio,
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

func toAPIBalanceRows(rows []financial.BalanceRow) []api.BalanceRow {
	out := make([]api.BalanceRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		var change *api.BalanceRowManualChange
		if r.ManualChange != "" {
			value := api.BalanceRowManualChange(r.ManualChange)
			change = &value
		}
		out = append(out, api.BalanceRow{
			ManualChange: change, ManualQuantity: r.ManualQuantity, Excluded: &r.Excluded,
			AccountId:            r.AccountID,
			AccountName:          r.AccountName,
			AccountType:          r.AccountType,
			IdentityId:           r.IdentityID,
			IdentityName:         r.IdentityName,
			Platform:             r.Platform,
			AssetId:              r.AssetID,
			AssetSymbol:          r.AssetSymbol,
			CanonicalAssetSymbol: r.CanonicalAssetSymbol,
			Equity:               collectedEquity(r),
			AvailableToWithdraw:  r.AvailableToWithdraw,
			AvailableToTrade:     r.AvailableToTrade,
			Liability:            r.Liability,
			Interest:             r.Interest,
			UnrealizedPnl:        r.UnrealizedPnl,
			PriceUsd:             r.PriceUSD,
			EquityUsd:            r.EquityUSD,
			NoPrice:              r.NoPrice,
			Freshness:            enumPtr[api.BalanceRowFreshness](nonEmptyStringPtr(r.Freshness)),
			ReportObservedAt:     r.ReportObservedAt,
			ManualModifiedAt:     r.ManualModifiedAt,
		})
	}
	return out
}

func toAPIPositionRows(rows []financial.PositionRow) []api.PositionRow {
	out := make([]api.PositionRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, api.PositionRow{
			AccountId:               r.AccountID,
			AccountName:             r.AccountName,
			AccountType:             r.AccountType,
			IdentityId:              r.IdentityID,
			IdentityName:            r.IdentityName,
			Platform:                r.Platform,
			PositionKey:             r.PositionKey,
			FuturesContractId:       r.FuturesContractID,
			FuturesSymbol:           r.FuturesSymbol,
			Kind:                    r.Kind,
			BaseAssetSymbol:         r.BaseAssetSymbol,
			QuoteAssetSymbol:        r.QuoteAssetSymbol,
			SettlementAssetSymbol:   r.SettlementAssetSymbol,
			ExpiryAt:                r.ExpiryAt,
			ContractSize:            r.ContractSize,
			PositionMode:            enumPtr[api.PositionRowPositionMode](r.PositionMode),
			MarginSetting:           enumPtr[api.PositionRowMarginSetting](r.MarginSetting),
			MarginAssetSymbol:       r.MarginAssetSymbol,
			MarginBasis:             enumPtr[api.PositionRowMarginBasis](r.MarginBasis),
			PortfolioMargin:         r.PortfolioMargin,
			Size:                    r.Size,
			SizeSymbol:              r.SizeSymbol,
			SettlementMargin:        r.SettleMargin,
			DisplayLeverage:         r.DisplayLeverage,
			MarginUnavailableCode:   enumPtr[api.PositionRowMarginUnavailableCode](r.MarginUnavailableCode),
			Issues:                  toAPIIssues(r.Issues),
			MarginSource:            enumPtr[api.PositionRowMarginSource](r.MarginSource),
			MarginUnavailableReason: r.MarginUnavailableReason,
			Side:                    api.PositionRowSide(r.Side),
			SourceQuantity:          r.SourceQuantity,
			SourceQuantityUnit:      api.PositionRowSourceQuantityUnit(r.SourceQuantityUnit),
			EntryPrice:              r.EntryPrice,
			MarkPrice:               r.MarkPrice,
			LiquidationPrice:        r.LiquidationPrice,
			UnrealizedPnl:           r.UnrealizedPnl,
			ReportedLeverage:        r.Leverage,
			ReportedMargin:          r.Margin,
			NotionalUsd:             r.NotionalUSD,
			Freshness:               api.Freshness(r.Freshness),
			ReportObservedAt:        r.ReportObservedAt,
		})
	}
	return out
}

func toAPIBoundaryEvents(events []valuation.BoundaryEvent) []api.BoundaryEvent {
	out := make([]api.BoundaryEvent, 0, len(events))
	for i := range events {
		e := &events[i]
		complete := api.Completeness("incomplete")
		if e.ValuationComplete {
			complete = api.Completeness("complete")
		}
		out = append(out, api.BoundaryEvent{
			CycleId:      e.CycleID,
			EvaluatedAt:  e.EvaluatedAt,
			AccountId:    e.AccountID,
			AccountName:  e.AccountName,
			IdentityId:   nonNilUUIDPtr(e.IdentityID),
			IdentityName: nonEmptyStringPtr(e.IdentityName),
			Platform:     nonEmptyStringPtr(e.Platform),
			Direction:    api.BoundaryEventDirection(e.Direction),
			Completeness: complete,
			ValueUsd:     e.ValueUSD,
			Method:       e.Method,
		})
	}
	return out
}

func toAPIPeriodClose(c *financial.PeriodClose) api.PeriodClose {
	return api.PeriodClose{
		PeriodKind:   api.PeriodKind(c.PeriodKind),
		Label:        c.Label,
		PeriodStart:  c.PeriodStart,
		PeriodEnd:    c.PeriodEnd,
		Point:        toAPIPointPtr(c.Point),
		Completeness: api.Completeness(c.Completeness),
		Issues:       toAPIIssues(c.Issues),
	}
}

func toAPIValueChange(v *financial.ValueChange) api.ValueChange {
	return api.ValueChange{
		PeriodKind:               v.PeriodKind,
		Label:                    v.Label,
		From:                     v.From,
		To:                       v.To,
		StartPoint:               toAPIPointPtr(v.StartPoint),
		EndPoint:                 toAPIPointPtr(v.EndPoint),
		ContributionsUsd:         v.ContributionsUSD,
		WithdrawalsUsd:           v.WithdrawalsUSD,
		BoundaryContributionsUsd: v.BoundaryContributionsUSD,
		BoundaryWithdrawalsUsd:   v.BoundaryWithdrawalsUSD,
		AmountUsd:                v.AmountUSD,
		Completeness:             api.Completeness(v.Completeness),
		Issues:                   toAPIIssues(v.Issues),
		Comparable:               v.Comparable,
		FlowsCompleteness:        api.Completeness(v.FlowsCompleteness),
		BoundaryEvents:           toAPIBoundaryEvents(v.BoundaryEvents),
	}
}

func toAPIDualPositionRows(rows []financial.DualPositionRow) []api.DualPositionRow {
	out := make([]api.DualPositionRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, api.DualPositionRow{
			AccountId:            r.AccountID,
			AccountName:          r.AccountName,
			AccountType:          r.AccountType,
			IdentityId:           r.IdentityID,
			IdentityName:         r.IdentityName,
			Platform:             r.Platform,
			PositionKey:          r.PositionKey,
			ProductRef:           r.ProductRef,
			OrderRef:             r.OrderRef,
			BaseAssetSymbol:      r.BaseAssetSymbol,
			QuoteAssetSymbol:     r.QuoteAssetSymbol,
			PrincipalAssetSymbol: r.PrincipalAssetSymbol,
			Principal:            r.Principal,
			Direction:            r.Direction,
			TargetPrice:          r.TargetPrice,
			ExpiryAt:             r.ExpiryAt,
			AnnualizedRate:       r.AnnualizedRate,
			RateKind:             r.RateKind,
			Status:               r.Status,
			Freshness:            api.Freshness(r.Freshness),
			ReportObservedAt:     r.ReportObservedAt,
			SubscribedAt:         r.SubscribedAt,
			Issues:               new(toAPIIssues(r.Issues)),
		})
	}
	return out
}

func collectedEquity(row *financial.BalanceRow) *string {
	if row.ManualChange == "added" {
		return nil
	}
	return &row.Equity
}

func toAPIPortfolioContext(c *financial.Context) api.PortfolioReadContext {
	return api.PortfolioReadContext{PortfolioId: c.PortfolioID, Timezone: c.Timezone}
}

func toAPISeries(points []valuation.Point) []api.ValuationSeriesPoint {
	out := make([]api.ValuationSeriesPoint, 0, len(points))
	for _, point := range points {
		out = append(out, api.ValuationSeriesPoint{CycleId: point.CycleID, EvaluatedAt: point.EvaluatedAt, TotalUsd: point.TotalUSD, Completeness: api.Completeness(point.Completeness())})
	}
	return out
}

func toAPIReportCoverage(c financial.ReportCoverage) api.ReportCoverage {
	ids := c.MissingAccountIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return api.ReportCoverage{Complete: c.Complete, MissingAccountIds: ids}
}

func toAPIManualOverlay(overlay *financial.ManualOverlay) *api.ManualOverlay {
	if overlay == nil {
		return nil
	}
	versions := make([]api.ManualLedgerVersion, 0, len(overlay.Versions))
	for _, v := range overlay.Versions {
		versions = append(versions, api.ManualLedgerVersion{AccountId: v.AccountID, Version: v.Version})
	}
	return &api.ManualOverlay{Versions: versions}
}
