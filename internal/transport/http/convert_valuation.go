package httpserver

import (
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

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
		PointId:     c.PointID,
		JobId:       c.JobID,
		EvaluatedAt: c.EvaluatedAt,
	}
}

func toAPIIssues(issues []valuation.Issue) []api.Issue {
	out := make([]api.Issue, 0, len(issues))
	for i := range issues {
		item := &issues[i]
		out = append(out, api.Issue{
			Code:        item.Code,
			Detail:      item.Detail,
			AccountId:   item.AccountID,
			AccountName: nonEmptyStringPtr(item.AccountName),
			Symbol:      nonEmptyStringPtr(item.Symbol),
		})
	}
	return out
}

func toAPIAccountValuation(v *valuation.AccountValuation) api.AccountValuation {
	omitted := make([]api.OmittedBalance, 0, len(v.Omitted))
	for i := range v.Omitted {
		omitted = append(omitted, api.OmittedBalance{Symbol: v.Omitted[i].Symbol, AssetSymbol: nonEmptyStringPtr(v.Omitted[i].AssetSymbol), Equity: v.Omitted[i].Equity, Reason: v.Omitted[i].Reason})
	}
	return api.AccountValuation{
		ReportId:               nonNilUUIDPtr(v.ReportID),
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

func toAPIGroupTotals(groups []financial.GroupTotal) []api.GroupTotal {
	out := make([]api.GroupTotal, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		out = append(out, api.GroupTotal{
			Key:          g.Key,
			Label:        g.Label,
			Platform:     nonEmptyStringPtr(g.Platform),
			ValueUsd:     g.ValueUSD,
			AccountCount: g.AccountCount,
			RowCount:     g.RowCount,
			Completeness: api.Completeness(g.Completeness),
		})
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
			AccountId:           r.AccountID,
			AccountName:         r.AccountName,
			AccountType:         r.AccountType,
			IdentityId:          r.IdentityID,
			IdentityName:        r.IdentityName,
			Platform:            r.Platform,
			ListingId:           r.ListingID,
			Symbol:              r.Symbol,
			AssetSymbol:         r.AssetSymbol,
			Equity:              r.Equity,
			AvailableToWithdraw: r.AvailableToWithdraw,
			AvailableToTrade:    r.AvailableToTrade,
			Liability:           r.Liability,
			Interest:            r.Interest,
			UnrealizedPnl:       r.UnrealizedPnl,
			PriceUsd:            r.PriceUSD,
			EquityUsd:           r.EquityUSD,
			NoPrice:             r.NoPrice,
			Freshness:           api.Freshness(r.Freshness),
			ReportObservedAt:    r.ReportObservedAt,
			ManualModifiedAt:    r.ManualModifiedAt,
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
			PlatformPositionRef:     r.PlatformPositionRef,
			InstrumentId:            r.InstrumentID,
			Symbol:                  r.Symbol,
			Kind:                    r.Kind,
			BaseSymbol:              r.BaseSymbol,
			QuoteSymbol:             r.QuoteSymbol,
			SettlementSymbol:        r.SettlementSymbol,
			ExpiryAt:                r.ExpiryAt,
			ContractSize:            r.ContractSize,
			PositionMode:            enumPtr[api.PositionRowPositionMode](r.PositionMode),
			MarginMode:              enumPtr[api.PositionRowMarginMode](r.MarginMode),
			MarginCurrency:          r.MarginCurrency,
			MarginBasis:             enumPtr[api.PositionRowMarginBasis](r.MarginBasis),
			PortfolioMargin:         r.PortfolioMargin,
			Size:                    r.Size,
			SizeSymbol:              r.SizeSymbol,
			SettleMargin:            r.SettleMargin,
			Issues:                  toAPIIssues(r.Issues),
			MarginSource:            enumPtr[api.PositionRowMarginSource](r.MarginSource),
			MarginUnavailableReason: r.MarginUnavailableReason,
			Side:                    api.PositionRowSide(r.Side),
			Quantity:                r.Quantity,
			QuantityUnit:            api.PositionRowQuantityUnit(r.QuantityUnit),
			EntryPrice:              r.EntryPrice,
			MarkPrice:               r.MarkPrice,
			LiquidationPrice:        r.LiquidationPrice,
			UnrealizedPnl:           r.UnrealizedPnl,
			Leverage:                r.Leverage,
			Margin:                  r.Margin,
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
		out = append(out, api.BoundaryEvent{
			PointId:      e.PointID,
			EvaluatedAt:  e.EvaluatedAt,
			AccountId:    e.AccountID,
			AccountName:  e.AccountName,
			IdentityId:   nonNilUUIDPtr(e.IdentityID),
			IdentityName: nonEmptyStringPtr(e.IdentityName),
			Platform:     nonEmptyStringPtr(e.Platform),
			Direction:    api.BoundaryEventDirection(e.Direction),
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
		Reasons:      nonNilStrings(c.Reasons),
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
		Reasons:                  nonNilStrings(v.Reasons),
		BoundaryEvents:           toAPIBoundaryEvents(v.BoundaryEvents),
	}
}

func toAPIDualPositionRows(rows []financial.DualPositionRow) []api.DualPositionRow {
	out := make([]api.DualPositionRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, api.DualPositionRow{
			AccountId:           r.AccountID,
			AccountName:         r.AccountName,
			AccountType:         r.AccountType,
			IdentityId:          r.IdentityID,
			IdentityName:        r.IdentityName,
			Platform:            r.Platform,
			PlatformPositionRef: r.PlatformPositionRef,
			ProductRef:          r.ProductRef,
			OrderRef:            r.OrderRef,
			BaseSymbol:          r.BaseSymbol,
			QuoteSymbol:         r.QuoteSymbol,
			InvestSymbol:        r.InvestSymbol,
			Principal:           r.Principal,
			Direction:           r.Direction,
			TargetPrice:         r.TargetPrice,
			ExpiryAt:            r.ExpiryAt,
			AnnualizedRate:      r.AnnualizedRate,
			RateKind:            r.RateKind,
			Status:              r.Status,
			Freshness:           api.Freshness(r.Freshness),
			ReportObservedAt:    r.ReportObservedAt,
			SubscribedAt:        r.SubscribedAt,
			Issues:              new(toAPIIssues(r.Issues)),
		})
	}
	return out
}
