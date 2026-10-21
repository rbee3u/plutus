package bybit

import (
	"context"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type savingsPosition struct {
	Coin           string `json:"coin"`
	Amount         string `json:"amount"`
	ClaimableYield string `json:"claimableYield"`
	Category       string `json:"category"`
	ProductID      string `json:"productId"`
	PositionID     string `json:"positionId"`
	OrderID        string `json:"orderId"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	SettlementTime string `json:"settlementTime"`
	Duration       string `json:"duration"`
	TotalPNL       string `json:"totalPnl"`
	YesterdayYield string `json:"yesterdayYield"`
	Interest       []struct {
		Coin     string `json:"coin"`
		APY      string `json:"apy"`
		Expected string `json:"expectReturnEarning"`
	} `json:"interestCoinApyList"`
}

func (r *savingsPosition) position(category, amount string) adapter.EarnPosition {
	ref := r.ProductID
	if ref == "" {
		ref = r.Coin
	}
	p := adapter.NewEarnPosition("flexible:"+ref+":"+strings.ToUpper(strings.TrimSpace(r.Coin)), "flexible", "product", "principal")
	p.ProductName = adapter.OptionalText("Flexible Savings")
	p.ProductRef = adapter.OptionalText(r.ProductID)
	if category == "FixedTermSaving" {
		ref = ""
		if r.PositionID != "" {
			ref = "fixed:" + r.PositionID
		}
		p = adapter.NewEarnPosition(ref, "fixed", "position", "principal")
		p.ProductName = adapter.OptionalText("Fixed-Term Savings")
		p.ProductRef = adapter.OptionalText(r.ProductID)
		p.Status = adapter.OptionalText(r.Status)
		p.SubscribedAt = p.RequiredMillis("Subscription time", r.CreatedAt)
		p.MaturityAt = p.RequiredMillis("Maturity time", r.SettlementTime)
		p.AddDetail("Order", r.OrderID)
		p.AddDetail("Duration", r.Duration)
		for _, rate := range r.Interest {
			addPercentRate(&p, rate.APY, "apy", "reported", "APY", adapter.OptionalText(rate.Coin))
			p.AddReward(rate.Coin, rate.Expected, "estimated")
		}
	} else {
		// FlexibleSaving does not report a lifecycle status; OnChain's status
		// and the catalogue's Available flag describe different products/facts.
		p.AddReward(r.Coin, r.ClaimableYield, "pending")
		p.AddReward(r.Coin, r.TotalPNL, "reported")
		p.AddReward(r.Coin, r.YesterdayYield, "yesterday")
	}
	p.Assets = append(p.Assets, adapter.EarnAmount{AssetSymbol: r.Coin, Amount: amount})
	return p
}

func addPercentRate(p *adapter.EarnPosition, value, kind, qualifier, label string, symbol *string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if !strings.HasSuffix(value, "%") {
		p.Problems = append(p.Problems, label+" has an unknown unit.")
		return
	}
	n := p.OptionalDecimal(label, strings.TrimSpace(strings.TrimSuffix(value, "%")))
	if n == nil {
		return
	}
	p.AddRate(decimal.Format(new(big.Rat).Quo(decimal.ParseOrZero(*n), decimal.ParseOrZero("100"))), kind, qualifier, label, symbol)
}

// Optional public catalogue enrichment is read once and does not affect funds.
func enrichFlexible(ctx context.Context, c *client, positions []adapter.EarnPosition) {
	needed := false
	for i := range positions {
		if positions[i].Type == "flexible" {
			needed = true
		}
	}
	if !needed {
		return
	}
	var result struct {
		List []struct {
			ProductID string `json:"productId"`
			Coin      string `json:"coin"`
			Category  string `json:"category"`
			APR       string `json:"estimateApr"`
		} `json:"list"`
	}
	err := c.call(ctx, epEarnProduct, nil, &result)
	for i := range positions {
		p := &positions[i]
		if p.Type != "flexible" {
			continue
		}
		if err != nil || result.List == nil {
			p.Problems = append(p.Problems, "Estimated APR could not be read.")
			continue
		}
		matches := 0
		value := ""
		for _, r := range result.List {
			if p.ProductRef != nil && r.ProductID == *p.ProductRef && r.Coin == p.Assets[0].AssetSymbol && r.Category == "FlexibleSaving" {
				matches++
				value = r.APR
			}
		}
		if matches != 1 || strings.TrimSpace(value) == "" {
			p.Problems = append(p.Problems, "Estimated APR could not be matched to this product.")
			continue
		}
		addPercentRate(p, value, "apr", "estimated", "Estimated APR", nil)
	}
}
