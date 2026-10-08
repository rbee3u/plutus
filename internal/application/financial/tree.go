package financial

import (
	"context"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Tree answers the Platform → Identity → Account view from the latest point.
func (s *Service) Tree(ctx context.Context, p PortfolioContext) (Tree, error) {
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return Tree{}, err
	}
	out := Tree{Context: c, Completeness: valuation.CompletenessIncomplete, Platforms: []TreePlatform{}, Issues: []valuation.Issue{}}
	if point == nil {
		out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
	} else {
		total := point.TotalUSD
		out.TotalUSD = &total
		out.Completeness = point.Completeness()
		out.Issues = point.Issues
	}
	collectedIdentities := make(map[uuid.UUID]bool)
	collectedPlatforms := make(map[string]bool)
	for _, account := range accounts {
		collectedIdentities[account.IdentityID] = true
		collectedPlatforms[account.Platform] = true
	}
	accounts, err = s.withManualAccounts(ctx, p, accounts)
	if err != nil {
		return Tree{}, err
	}
	current, err := s.currentManualBalances(ctx, p)
	if err != nil {
		return Tree{}, err
	}
	platformIndex := map[string]int{}
	for i := range accounts {
		a := &accounts[i]
		pi, ok := platformIndex[a.Platform]
		if !ok {
			pi = len(out.Platforms)
			platformIndex[a.Platform] = pi
			out.Platforms = append(out.Platforms, TreePlatform{Platform: a.Platform, Label: adapters.PlatformName(adapters.Platform(a.Platform)), Completeness: valuation.CompletenessComplete})
		}
		platform := &out.Platforms[pi]
		var identity *TreeIdentity
		for j := range platform.Identities {
			if platform.Identities[j].ID == a.IdentityID {
				identity = &platform.Identities[j]
				break
			}
		}
		if identity == nil {
			platform.Identities = append(platform.Identities, TreeIdentity{ID: a.IdentityID, DisplayName: a.IdentityName, Status: a.IdentityStatus, ConnectionStatus: a.ConnectionStatus, Completeness: valuation.CompletenessComplete})
			identity = &platform.Identities[len(platform.Identities)-1]
		}
		empty := a.ReportID == uuid.Nil
		if !empty {
			empty, err = s.confirmedEmpty(ctx, a)
			if err != nil {
				return Tree{}, err
			}
		}
		if len(current[a.AccountID]) > 0 {
			empty = false
		}
		if !empty {
			identity.Accounts = append(identity.Accounts, *a)
		}
	}
	for pi := range out.Platforms {
		platform := &out.Platforms[pi]
		platformSum := decimal.Zero()
		for ii := range platform.Identities {
			identity := &platform.Identities[ii]
			sum := decimal.Zero()
			for k := range identity.Accounts {
				a := &identity.Accounts[k]
				if a.ReportID == uuid.Nil {
					continue
				}
				if a.ValueUSD != nil {
					if value, ok := decimal.Parse(*a.ValueUSD); ok {
						sum = decimal.Add(sum, value)
					}
				}
				if a.Completeness() == valuation.CompletenessIncomplete {
					identity.Completeness = valuation.CompletenessIncomplete
					platform.Completeness = valuation.CompletenessIncomplete
				}
			}
			if collectedIdentities[identity.ID] {
				identity.ValueUSD = decimal.FormatPtr(sum)
			} else {
				identity.Completeness = valuation.CompletenessIncomplete
			}
			platformSum = decimal.Add(platformSum, sum)
		}
		if collectedPlatforms[platform.Platform] {
			platform.ValueUSD = decimal.FormatPtr(platformSum)
		} else {
			platform.Completeness = valuation.CompletenessIncomplete
		}
	}
	return out, nil
}
