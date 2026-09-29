package okx

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/adapters"
)

// dcdOrder is one order of the dual investment (DCD) order history.
type dcdOrder struct {
	OrdID           string `json:"ordId"`
	State           string `json:"state"`
	NotionalSz      string `json:"notionalSz"`
	NotionalCcy     string `json:"notionalCcy"`
	ProductID       string `json:"productId"`
	BaseCcy         string `json:"baseCcy"`
	QuoteCcy        string `json:"quoteCcy"`
	Strike          string `json:"strike"`
	AnnualizedYield string `json:"annualizedYield"`
	SettleTime      string `json:"settleTime"`
	ExpTime         string `json:"expTime"`
	CTime           string `json:"cTime"`
}

// dcdInvested says, for every order state OKX documents across its dual investment endpoints,
// whether the order still holds its principal. Only a rejected, settled, or redeemed order has
// paid it back.
var dcdInvested = map[string]bool{
	"initial":                true,
	"pending_book":           true,
	"live":                   true,
	"pending_settle":         true,
	"pending_redeem_booking": true,
	"pending_redeem":         true,
	"redeeming":              true,
	"rejected":               false,
	"settled":                false,
	"redeemed":               false,
}

// dcdPrincipalInvested accepts the documented states regardless of the casing OKX returns.
func dcdPrincipalInvested(state string) (bool, bool) {
	invested, known := dcdInvested[strings.ToLower(strings.TrimSpace(state))]
	return invested, known
}

const (
	// dcdPageSize is the largest page the order history serves.
	dcdPageSize = 100
	// dcdMaxPages bounds one run's walk through the order history to 1,000 orders.
	dcdMaxPages = 10
)

// dcdPage is one request of the order history: the newest orders first, then, for a cursor, the
// orders before it.
func dcdPage(before string) endpoint {
	ep := epDCD
	if before != "" {
		ep.query = "endId=" + before + "&" + ep.query
	}
	return ep
}

// dualInvestmentOrders reads the whole dual investment order history, newest first, one page per
// second as OKX allows. The history cannot be filtered to every state that still holds principal,
// so no page is skipped: the walk ends at a short page, and a history longer than dcdMaxPages
// pages fails the selected Earn and Dual Accounts instead of dropping an older order that is still invested.
func (a *Adapter) dualInvestmentOrders(ctx context.Context, c *client) ([]dcdOrder, error) {
	var orders []dcdOrder
	before := ""
	for page := 0; page < dcdMaxPages; page++ {
		if page > 0 {
			if err := a.wait(ctx); err != nil {
				return nil, err
			}
		}
		var rows []dcdOrder
		if err := c.call(ctx, dcdPage(before), &rows); err != nil {
			return nil, err
		}
		oldest, err := dcdCursor(rows, before)
		if err != nil {
			return nil, err
		}
		orders = append(orders, rows...)
		if len(rows) < dcdPageSize {
			return orders, nil
		}
		before = oldest
	}
	return nil, adapters.NewError(adapters.ClassUnsupported, fmt.Sprintf(
		"The OKX dual investment order history reaches the %d orders Plutus reads, so selected Earn and Dual Accounts are kept stale instead of missing an older order that is still invested.",
		dcdMaxPages*dcdPageSize))
}

// dcdCursor checks that a page lists its orders newest first and below the previous page's
// oldest order, and returns the id of its own oldest order. The history pages by order id, so a
// page out of order could end the walk before an order that still holds principal.
func dcdCursor(rows []dcdOrder, before string) (string, error) {
	oldest := before
	var previous *big.Int
	if before != "" {
		previous, _ = new(big.Int).SetString(before, 10)
	}
	for i := range rows {
		id := strings.TrimSpace(rows[i].OrdID)
		value, ok := new(big.Int).SetString(id, 10)
		if !ok || value.Sign() <= 0 {
			return "", adapters.Malformed(epDCD.String(), "ordId")
		}
		if previous != nil && value.Cmp(previous) >= 0 {
			return "", adapters.NewError(adapters.ClassNormalization,
				epDCD.String()+": the dual investment order history is not ordered newest first.")
		}
		previous, oldest = value, id
	}
	return oldest, nil
}

// dualPositions preserves every order that is still invested: notionalSz of
// notionalCcy, the amount and coin the User put in. The yield is counted only once OKX pays it out
// at settlement. A state OKX has not documented fails selected Earn and Dual Accounts instead of being guessed.
func dualPositions(orders []dcdOrder) ([]adapters.DualPosition, error) {
	where := epDCD.String()
	var out []adapters.DualPosition
	for i := range orders {
		row := &orders[i]
		invested, known := dcdPrincipalInvested(row.State)
		if !known {
			return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: dual investment order state %q is not documented.", where, truncate(strings.TrimSpace(row.State), 40)))
		}
		if !invested {
			continue
		}
		// Live order history supplies the scheduled settlement time; older documented
		// examples use expTime instead.
		expiryTime := row.SettleTime
		if strings.TrimSpace(expiryTime) == "" {
			expiryTime = row.ExpTime
		}
		expiry, err := adapters.DualExpiry(where, expiryTime)
		if err != nil {
			return nil, err
		}
		subscribed, err := adapters.DualTime(where, "dual subscription time", row.CTime)
		if err != nil {
			return nil, err
		}
		state := map[string]string{"initial": "pending", "pending_book": "pending", "live": "active", "pending_settle": "settling", "pending_redeem_booking": "redeeming", "pending_redeem": "redeeming", "redeeming": "redeeming"}[strings.ToLower(strings.TrimSpace(row.State))]
		p, err := adapters.NormalizeDualPosition(where, &adapters.DualPosition{PlatformPositionRef: row.OrdID, OrderRef: adapters.DualText(row.OrdID), ProductRef: adapters.DualText(row.ProductID), BaseSymbol: adapters.DualText(row.BaseCcy), QuoteSymbol: adapters.DualText(row.QuoteCcy), InvestSymbol: row.NotionalCcy, Principal: row.NotionalSz, TargetPrice: adapters.DualText(row.Strike), ExpiryAt: expiry, SubscribedAt: subscribed, AnnualizedRate: adapters.DualText(row.AnnualizedYield), RateKind: adapters.DualText("annualized_yield"), Status: state})
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
