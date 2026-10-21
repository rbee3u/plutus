package okx

import "github.com/rbee3u/plutus/internal/platform/adapter"

type savingsBalance struct {
	Ccy      string `json:"ccy"`
	Amt      string `json:"amt"`
	Rate     string `json:"rate"`
	Earnings string `json:"earnings"`
	Loan     string `json:"loanAmt"`
	Pending  string `json:"pendingAmt"`
}

func (r *savingsBalance) position(amount string) adapter.EarnPosition {
	p := adapter.NewEarnPosition("savings:"+r.Ccy, "flexible", "product", "total")
	p.ProductName = adapter.OptionalText("Simple Earn Flexible")
	p.Assets = append(p.Assets, adapter.EarnAmount{AssetSymbol: r.Ccy, Amount: amount})
	p.AddRate(r.Rate, "apr", "minimum", "Minimum lending APR configured by the user", nil)
	p.AddReward(r.Ccy, r.Earnings, "reported")
	for _, d := range []struct{ label, value string }{{"Lending amount", r.Loan}, {"Pending amount", r.Pending}} {
		if n := p.OptionalDecimal(d.label, d.value); n != nil {
			p.AddDetail(d.label, *n+" "+r.Ccy)
		}
	}
	return p
}

type stakingOrder struct {
	ID         string `json:"ordId"`
	ProductID  string `json:"productId"`
	Protocol   string `json:"protocol"`
	State      string `json:"state"`
	APY        string `json:"apy"`
	Term       string `json:"term"`
	Purchased  string `json:"purchasedTime"`
	Settlement string `json:"estSettlementTime"`
	InvestData []struct {
		Ccy string `json:"ccy"`
		Amt string `json:"amt"`
	} `json:"investData"`
	EarningData []struct {
		Ccy      string `json:"ccy"`
		Earnings string `json:"earnings"`
		Type     string `json:"earningType"`
	} `json:"earningData"`
}

func (r *stakingOrder) position() adapter.EarnPosition {
	ref := ""
	if r.ID != "" {
		ref = "onchain:" + r.ID
	}
	p := adapter.NewEarnPosition(ref, "on_chain", "position", "principal")
	p.ProductRef = adapter.OptionalText(r.ProductID)
	p.ProductName = adapter.OptionalText(r.Protocol)
	p.SubscribedAt = p.Millis("Subscription time", r.Purchased)
	p.AddMillisDetail("Estimated redemption settlement", r.Settlement)
	p.AddRate(r.APY, "apy", "estimated", "Estimated APY", nil)
	if r.Term == "0" {
		p.AddDetail("Term", "Flexible")
	} else if term := p.OptionalDecimal("Term", r.Term); term != nil {
		p.AddDetail("Term (days)", *term)
	}
	states := map[string]string{"8": "Pending", "13": "Cancelling", "9": "Onchain", "1": "Earning", "2": "Redeeming"}
	if status, ok := states[r.State]; ok {
		p.Status = adapter.OptionalText(status)
	} else if r.State != "" {
		p.Problems = append(p.Problems, "Product state is unsupported.")
	}
	for _, reward := range r.EarningData {
		kind, ok := map[string]string{"0": "estimated", "1": "cumulative"}[reward.Type]
		if !ok {
			kind = "reported"
			if reward.Type != "" {
				p.Problems = append(p.Problems, "Reward type is unsupported.")
			}
		}
		p.AddReward(reward.Ccy, reward.Earnings, kind)
	}
	return p
}
