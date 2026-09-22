package projection

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

type facts struct {
	portfolioID  uuid.UUID
	timezone     string
	displayAsset uuid.UUID
	identities   []identityFact
	accounts     []accountFact
	connections  map[uuid.UUID]int
	listings     map[uuid.UUID]listingFact
	instruments  map[uuid.UUID]instrumentFact
	mappings     map[uuid.UUID]mappingFact
	snapshots    []snapshotFact
	balances     []balanceFact
	positions    []positionFact
	prices       []priceFact
	equities     []equityFact
	equityIDs    map[uuid.UUID][]uuid.UUID
	equityAccts  map[uuid.UUID][]uuid.UUID
	failures     []failureFact
	baseline     *capitalFact
	flows        []flowFact
}

type identityFact struct {
	ID         uuid.UUID
	PlatformID uuid.UUID
	Status     string
	CreatedAt  time.Time
	ArchivedAt *time.Time
}

type snapshotFact struct {
	ID          uuid.UUID
	IdentityID  uuid.UUID
	CompletedAt time.Time
	Status      string
}

type accountFact struct {
	ID         uuid.UUID
	IdentityID uuid.UUID
}

type listingFact struct {
	ID         uuid.UUID
	PlatformID uuid.UUID
}

type instrumentFact struct {
	ID           uuid.UUID
	PlatformID   uuid.UUID
	UnderlyingID *uuid.UUID
	BaseID       *uuid.UUID
	QuoteID      *uuid.UUID
	ContractSize string
}

type mappingFact struct {
	AssetID uuid.UUID
	State   string
}

type balanceFact struct {
	ID, SnapshotID, IdentityID, AccountID, ListingID uuid.UUID
	ObservedAt                                       time.Time
	Completeness                                     string
	Total, Available, Locked                         *string
}

type positionFact struct {
	ID, SnapshotID, IdentityID, AccountID, InstrumentID uuid.UUID
	Ref, Side, Quantity, Completeness                   string
	ObservedAt                                          time.Time
	Mark, Unrealized                                    *string
}

type priceFact struct {
	ID, IdentityID, QuoteListingID uuid.UUID
	InstrumentID, BaseListingID    *uuid.UUID
	ObservedAt                     time.Time
	Type, Price                    string
}

type equityFact struct {
	ID, SnapshotID, IdentityID, DenomListingID uuid.UUID
	ObservedAt                                 time.Time
	Completeness, Amount                       string
}

type failureFact struct {
	SnapshotID uuid.UUID
	IdentityID uuid.UUID
	AccountID  *uuid.UUID
	ScopeKind  string
	ScopeRef   string
	OccurredAt time.Time
	ErrorCode  string
	SafeDetail string
}

type capitalFact struct {
	Quantity    string
	AssetID     uuid.UUID
	EffectiveAt time.Time
	Version     uint64
}

type flowFact struct {
	Direction   string
	Quantity    string
	AssetID     uuid.UUID
	EffectiveAt time.Time
	State       string
	Version     uint64
}

func (s *Service) loadFacts(ctx context.Context, portfolioID uuid.UUID) (*facts, error) {
	row, err := s.q.GetPortfolioByID(ctx, mustBin(portfolioID))
	if err != nil {
		return nil, err
	}
	display, err := uuid.FromBytes(row.DisplayAssetID)
	if err != nil {
		return nil, err
	}
	out := &facts{
		portfolioID:  portfolioID,
		timezone:     row.Timezone,
		displayAsset: display,
		connections:  map[uuid.UUID]int{},
		listings:     map[uuid.UUID]listingFact{},
		instruments:  map[uuid.UUID]instrumentFact{},
		mappings:     map[uuid.UUID]mappingFact{},
		equityIDs:    map[uuid.UUID][]uuid.UUID{},
		equityAccts:  map[uuid.UUID][]uuid.UUID{},
	}
	if err := s.loadConfigFacts(ctx, out); err != nil {
		return nil, err
	}
	if err := s.loadObservationFacts(ctx, out); err != nil {
		return nil, err
	}
	if err := s.loadFailures(ctx, out); err != nil {
		return nil, err
	}
	return out, s.loadCapital(ctx, out)
}

func (s *Service) loadConfigFacts(ctx context.Context, out *facts) error {
	identities, err := s.q.ListIdentitiesByPortfolio(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range identities {
		id, err := uuid.FromBytes(identities[i].ID)
		if err != nil {
			return err
		}
		platform, err := uuid.FromBytes(identities[i].PlatformID)
		if err != nil {
			return err
		}
		item := identityFact{ID: id, PlatformID: platform, Status: identities[i].Status, CreatedAt: identities[i].CreatedAt}
		if identities[i].ArchivedAt.Valid {
			archived := identities[i].ArchivedAt.Time
			item.ArchivedAt = &archived
		}
		out.identities = append(out.identities, item)
	}
	accounts, err := s.q.ListAllAccountsByPortfolio(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range accounts {
		id, err := uuid.FromBytes(accounts[i].ID)
		if err != nil {
			return err
		}
		identityID, err := uuid.FromBytes(accounts[i].IdentityID)
		if err != nil {
			return err
		}
		out.accounts = append(out.accounts, accountFact{ID: id, IdentityID: identityID})
	}
	conns, err := s.q.ListConnectionsByPortfolio(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range conns {
		identityID, err := uuid.FromBytes(conns[i].IdentityID)
		if err != nil {
			return err
		}
		out.connections[identityID] = int(conns[i].FreshnessAfterSeconds)
	}
	listings, err := s.q.ListAssetListings(ctx)
	if err != nil {
		return err
	}
	for i := range listings {
		id, err := uuid.FromBytes(listings[i].ID)
		if err != nil {
			return err
		}
		platform, err := uuid.FromBytes(listings[i].PlatformID)
		if err != nil {
			return err
		}
		out.listings[id] = listingFact{ID: id, PlatformID: platform}
	}
	instruments, err := s.q.ListInstruments(ctx)
	if err != nil {
		return err
	}
	for i := range instruments {
		item, err := instrumentFromRow(&instruments[i])
		if err != nil {
			return err
		}
		out.instruments[item.ID] = item
	}
	mappings, err := s.q.ListCurrentAssetMappings(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range mappings {
		listingID, err := uuid.FromBytes(mappings[i].AssetListingID)
		if err != nil {
			return err
		}
		assetID, err := uuid.FromBytes(mappings[i].AssetID)
		if err != nil {
			return err
		}
		out.mappings[listingID] = mappingFact{AssetID: assetID, State: mappings[i].State}
	}
	return nil
}

func (s *Service) loadObservationFacts(ctx context.Context, out *facts) error {
	snapshots, err := s.q.ListIdentitySnapshotFacts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range snapshots {
		if !snapshots[i].CompletedAt.Valid {
			continue
		}
		id, err := uuid.FromBytes(snapshots[i].ID)
		if err != nil {
			return err
		}
		identityID, err := uuid.FromBytes(snapshots[i].IdentityID)
		if err != nil {
			return err
		}
		out.snapshots = append(out.snapshots, snapshotFact{
			ID: id, IdentityID: identityID, CompletedAt: snapshots[i].CompletedAt.Time, Status: snapshots[i].Status,
		})
	}
	balances, err := s.q.ListBalanceFacts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range balances {
		item, ok, err := balanceFromRow(&balances[i])
		if err != nil {
			return err
		}
		if ok {
			out.balances = append(out.balances, item)
		}
	}
	positions, err := s.q.ListPositionFacts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range positions {
		item, ok, err := positionFromRow(&positions[i])
		if err != nil {
			return err
		}
		if ok {
			out.positions = append(out.positions, item)
		}
	}
	prices, err := s.q.ListPriceFacts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range prices {
		item, err := priceFromRow(&prices[i])
		if err != nil {
			return err
		}
		out.prices = append(out.prices, item)
	}
	equities, err := s.q.ListReportedEquityFacts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range equities {
		item, err := equityFromRow(&equities[i])
		if err != nil {
			return err
		}
		out.equities = append(out.equities, item)
	}
	ids, err := s.q.ListReportedEquityCoverageIdentities(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range ids {
		obs, err := uuid.FromBytes(ids[i].ObservationID)
		if err != nil {
			return err
		}
		identityID, err := uuid.FromBytes(ids[i].IdentityID)
		if err != nil {
			return err
		}
		out.equityIDs[obs] = append(out.equityIDs[obs], identityID)
	}
	accts, err := s.q.ListReportedEquityCoverageAccounts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range accts {
		obs, err := uuid.FromBytes(accts[i].ObservationID)
		if err != nil {
			return err
		}
		accountID, err := uuid.FromBytes(accts[i].AccountID)
		if err != nil {
			return err
		}
		out.equityAccts[obs] = append(out.equityAccts[obs], accountID)
	}
	return nil
}

func (s *Service) loadFailures(ctx context.Context, out *facts) error {
	rows, err := s.q.ListCollectionFailureFacts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range rows {
		identityID, err := uuid.FromBytes(rows[i].IdentityID)
		if err != nil {
			return err
		}
		snapshotID, err := uuid.FromBytes(rows[i].IdentitySnapshotID)
		if err != nil {
			return err
		}
		ref := ""
		if rows[i].ScopeRef.Valid {
			ref = rows[i].ScopeRef.String
		}
		out.failures = append(out.failures, failureFact{
			SnapshotID: snapshotID,
			IdentityID: identityID,
			AccountID:  optionalID(rows[i].AccountID),
			ScopeKind:  rows[i].ScopeKind,
			ScopeRef:   ref,
			OccurredAt: rows[i].OccurredAt,
			ErrorCode:  rows[i].ErrorCode,
			SafeDetail: rows[i].SafeDetail,
		})
	}
	return nil
}

func (s *Service) loadCapital(ctx context.Context, out *facts) error {
	head, err := s.q.GetCapitalBaselineByPortfolio(ctx, mustBin(out.portfolioID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && len(head.CurrentRevisionID) == 16 {
		rev, err := s.q.GetCapitalBaselineRevision(ctx, sqlcgen.GetCapitalBaselineRevisionParams{
			PortfolioID: mustBin(out.portfolioID),
			ID:          head.CurrentRevisionID,
		})
		if err != nil {
			return err
		}
		assetID, err := uuid.FromBytes(rev.AssetID)
		if err != nil {
			return err
		}
		out.baseline = &capitalFact{Quantity: compactDecimal(rev.Quantity), AssetID: assetID, EffectiveAt: rev.EffectiveAt, Version: head.Version}
	}
	flows, err := s.q.ListCapitalFlowFacts(ctx, mustBin(out.portfolioID))
	if err != nil {
		return err
	}
	for i := range flows {
		assetID, err := uuid.FromBytes(flows[i].AssetID)
		if err != nil {
			return err
		}
		out.flows = append(out.flows, flowFact{
			Direction:   flows[i].Direction,
			Quantity:    compactDecimal(flows[i].Quantity),
			AssetID:     assetID,
			EffectiveAt: flows[i].EffectiveAt,
			State:       flows[i].State,
			Version:     flows[i].Version,
		})
	}
	return nil
}

func instrumentFromRow(row *sqlcgen.Instrument) (instrumentFact, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return instrumentFact{}, err
	}
	platform, err := uuid.FromBytes(row.PlatformID)
	if err != nil {
		return instrumentFact{}, err
	}
	item := instrumentFact{ID: id, PlatformID: platform, UnderlyingID: optionalID(row.UnderlyingListingID), BaseID: optionalID(row.BaseListingID), QuoteID: optionalID(row.QuoteListingID)}
	if row.ContractSize.Valid {
		item.ContractSize = compactDecimal(row.ContractSize.String)
	}
	return item, nil
}

func balanceFromRow(row *sqlcgen.ListBalanceFactsRow) (balanceFact, bool, error) {
	accountID := optionalID(row.AccountID)
	if accountID == nil {
		return balanceFact{}, false, nil
	}
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return balanceFact{}, false, err
	}
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return balanceFact{}, false, err
	}
	listingID, err := uuid.FromBytes(row.AssetListingID)
	if err != nil {
		return balanceFact{}, false, err
	}
	snapshotID, err := uuid.FromBytes(row.IdentitySnapshotID)
	if err != nil {
		return balanceFact{}, false, err
	}
	return balanceFact{
		ID: id, SnapshotID: snapshotID, IdentityID: identityID, AccountID: *accountID, ListingID: listingID,
		ObservedAt: row.SourceObservedAt, Completeness: row.Completeness,
		Total: nullStringPtr(row.TotalQuantity), Available: nullStringPtr(row.AvailableQuantity), Locked: nullStringPtr(row.LockedQuantity),
	}, true, nil
}

func positionFromRow(row *sqlcgen.ListPositionFactsRow) (positionFact, bool, error) {
	accountID := optionalID(row.AccountID)
	if accountID == nil {
		return positionFact{}, false, nil
	}
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return positionFact{}, false, err
	}
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return positionFact{}, false, err
	}
	instrumentID, err := uuid.FromBytes(row.InstrumentID)
	if err != nil {
		return positionFact{}, false, err
	}
	snapshotID, err := uuid.FromBytes(row.IdentitySnapshotID)
	if err != nil {
		return positionFact{}, false, err
	}
	return positionFact{
		ID: id, SnapshotID: snapshotID, IdentityID: identityID, AccountID: *accountID, InstrumentID: instrumentID,
		Ref: row.PlatformPositionRef, Side: row.Side, Quantity: compactDecimal(row.Quantity), Completeness: row.Completeness,
		ObservedAt: row.SourceObservedAt, Mark: nullStringPtr(row.MarkPrice), Unrealized: nullStringPtr(row.UnrealizedValue),
	}, true, nil
}

func priceFromRow(row *sqlcgen.ListPriceFactsRow) (priceFact, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return priceFact{}, err
	}
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return priceFact{}, err
	}
	quote, err := uuid.FromBytes(row.QuoteListingID)
	if err != nil {
		return priceFact{}, err
	}
	return priceFact{
		ID: id, IdentityID: identityID, QuoteListingID: quote,
		InstrumentID: optionalID(row.InstrumentID), BaseListingID: optionalID(row.BaseListingID),
		ObservedAt: row.SourceObservedAt, Type: row.PriceType, Price: compactDecimal(row.Price),
	}, nil
}

func equityFromRow(row *sqlcgen.ListReportedEquityFactsRow) (equityFact, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return equityFact{}, err
	}
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return equityFact{}, err
	}
	listingID, err := uuid.FromBytes(row.DenominationListingID)
	if err != nil {
		return equityFact{}, err
	}
	snapshotID, err := uuid.FromBytes(row.IdentitySnapshotID)
	if err != nil {
		return equityFact{}, err
	}
	return equityFact{
		ID: id, SnapshotID: snapshotID, IdentityID: identityID, DenomListingID: listingID,
		ObservedAt: row.SourceObservedAt, Completeness: row.Completeness, Amount: compactDecimal(row.Amount),
	}, nil
}

func optionalID(b []byte) *uuid.UUID {
	if len(b) != 16 {
		return nil
	}
	id, err := uuid.FromBytes(b)
	if err != nil {
		return nil
	}
	return &id
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	value := compactDecimal(v.String)
	return &value
}

func mustBin(id uuid.UUID) []byte {
	b, err := id.MarshalBinary()
	if err != nil {
		return id[:]
	}
	return b
}
