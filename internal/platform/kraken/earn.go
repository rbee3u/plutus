package kraken

import (
	"context"
	"fmt"
	"net/url"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// earnInMain lists the lock types Kraken documents. flex strategies are Kraken Rewards, which earn
// on main balances and stay in main; the other lock types hold funds allocated to earn.
var earnInMain = map[string]bool{"flex": true, "instant": false, "bonded": false, "timed": false}

// maxStrategyPages bounds the strategy pages one collection follows.
const maxStrategyPages = 1000

// earn maps allocated earn: every allocation of an instant, bonded, or timed strategy, one balance
// per coin. A nonzero allocation whose strategy is unknown or has an undocumented lock type fails
// earn: dropping it would erase real holdings under the complete set rule.
func (r *run) earn(ctx context.Context) (adapter.AccountReport, error) {
	strategies, err := r.earnStrategies(ctx)
	if err != nil {
		return adapter.AccountReport{}, err
	}
	where := "Kraken " + epEarnAllocations.String()
	var allocations struct {
		Items      []earnAllocation `json:"items"`
		NextCursor *string          `json:"next_cursor"`
	}
	if err := r.c.call(ctx, epEarnAllocations, url.Values{"converted_asset": {"USD"}, "hide_zero_allocations": {"true"}}, &allocations); err != nil {
		return adapter.AccountReport{}, err
	}
	// Kraken documents that allocations are not paged; a cursor would mean a partial list.
	if allocations.Items == nil || (allocations.NextCursor != nil && *allocations.NextCursor != "") {
		return adapter.AccountReport{}, adapter.Malformed(where, "the complete allocation list")
	}
	totals := adapter.Totals{}
	positions := []adapter.EarnPosition{}
	for i := range allocations.Items {
		row := &allocations.Items[i]
		amount, err := adapter.Decimal(where, "amount_allocated.total.native of "+row.NativeAsset, string(row.Allocated.Total.Native))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		// Kraken can return past allocations after their strategies are no longer available.
		// A zero total has no holdings to classify, even if the filter above is ignored.
		if amount.Sign() == 0 {
			continue
		}
		lockType, ok := strategies[row.StrategyID]
		if !ok {
			return adapter.AccountReport{}, adapter.Malformed(where, "the strategy of an allocation")
		}
		inMain, known := earnInMain[lockType]
		if !known {
			return adapter.AccountReport{}, adapter.NewError(adapter.ClassNormalization, fmt.Sprintf("%s: strategy lock type %q is not documented.", where, lockType))
		}
		if inMain {
			continue
		}
		symbol, err := r.assets.symbol(where, row.NativeAsset)
		if err != nil {
			return adapter.AccountReport{}, err
		}
		if err := totals.Add(where, symbol, amount); err != nil {
			return adapter.AccountReport{}, err
		}
		positions = append(positions, row.position(symbol, decimal.Format(amount), r.earnMetadata[row.StrategyID]))
	}
	balances := totals.Balances()
	if err := adapter.CheckEarnPositions(positions, balances); err != nil {
		return adapter.AccountReport{}, err
	}
	return adapter.AccountReport{AccountType: TypeEarn, Status: adapter.StatusSucceeded, CollectedAt: r.a.now(), Balances: balances, EarnPositions: positions}, nil
}

// earnStrategies returns the lock type of every strategy, following next_cursor until Kraken
// stops returning one. A repeated cursor would loop forever and fails earn instead.
func (r *run) earnStrategies(ctx context.Context) (map[string]string, error) {
	where := "Kraken " + epEarnStrategies.String()
	out := map[string]string{}
	r.earnMetadata = map[string]earnStrategy{}
	seen := map[string]bool{}
	var params url.Values
	for range maxStrategyPages {
		var page struct {
			Items      []earnStrategy `json:"items"`
			NextCursor *string        `json:"next_cursor"`
		}
		if err := r.c.call(ctx, epEarnStrategies, params, &page); err != nil {
			return nil, err
		}
		if page.Items == nil {
			return nil, adapter.Malformed(where, "the strategy list")
		}
		for i := range page.Items {
			strategy := &page.Items[i]
			if strategy.ID == "" || strategy.LockType.Type == "" {
				return nil, adapter.Malformed(where, "a strategy id or lock type")
			}
			out[strategy.ID] = strategy.LockType.Type
			r.earnMetadata[strategy.ID] = *strategy
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return out, nil
		}
		if seen[*page.NextCursor] {
			return nil, adapter.Malformed(where, "the strategy pagination")
		}
		seen[*page.NextCursor] = true
		params = url.Values{"cursor": {*page.NextCursor}}
	}
	return nil, adapter.Malformed(where, "the strategy pagination")
}
