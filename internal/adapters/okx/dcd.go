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
	OrdID       string `json:"ordId"`
	State       string `json:"state"`
	NotionalSz  string `json:"notionalSz"`
	NotionalCcy string `json:"notionalCcy"`
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
// pages fails the Earn Account instead of dropping an older order that is still invested.
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
		"The OKX dual investment order history reaches the %d orders Plutus reads, so Earn is kept stale instead of missing an older order that is still invested.",
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

// addDualInvestment adds the principal of every order that is still invested: notionalSz of
// notionalCcy, the amount and coin the User put in. The yield is counted only once OKX pays it out
// at settlement. A state OKX has not documented fails the Earn Account instead of being guessed.
func addDualInvestment(totals adapters.Totals, orders []dcdOrder) error {
	where := epDCD.String()
	for i := range orders {
		order := &orders[i]
		state := strings.TrimSpace(order.State)
		invested, known := dcdInvested[state]
		if !known {
			return adapters.NewError(adapters.ClassNormalization,
				fmt.Sprintf("%s: dual investment order state %q is not documented.", where, truncate(state, 40)))
		}
		if !invested {
			continue
		}
		amount, err := adapters.Decimal(where, "notionalSz of "+order.NotionalCcy, order.NotionalSz)
		if err != nil {
			return err
		}
		if err := totals.Add(where, order.NotionalCcy, amount); err != nil {
			return err
		}
	}
	return nil
}
