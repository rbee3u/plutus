package financial

import (
	"context"
	"database/sql"
	"errors"
	"math/big"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Account-level risk exposure is the signed coin quantity a unified account
// holds after netting spot equity against its open linear and inverse
// contracts. It is a display derivation over retained report facts; it never
// changes valuation, totals, or position source amounts.
const (
	// RiskExposureNotUnified marks an account whose scope is not unified.
	RiskExposureNotUnified = "not_unified"
	// RiskExposurePlatformPending marks a supported scope awaiting rollout.
	RiskExposurePlatformPending = "platform_pending"
	// RiskExposureUnknownPosition marks unconvertible position inputs.
	RiskExposureUnknownPosition = "unknown_position_inputs"
)

// riskExposurePlatforms is the explicit rollout gate. Every currently supported
// unified scope is enabled; a future scope must be added deliberately and
// reports platform_pending until then.
var riskExposurePlatforms = map[adapter.Platform]bool{
	adapter.PlatformBinance: true,
	adapter.PlatformOKX:     true,
	adapter.PlatformBybit:   true,
	adapter.PlatformKraken:  true,
}

// riskPosition retains the facts needed to price one contract when no spot
// quote exists. Size is already normalized to base quantity for linear
// contracts and quote face for inverse contracts.
type riskPosition struct {
	futuresSymbol string
	quote         string
	settlement    string
	inverse       bool
	mark          *big.Rat
	notional      *big.Rat
	pnl           *big.Rat
	pnlKnown      bool
}

// riskAsset aggregates every open contract whose base is one platform asset.
// A single unconvertible position makes the whole exposure unknown; partial
// sums would understate or overstate the net.
type riskAsset struct {
	contract      *big.Rat
	unknown       bool
	positions     []riskPosition
	pnl           *big.Rat
	pnlUnknown    bool
	hasSettledPnl bool
}

func (s *Service) applyRiskExposure(
	ctx context.Context,
	p PortfolioContext,
	account *valuation.AccountValuation,
	rows []BalanceRow,
	positions []PositionRow,
	balanceAssets map[string]bool,
) ([]BalanceRow, error) {
	if !adapter.SupportedUnifiedAccount(
		adapter.Platform(account.Platform),
		adapter.AccountStructure(account.AccountStructure),
		adapter.AccountType(account.AccountType),
	) {
		markRiskUnavailable(rows, RiskExposureNotUnified)
		return rows, nil
	}
	if !riskExposurePlatforms[adapter.Platform(account.Platform)] {
		markRiskUnavailable(rows, RiskExposurePlatformPending)
		return rows, nil
	}

	assets := riskAssets(positions)
	applyRiskExposureToRows(rows, assets)
	for symbol, asset := range assets {
		if balanceAssets[symbol] {
			continue
		}
		row, ok, err := s.syntheticRiskRow(ctx, p, account, symbol, asset)
		if err != nil {
			return nil, err
		}
		if ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func markRiskUnavailable(rows []BalanceRow, reason string) {
	for i := range rows {
		rows[i].RiskExposure = nil
		rows[i].RiskExposureUnavailableReason = reason
		rows[i].ContractOnly = false
	}
}

// riskAssets groups positions by normalized base asset and computes the signed
// contract quantity. The side is authoritative; Size may be negative for a
// one-way short, so it is reduced to its magnitude first.
func riskAssets(positions []PositionRow) map[string]*riskAsset {
	out := make(map[string]*riskAsset, len(positions))
	for i := range positions {
		position := &positions[i]
		base, ok := riskSymbol(position.BaseAssetSymbol)
		if !ok {
			continue
		}
		asset := out[base]
		if asset == nil {
			asset = &riskAsset{contract: decimal.Zero()}
			out[base] = asset
		}
		item := riskPosition{
			futuresSymbol: position.FuturesSymbol,
			quote:         normalizedRiskSymbol(position.QuoteAssetSymbol),
			settlement:    normalizedRiskSymbol(position.SettlementAssetSymbol),
			inverse:       inverseKind(position.Kind),
		}
		if position.MarkPrice != nil {
			if mark, valid := decimal.Parse(*position.MarkPrice); valid && mark.Sign() > 0 {
				item.mark = mark
			}
		}
		if position.NotionalUSD != nil {
			if notional, valid := decimal.Parse(*position.NotionalUSD); valid && notional.Sign() >= 0 {
				item.notional = notional
			}
		}
		if position.UnrealizedPnl != nil {
			if pnl, valid := decimal.Parse(*position.UnrealizedPnl); valid {
				item.pnl, item.pnlKnown = pnl, true
			}
		}
		if position.Size == nil {
			asset.unknown = true
		} else if size, valid := decimal.Parse(*position.Size); !valid {
			asset.unknown = true
		} else {
			size = decimal.Abs(size)
			var quantity *big.Rat
			if item.inverse {
				if item.mark == nil {
					asset.unknown = true
				} else {
					quantity = new(big.Rat).Quo(size, item.mark)
				}
			} else {
				quantity = new(big.Rat).Set(size)
			}
			if quantity != nil {
				switch position.Side {
				case adapter.SideLong:
				case adapter.SideShort:
					quantity.Neg(quantity)
				default:
					quantity = nil
					asset.unknown = true
				}
				if quantity != nil {
					asset.contract = decimal.Add(asset.contract, quantity)
				}
			}
		}
		if item.settlement == base {
			asset.hasSettledPnl = true
			if !item.pnlKnown {
				asset.pnlUnknown = true
			} else {
				asset.pnl = decimal.Add(asset.pnl, item.pnl)
			}
		}
		asset.positions = append(asset.positions, item)
	}
	return out
}

// applyRiskExposureToRows adds the spot leg to each real balance row. Rows with
// no contract keep their equity as the exposure.
func applyRiskExposureToRows(rows []BalanceRow, assets map[string]*riskAsset) {
	for i := range rows {
		row := &rows[i]
		symbol, ok := riskSymbol(&row.AssetSymbol)
		if !ok {
			row.RiskExposure = nil
			row.RiskExposureUnavailableReason = RiskExposureUnknownPosition
			continue
		}
		asset := assets[symbol]
		if asset == nil {
			exposure, valid := decimal.Parse(row.Equity)
			if !valid {
				row.RiskExposure = nil
				row.RiskExposureUnavailableReason = RiskExposureUnknownPosition
				continue
			}
			row.RiskExposure = decimal.FormatPtr(exposure)
			row.RiskExposureUnavailableReason = ""
			row.ContractOnly = false
			applyRiskPrice(row, nil)
			continue
		}
		row.ContractOnly = riskZero(row.Equity)
		if asset.unknown {
			row.RiskExposure = nil
			row.RiskExposureUnavailableReason = RiskExposureUnknownPosition
		} else if spot, valid := decimal.Parse(row.Equity); !valid {
			row.RiskExposure = nil
			row.RiskExposureUnavailableReason = RiskExposureUnknownPosition
		} else {
			row.RiskExposure = decimal.FormatPtr(decimal.Add(spot, asset.contract))
			row.RiskExposureUnavailableReason = ""
		}
		applyRiskPrice(row, asset)
	}
}

// applyRiskPrice falls back to the largest known contract mark when the balance
// has no adopted spot price. The fallback is display-only: EquityUSD and the
// no-price valuation flag remain unchanged.
func applyRiskPrice(row *BalanceRow, asset *riskAsset) {
	if row.PriceUSD != nil {
		row.PriceSource = "spot"
		row.PriceSourceSymbol = nil
		return
	}
	if asset == nil {
		row.PriceSource = ""
		row.PriceSourceSymbol = nil
		return
	}
	mark, symbol := riskPrice(asset)
	if mark == nil {
		row.PriceSource = ""
		row.PriceSourceSymbol = nil
		return
	}
	row.PriceUSD = decimal.FormatPtr(mark)
	row.PriceSource = "contract"
	row.PriceSourceSymbol = &symbol
}

// riskPrice selects the largest absolute USD notional contract with a usable
// mark. Position order breaks ties deterministically.
func riskPrice(asset *riskAsset) (*big.Rat, string) {
	var best *riskPosition
	for i := range asset.positions {
		position := &asset.positions[i]
		if position.mark == nil || position.notional == nil || !valuation.USDQuote(position.quote) {
			continue
		}
		if best == nil || decimal.Abs(position.notional).Cmp(decimal.Abs(best.notional)) > 0 {
			best = position
		}
	}
	if best == nil {
		return nil, ""
	}
	return best.mark, best.futuresSymbol
}

// riskPnl is the coin-settled position result for a contract-only row. A
// position settled in another coin contributes nothing; its result belongs to
// that settlement coin's balance row. Unknown contributing input stays unknown.
func riskPnl(asset *riskAsset) *string {
	if asset.pnlUnknown {
		return nil
	}
	if !asset.hasSettledPnl {
		return new("0")
	}
	return decimal.FormatPtr(asset.pnl)
}

// riskPositionsByCycle loads every account's positions at one point for the
// flat balance list. The same retained source report feeds exposure and the
// Futures view.
func (s *Service) riskPositionsByCycle(ctx context.Context, p PortfolioContext, cycleID uuid.UUID, accounts map[uuid.UUID]*valuation.AccountValuation) (map[uuid.UUID][]PositionRow, error) {
	rows, err := mysql.Queries(ctx, s.db).ListPositionsByCycle(ctx, sqlcgen.ListPositionsByCycleParams{PortfolioID: p.ID[:], CycleID: cycleID[:]})
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]PositionRow{}
	for i := range rows {
		r := &rows[i]
		accountID, err := uuid.FromBytes(r.AccountID)
		if err != nil {
			return nil, err
		}
		account := accounts[accountID]
		if account == nil {
			continue
		}
		row, err := positionRow(account, &sqlcgen.ListPositionsByReportRow{
			FuturesContractID:     r.FuturesContractID,
			PositionKey:           r.PositionKey,
			Side:                  r.Side,
			SourceQuantity:        r.SourceQuantity,
			SourceQuantityUnit:    r.SourceQuantityUnit,
			EntryPrice:            r.EntryPrice,
			MarkPrice:             r.MarkPrice,
			LiquidationPrice:      r.LiquidationPrice,
			UnrealizedPnl:         r.UnrealizedPnl,
			Leverage:              r.Leverage,
			Margin:                r.Margin,
			PositionMode:          r.PositionMode,
			MarginSetting:         r.MarginSetting,
			MarginAssetSymbol:     r.MarginAssetSymbol,
			MarginBasis:           r.MarginBasis,
			Platform:              r.Platform,
			FuturesSymbol:         r.FuturesSymbol,
			Kind:                  r.Kind,
			BaseAssetSymbol:       r.BaseAssetSymbol,
			QuoteAssetSymbol:      r.QuoteAssetSymbol,
			SettlementAssetSymbol: r.SettlementAssetSymbol,
			ExpiryAt:              r.ExpiryAt,
			ContractSize:          r.ContractSize,
		})
		if err != nil {
			return nil, err
		}
		completePositionFacts(&row, account, positionMetadata{sql.NullString{String: r.PositionMode, Valid: true}, r.MarginSetting, r.MarginAssetSymbol, r.MarginBasis})
		out[accountID] = append(out[accountID], row)
	}
	for accountID := range out {
		sortPositions(out[accountID], nil)
	}
	return out, nil
}

func (s *Service) syntheticRiskRow(
	ctx context.Context,
	p PortfolioContext,
	account *valuation.AccountValuation,
	symbol string,
	asset *riskAsset,
) (BalanceRow, bool, error) {
	stored, err := mysql.Queries(ctx, s.db).GetAssetByPlatformSymbol(ctx, sqlcgen.GetAssetByPlatformSymbolParams{
		PortfolioID: p.ID[:],
		Platform:    account.Platform,
		Symbol:      symbol,
	})
	if errors.Is(err, sql.ErrNoRows) {
		// The collection registers position bases; a missing record is a legacy
		// report gap. Its Futures row remains the visible source fact.
		return BalanceRow{}, false, nil
	}
	if err != nil {
		return BalanceRow{}, false, err
	}
	id, err := uuid.FromBytes(stored.ID)
	if err != nil {
		return BalanceRow{}, false, err
	}
	row := BalanceRow{
		AccountID:            account.AccountID,
		AccountName:          account.AccountName,
		AccountType:          account.AccountType,
		IdentityID:           account.IdentityID,
		IdentityName:         account.IdentityName,
		Platform:             account.Platform,
		AssetID:              id,
		AssetSymbol:          stored.Symbol,
		CanonicalAssetSymbol: stored.CanonicalAssetSymbol,
		Equity:               "0",
		Liability:            new("0"),
		UnrealizedPnl:        riskPnl(asset),
		EquityUSD:            new("0"),
		ContractOnly:         true,
		Freshness:            account.Freshness,
		ReportObservedAt:     account.ReportObservedAt,
		equityUSDRat:         decimal.Zero(),
	}
	if asset.unknown {
		row.RiskExposureUnavailableReason = RiskExposureUnknownPosition
	} else {
		row.RiskExposure = decimal.FormatPtr(asset.contract)
	}
	if mark, futuresSymbol := riskPrice(asset); mark != nil {
		row.PriceUSD = decimal.FormatPtr(mark)
		row.PriceSource = "contract"
		row.PriceSourceSymbol = &futuresSymbol
	} else {
		row.NoPrice = true
	}
	return row, true, nil
}

func riskSymbol(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	symbol, err := catalog.NormalizeSymbol(*value)
	if err != nil {
		return "", false
	}
	return symbol, true
}

func normalizedRiskSymbol(value *string) string {
	symbol, _ := riskSymbol(value)
	return symbol
}

func riskZero(value string) bool {
	number, valid := decimal.Parse(value)
	return valid && number.Sign() == 0
}
