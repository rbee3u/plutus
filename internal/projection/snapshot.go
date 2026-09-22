package projection

import (
	"time"

	"github.com/google/uuid"
)

func (b *builder) inWindow(ident *identityFact, cutoff time.Time) bool {
	if ident.CreatedAt.After(cutoff) {
		return false
	}
	if ident.ArchivedAt != nil && !cutoff.Before(*ident.ArchivedAt) {
		return false
	}
	return true
}

func (b *builder) requiredMember(ident *identityFact, cutoff time.Time) bool {
	if !b.inWindow(ident, cutoff) {
		return false
	}
	return ident.Status == "enabled" || ident.Status == "archived"
}

func (b *builder) requiredMembers(cutoff time.Time) []identityFact {
	out := make([]identityFact, 0, len(b.facts.identities))
	for _, ident := range b.facts.identities {
		if b.requiredMember(&ident, cutoff) {
			out = append(out, ident)
		}
	}
	return out
}

func (b *builder) hasSuccessfulSnapshot(identity uuid.UUID, cutoff time.Time) bool {
	for _, kind := range []string{"balance", "position", "reported_equity"} {
		if _, ok := b.owningSnapshot(identity, kind, cutoff, nil); ok {
			return true
		}
	}
	return false
}

func (b *builder) owningSnapshot(identity uuid.UUID, kind string, cutoff time.Time, before *time.Time) (snapshotFact, bool) {
	var chosen snapshotFact
	found := false
	for i := range b.facts.snapshots {
		snap := b.facts.snapshots[i]
		if snap.IdentityID != identity || snap.Status == "failed" || snap.CompletedAt.After(cutoff) {
			continue
		}
		if before != nil && !snap.CompletedAt.Before(*before) {
			continue
		}
		if b.wholeScopeFailed(snap.ID, kind) {
			continue
		}
		if !found || snap.CompletedAt.After(chosen.CompletedAt) || (snap.CompletedAt.Equal(chosen.CompletedAt) && snap.ID.String() > chosen.ID.String()) {
			chosen = snap
			found = true
		}
	}
	return chosen, found
}

func (b *builder) wholeScopeFailed(snapshot uuid.UUID, kind string) bool {
	for i := range b.facts.failures {
		fail := &b.facts.failures[i]
		if fail.SnapshotID == snapshot && fail.ScopeKind == kind && fail.ScopeRef == "" {
			return true
		}
	}
	return false
}

func (b *builder) selectedBalances(cutoff time.Time) []*balanceFact {
	var out []*balanceFact
	for _, ident := range b.facts.identities {
		if !b.inWindow(&ident, cutoff) {
			continue
		}
		snap, ok := b.owningSnapshot(ident.ID, "balance", cutoff, nil)
		if !ok {
			continue
		}
		failed := b.keyFailures(snap.ID, "balance")
		for i := range b.facts.balances {
			item := &b.facts.balances[i]
			if item.SnapshotID != snap.ID {
				continue
			}
			if balanceKeyFailed(failed, item.AccountID, item.ListingID) {
				continue
			}
			out = append(out, item)
		}
		for i := range failed {
			fail := &failed[i]
			if item := b.carryBalance(ident.ID, fail, snap.CompletedAt, cutoff); item != nil {
				out = append(out, item)
			}
		}
	}
	return out
}

func (b *builder) selectedPositions(cutoff time.Time) []*positionFact {
	var out []*positionFact
	for _, ident := range b.facts.identities {
		if !b.inWindow(&ident, cutoff) {
			continue
		}
		snap, ok := b.owningSnapshot(ident.ID, "position", cutoff, nil)
		if !ok {
			continue
		}
		failed := b.keyFailures(snap.ID, "position")
		for i := range b.facts.positions {
			item := &b.facts.positions[i]
			if item.SnapshotID != snap.ID {
				continue
			}
			if positionKeyFailed(failed, item.AccountID, item.Ref) {
				continue
			}
			out = append(out, item)
		}
		for i := range failed {
			fail := &failed[i]
			if item := b.carryPosition(ident.ID, fail, snap.CompletedAt, cutoff); item != nil {
				out = append(out, item)
			}
		}
	}
	return out
}

func (b *builder) selectedEquities(cutoff time.Time) []equityFact {
	var out []equityFact
	for _, ident := range b.facts.identities {
		if !b.inWindow(&ident, cutoff) {
			continue
		}
		snap, ok := b.owningSnapshot(ident.ID, "reported_equity", cutoff, nil)
		if !ok {
			continue
		}
		for i := range b.facts.equities {
			item := b.facts.equities[i]
			if item.SnapshotID == snap.ID {
				out = append(out, item)
			}
		}
	}
	return out
}

func (b *builder) keyFailures(snapshot uuid.UUID, kind string) []failureFact {
	var out []failureFact
	for i := range b.facts.failures {
		fail := b.facts.failures[i]
		if fail.SnapshotID == snapshot && fail.ScopeKind == kind && fail.ScopeRef != "" {
			out = append(out, fail)
		}
	}
	return out
}

func balanceKeyFailed(failed []failureFact, account, listing uuid.UUID) bool {
	for i := range failed {
		fail := &failed[i]
		if fail.AccountID != nil && *fail.AccountID != account {
			continue
		}
		if fail.ScopeRef == listing.String() {
			return true
		}
	}
	return false
}

func positionKeyFailed(failed []failureFact, account uuid.UUID, ref string) bool {
	for i := range failed {
		fail := &failed[i]
		if fail.ScopeRef != ref {
			continue
		}
		if fail.AccountID != nil && *fail.AccountID != account {
			continue
		}
		return true
	}
	return false
}

func (b *builder) carryBalance(identity uuid.UUID, fail *failureFact, before, cutoff time.Time) *balanceFact {
	snap, ok := b.owningSnapshot(identity, "balance", cutoff, &before)
	if !ok {
		return nil
	}
	account := uuid.Nil
	if fail.AccountID != nil {
		account = *fail.AccountID
	}
	listing, err := uuid.Parse(fail.ScopeRef)
	if err == nil && balanceKeyFailed(b.keyFailures(snap.ID, "balance"), account, listing) {
		return b.carryBalance(identity, fail, snap.CompletedAt, cutoff)
	}
	for i := range b.facts.balances {
		item := &b.facts.balances[i]
		if item.SnapshotID != snap.ID || fail.ScopeRef != item.ListingID.String() {
			continue
		}
		if fail.AccountID != nil && item.AccountID != *fail.AccountID {
			continue
		}
		return item
	}
	return nil
}

func (b *builder) carryPosition(identity uuid.UUID, fail *failureFact, before, cutoff time.Time) *positionFact {
	snap, ok := b.owningSnapshot(identity, "position", cutoff, &before)
	if !ok {
		return nil
	}
	account := uuid.Nil
	if fail.AccountID != nil {
		account = *fail.AccountID
	}
	if positionKeyFailed(b.keyFailures(snap.ID, "position"), account, fail.ScopeRef) {
		return b.carryPosition(identity, fail, snap.CompletedAt, cutoff)
	}
	for i := range b.facts.positions {
		item := &b.facts.positions[i]
		if item.SnapshotID != snap.ID || item.Ref != fail.ScopeRef {
			continue
		}
		if fail.AccountID != nil && item.AccountID != *fail.AccountID {
			continue
		}
		return item
	}
	return nil
}
