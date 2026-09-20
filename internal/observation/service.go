package observation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
	"github.com/rbee3u/plutus/internal/jobs"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

var decimalPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// Service persists normalized observations and collection facts.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs an observation service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Observation is a typed normalized fact.
type Observation struct {
	ID                 uuid.UUID
	PortfolioID        uuid.UUID
	IdentitySnapshotID uuid.UUID
	IdentityID         uuid.UUID
	AccountID          *uuid.UUID
	Kind               string
	SourceObservedAt   time.Time
	CollectedAt        time.Time
	Completeness       string
	AdapterName        string
	AdapterVersion     string
	SchemaVersion      string
	Version            uint64
	Balance            *Balance
	Position           *Position
	ReportedEquity     *ReportedEquity
	Price              *Price
	AccountMetric      *AccountMetric
}

// Balance is a balance subtype.
type Balance struct {
	AssetListingID    uuid.UUID `json:"assetListingId"`
	TotalQuantity     *string   `json:"totalQuantity,omitempty"`
	AvailableQuantity *string   `json:"availableQuantity,omitempty"`
	LockedQuantity    *string   `json:"lockedQuantity,omitempty"`
}

// Position is a position subtype.
type Position struct {
	InstrumentID        uuid.UUID `json:"instrumentId"`
	PlatformPositionRef string    `json:"platformPositionRef"`
	Side                string    `json:"side"`
	Quantity            string    `json:"quantity"`
	EntryPrice          *string   `json:"entryPrice,omitempty"`
	MarkPrice           *string   `json:"markPrice,omitempty"`
	UnrealizedValue     *string   `json:"unrealizedValue,omitempty"`
	Leverage            *string   `json:"leverage,omitempty"`
}

// ReportedEquity is a reported-equity subtype.
type ReportedEquity struct {
	DenominationListingID uuid.UUID   `json:"denominationListingId"`
	Amount                string      `json:"amount"`
	CoveredIdentityIDs    []uuid.UUID `json:"coveredIdentityIds"`
	CoveredAccountIDs     []uuid.UUID `json:"coveredAccountIds"`
}

// Price is a price subtype.
type Price struct {
	InstrumentID   *uuid.UUID `json:"instrumentId,omitempty"`
	BaseListingID  *uuid.UUID `json:"baseListingId,omitempty"`
	QuoteListingID uuid.UUID  `json:"quoteListingId"`
	PriceType      string     `json:"priceType"`
	Price          string     `json:"price"`
}

// AccountMetric is an account-metric subtype.
type AccountMetric struct {
	MetricCode       string  `json:"metricCode"`
	Unit             string  `json:"unit"`
	DecimalValue     *string `json:"decimalValue,omitempty"`
	TextValue        *string `json:"textValue,omitempty"`
	SemanticsVersion string  `json:"semanticsVersion"`
}

// Account is a discovered platform account.
type Account struct {
	ID          uuid.UUID
	PortfolioID uuid.UUID
	IdentityID  uuid.UUID
	ExternalRef string
	DisplayName string
	AccountType string
	Status      string
}

// AccountPage is an offset page of accounts.
type AccountPage struct {
	Items  []Account
	Offset int
	Limit  int
	Total  int
}

// Snapshot is an identity collection snapshot.
type Snapshot struct {
	ID              uuid.UUID
	PortfolioID     uuid.UUID
	CollectionRunID uuid.UUID
	IdentityID      uuid.UUID
}

// PatchInput is a correction payload.
type PatchInput struct {
	ActorUserID       uuid.UUID
	RequestID         string
	Completeness      *string
	TotalQuantity     *string
	AvailableQuantity *string
	LockedQuantity    *string
	Side              *string
	Quantity          *string
	EntryPrice        *string
	MarkPrice         *string
	UnrealizedValue   *string
	Leverage          *string
	Amount            *string
	Price             *string
	DecimalValue      *string
	TextValue         *string
}

// CreateAccountInput creates a discovered account.
type CreateAccountInput struct {
	IdentityID  uuid.UUID
	ExternalRef string
	DisplayName string
	AccountType string
	Status      string
}

// CreateSnapshotInput opens a collection snapshot for persisting facts.
type CreateSnapshotInput struct {
	IdentityID     uuid.UUID
	ConnectionID   uuid.UUID
	AdapterName    string
	AdapterVersion string
	StartedAt      time.Time
}

// CreateInput inserts one typed observation into an existing snapshot.
type CreateInput struct {
	IdentitySnapshotID uuid.UUID
	AccountID          *uuid.UUID
	Kind               string
	SourceObservedAt   time.Time
	Completeness       string
	AdapterName        string
	AdapterVersion     string
	SchemaVersion      string
	Balance            *Balance
	Position           *Position
	ReportedEquity     *ReportedEquity
	Price              *Price
	AccountMetric      *AccountMetric
}

// FailureInput records a failed or omitted collection scope.
type FailureInput struct {
	IdentitySnapshotID uuid.UUID
	AccountID          *uuid.UUID
	ScopeKind          string
	ScopeRef           string
	Phase              string
	ErrorClass         string
	ErrorCode          string
	SafeDetail         string
	Retriable          bool
	OccurredAt         time.Time
}

// ETag returns the observation validator.
func (o *Observation) ETag() string {
	return fmt.Sprintf(`"observation:%d"`, o.Version)
}

// ParseETag extracts the version from an observation ETag.
func ParseETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"observation:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}

// ListAccounts returns discovered accounts for a portfolio.
func (s *Service) ListAccounts(ctx context.Context, portfolioID uuid.UUID, offset, limit int) (AccountPage, error) {
	offset, limit, err := normalizePage(offset, limit)
	if err != nil {
		return AccountPage{}, err
	}
	total, err := s.q.CountAccountsByPortfolio(ctx, mustBin(portfolioID))
	if err != nil {
		return AccountPage{}, err
	}
	rows, err := s.q.ListAccountsByPortfolio(ctx, sqlcgen.ListAccountsByPortfolioParams{
		PortfolioID: mustBin(portfolioID),
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		return AccountPage{}, err
	}
	items := make([]Account, 0, len(rows))
	for i := range rows {
		item, err := accountFromRow(&rows[i])
		if err != nil {
			return AccountPage{}, err
		}
		items = append(items, item)
	}
	return AccountPage{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// CreateAccount inserts a discovered account.
func (s *Service) CreateAccount(ctx context.Context, portfolioID uuid.UUID, in *CreateAccountInput) (Account, error) {
	if in == nil {
		return Account{}, ErrInvalid
	}
	if _, err := s.q.GetIdentity(ctx, sqlcgen.GetIdentityParams{PortfolioID: mustBin(portfolioID), ID: mustBin(in.IdentityID)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, ErrNotFound
		}
		return Account{}, err
	}
	ref := strings.TrimSpace(in.ExternalRef)
	name := strings.TrimSpace(in.DisplayName)
	kind := strings.TrimSpace(in.AccountType)
	status := in.Status
	if status == "" {
		status = "active"
	}
	if ref == "" || name == "" || kind == "" || !validAccountStatus(status) {
		return Account{}, ErrInvalid
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Account{}, err
	}
	now := s.now().UTC()
	if err := s.q.InsertAccount(ctx, sqlcgen.InsertAccountParams{
		ID:          mustBin(id),
		PortfolioID: mustBin(portfolioID),
		IdentityID:  mustBin(in.IdentityID),
		ExternalRef: ref,
		DisplayName: name,
		AccountType: kind,
		Status:      status,
		FirstSeenAt: now,
		LastSeenAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		if mysql.IsDuplicate(err) {
			return Account{}, ErrInvalid
		}
		return Account{}, err
	}
	return s.getAccount(ctx, portfolioID, id)
}

// CreateSnapshot records a collection job, run, and identity snapshot.
func (s *Service) CreateSnapshot(ctx context.Context, portfolioID uuid.UUID, in *CreateSnapshotInput) (Snapshot, error) {
	if in == nil {
		return Snapshot{}, ErrInvalid
	}
	if _, err := s.q.GetIdentity(ctx, sqlcgen.GetIdentityParams{PortfolioID: mustBin(portfolioID), ID: mustBin(in.IdentityID)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Snapshot{}, ErrNotFound
		}
		return Snapshot{}, err
	}
	if _, err := s.q.GetConnection(ctx, sqlcgen.GetConnectionParams{PortfolioID: mustBin(portfolioID), ID: mustBin(in.ConnectionID)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Snapshot{}, ErrNotFound
		}
		return Snapshot{}, err
	}
	jobID, err := uuid.NewV7()
	if err != nil {
		return Snapshot{}, err
	}
	runID, err := uuid.NewV7()
	if err != nil {
		return Snapshot{}, err
	}
	snapshotID, err := uuid.NewV7()
	if err != nil {
		return Snapshot{}, err
	}
	started := in.StartedAt.UTC()
	if started.IsZero() {
		started = s.now().UTC()
	}
	adapterName := strings.TrimSpace(in.AdapterName)
	if adapterName == "" {
		adapterName = "manual"
	}
	adapterVersion := strings.TrimSpace(in.AdapterVersion)
	if adapterVersion == "" {
		adapterVersion = "1"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.InsertJob(ctx, sqlcgen.InsertJobParams{
		ID:           mustBin(jobID),
		PortfolioID:  mustBin(portfolioID),
		Kind:         jobs.KindCollection,
		Status:       "succeeded",
		Priority:     10,
		DedupeKey:    "collection:snapshot:" + snapshotID.String(),
		AvailableAt:  started,
		AttemptCount: 1,
		MaxAttempts:  5,
		CreatedAt:    started,
		UpdatedAt:    started,
		StartedAt:    sql.NullTime{Time: started, Valid: true},
		FinishedAt:   sql.NullTime{Time: started, Valid: true},
	}); err != nil {
		return Snapshot{}, err
	}
	if err := q.InsertCollectionJobDetail(ctx, sqlcgen.InsertCollectionJobDetailParams{
		PortfolioID:  mustBin(portfolioID),
		JobID:        mustBin(jobID),
		ConnectionID: mustBin(in.ConnectionID),
		JobTrigger:   "manual",
		Scope:        "identity",
	}); err != nil {
		return Snapshot{}, err
	}
	if err := q.InsertCollectionRun(ctx, sqlcgen.InsertCollectionRunParams{
		ID:                   mustBin(runID),
		PortfolioID:          mustBin(portfolioID),
		JobID:                mustBin(jobID),
		ConnectionID:         mustBin(in.ConnectionID),
		IdentityID:           mustBin(in.IdentityID),
		AdapterName:          adapterName,
		AdapterVersion:       adapterVersion,
		StartedAt:            started,
		FinishedAt:           sql.NullTime{Time: started, Valid: true},
		Status:               "succeeded",
		SuccessfulScopeCount: 0,
		FailedScopeCount:     0,
		CreatedAt:            started,
		UpdatedAt:            started,
	}); err != nil {
		return Snapshot{}, err
	}
	if err := q.InsertIdentitySnapshot(ctx, sqlcgen.InsertIdentitySnapshotParams{
		ID:                   mustBin(snapshotID),
		PortfolioID:          mustBin(portfolioID),
		CollectionRunID:      mustBin(runID),
		IdentityID:           mustBin(in.IdentityID),
		StartedAt:            started,
		CompletedAt:          sql.NullTime{Time: started, Valid: true},
		Status:               "complete",
		Completeness:         "complete",
		ExpectedScopeCount:   0,
		SuccessfulScopeCount: 0,
		FailedScopeCount:     0,
		CreatedAt:            started,
		UpdatedAt:            started,
	}); err != nil {
		return Snapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{ID: snapshotID, PortfolioID: portfolioID, CollectionRunID: runID, IdentityID: in.IdentityID}, nil
}

// Create inserts a header and exactly one subtype.
func (s *Service) Create(ctx context.Context, portfolioID uuid.UUID, in *CreateInput) (Observation, error) {
	if in == nil {
		return Observation{}, ErrInvalid
	}
	if err := validateCreate(in); err != nil {
		return Observation{}, err
	}
	snapshot, err := s.q.GetIdentitySnapshot(ctx, sqlcgen.GetIdentitySnapshotParams{
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(in.IdentitySnapshotID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Observation{}, ErrNotFound
	}
	if err != nil {
		return Observation{}, err
	}
	identityID, err := uuid.FromBytes(snapshot.IdentityID)
	if err != nil {
		return Observation{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Observation{}, err
	}
	now := s.now().UTC()
	observed := in.SourceObservedAt.UTC()
	if observed.IsZero() {
		observed = now
	}
	completeness := in.Completeness
	if completeness == "" {
		completeness = "complete"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Observation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	header := sqlcgen.InsertObservationParams{
		ID:                 mustBin(id),
		PortfolioID:        mustBin(portfolioID),
		IdentitySnapshotID: mustBin(in.IdentitySnapshotID),
		IdentityID:         mustBin(identityID),
		Kind:               in.Kind,
		SourceObservedAt:   observed,
		CollectedAt:        now,
		Completeness:       completeness,
		AdapterName:        defaultString(in.AdapterName, "manual"),
		AdapterVersion:     defaultString(in.AdapterVersion, "1"),
		SchemaVersion:      defaultString(in.SchemaVersion, "1"),
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if in.AccountID != nil {
		header.AccountID = mustBin(*in.AccountID)
	}
	if err := q.InsertObservation(ctx, header); err != nil {
		return Observation{}, err
	}
	if err := insertSubtype(ctx, q, portfolioID, id, in); err != nil {
		return Observation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Observation{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// RecordFailure persists an explicit collection failure without manufacturing zeros.
func (s *Service) RecordFailure(ctx context.Context, portfolioID uuid.UUID, in *FailureInput) error {
	if in == nil {
		return ErrInvalid
	}
	if strings.TrimSpace(in.ScopeKind) == "" || strings.TrimSpace(in.ErrorClass) == "" || strings.TrimSpace(in.ErrorCode) == "" {
		return ErrInvalid
	}
	snapshot, err := s.q.GetIdentitySnapshot(ctx, sqlcgen.GetIdentitySnapshotParams{
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(in.IdentitySnapshotID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	occurred := in.OccurredAt.UTC()
	if occurred.IsZero() {
		occurred = s.now().UTC()
	}
	params := sqlcgen.InsertCollectionFailureParams{
		ID:                 mustBin(id),
		PortfolioID:        mustBin(portfolioID),
		CollectionRunID:    snapshot.CollectionRunID,
		IdentitySnapshotID: mustBin(in.IdentitySnapshotID),
		ScopeKind:          in.ScopeKind,
		Phase:              defaultString(in.Phase, "normalize"),
		ErrorClass:         in.ErrorClass,
		ErrorCode:          in.ErrorCode,
		SafeDetail:         in.SafeDetail,
		Retriable:          in.Retriable,
		OccurredAt:         occurred,
	}
	if in.AccountID != nil {
		params.AccountID = mustBin(*in.AccountID)
	}
	if in.ScopeRef != "" {
		params.ScopeRef = sql.NullString{String: in.ScopeRef, Valid: true}
	}
	return s.q.InsertCollectionFailure(ctx, params)
}

// Get returns one typed observation.
func (s *Service) Get(ctx context.Context, portfolioID, observationID uuid.UUID) (Observation, error) {
	return s.getWith(ctx, s.q, portfolioID, observationID)
}

// Patch corrects permitted fields, writes canonical audit documents, and increments version.
func (s *Service) Patch(ctx context.Context, portfolioID, observationID uuid.UUID, version uint64, in *PatchInput) (Observation, error) {
	if in == nil {
		return Observation{}, ErrInvalid
	}
	current, err := s.Get(ctx, portfolioID, observationID)
	if err != nil {
		return Observation{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if in.empty() {
		return Observation{}, ErrInvalid
	}
	updated, err := applyPatch(&current, in)
	if err != nil {
		return Observation{}, err
	}
	before, err := json.Marshal(canonical(&current))
	if err != nil {
		return Observation{}, err
	}
	after, err := json.Marshal(canonical(&updated))
	if err != nil {
		return Observation{}, err
	}
	auditID, err := uuid.NewV7()
	if err != nil {
		return Observation{}, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Observation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.UpdateObservationHeader(ctx, sqlcgen.UpdateObservationHeaderParams{
		Completeness: updated.Completeness,
		UpdatedAt:    now,
		PortfolioID:  mustBin(portfolioID),
		ID:           mustBin(observationID),
		Version:      version,
	})
	if err != nil {
		return Observation{}, err
	}
	if n == 0 {
		latest, getErr := s.Get(ctx, portfolioID, observationID)
		if getErr != nil {
			return Observation{}, getErr
		}
		return latest, ErrPrecondition
	}
	if err := updateSubtype(ctx, q, &updated); err != nil {
		return Observation{}, err
	}
	if err := q.InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
		ID:           mustBin(auditID),
		PortfolioID:  mustBin(portfolioID),
		ActorUserID:  mustBin(in.ActorUserID),
		Action:       "observation.correct",
		ResourceType: "observation",
		ResourceID:   mustBin(observationID),
		RequestID:    defaultString(in.RequestID, "unknown"),
		BeforeJson:   sqljson.Column(before),
		AfterJson:    sqljson.Column(after),
		OccurredAt:   now,
	}); err != nil {
		return Observation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Observation{}, err
	}
	return s.Get(ctx, portfolioID, observationID)
}

func (s *Service) getWith(ctx context.Context, q *sqlcgen.Queries, portfolioID, observationID uuid.UUID) (Observation, error) {
	row, err := q.GetObservation(ctx, sqlcgen.GetObservationParams{PortfolioID: mustBin(portfolioID), ID: mustBin(observationID)})
	if errors.Is(err, sql.ErrNoRows) {
		return Observation{}, ErrNotFound
	}
	if err != nil {
		return Observation{}, err
	}
	item, err := observationFromHeader(&row)
	if err != nil {
		return Observation{}, err
	}
	switch item.Kind {
	case "balance":
		sub, err := q.GetBalanceObservation(ctx, sqlcgen.GetBalanceObservationParams{PortfolioID: mustBin(portfolioID), ObservationID: mustBin(observationID)})
		if err != nil {
			return Observation{}, err
		}
		listingID, err := uuid.FromBytes(sub.AssetListingID)
		if err != nil {
			return Observation{}, err
		}
		item.Balance = &Balance{
			AssetListingID:    listingID,
			TotalQuantity:     ptrString(sub.TotalQuantity),
			AvailableQuantity: ptrString(sub.AvailableQuantity),
			LockedQuantity:    ptrString(sub.LockedQuantity),
		}
	case "position":
		sub, err := q.GetPositionObservation(ctx, sqlcgen.GetPositionObservationParams{PortfolioID: mustBin(portfolioID), ObservationID: mustBin(observationID)})
		if err != nil {
			return Observation{}, err
		}
		instrumentID, err := uuid.FromBytes(sub.InstrumentID)
		if err != nil {
			return Observation{}, err
		}
		item.Position = &Position{
			InstrumentID:        instrumentID,
			PlatformPositionRef: sub.PlatformPositionRef,
			Side:                sub.Side,
			Quantity:            compactDecimal(sub.Quantity),
			EntryPrice:          ptrString(sub.EntryPrice),
			MarkPrice:           ptrString(sub.MarkPrice),
			UnrealizedValue:     ptrString(sub.UnrealizedValue),
			Leverage:            ptrString(sub.Leverage),
		}
	case "reported_equity":
		sub, err := q.GetReportedEquityObservation(ctx, sqlcgen.GetReportedEquityObservationParams{PortfolioID: mustBin(portfolioID), ObservationID: mustBin(observationID)})
		if err != nil {
			return Observation{}, err
		}
		listingID, err := uuid.FromBytes(sub.DenominationListingID)
		if err != nil {
			return Observation{}, err
		}
		identities, err := q.ListReportedEquityCoveredIdentities(ctx, sqlcgen.ListReportedEquityCoveredIdentitiesParams{PortfolioID: mustBin(portfolioID), ObservationID: mustBin(observationID)})
		if err != nil {
			return Observation{}, err
		}
		accounts, err := q.ListReportedEquityCoveredAccounts(ctx, sqlcgen.ListReportedEquityCoveredAccountsParams{PortfolioID: mustBin(portfolioID), ObservationID: mustBin(observationID)})
		if err != nil {
			return Observation{}, err
		}
		item.ReportedEquity = &ReportedEquity{
			DenominationListingID: listingID,
			Amount:                compactDecimal(sub.Amount),
			CoveredIdentityIDs:    idsFromBytes(identities),
			CoveredAccountIDs:     idsFromBytes(accounts),
		}
	case "price":
		sub, err := q.GetPriceObservation(ctx, sqlcgen.GetPriceObservationParams{PortfolioID: mustBin(portfolioID), ObservationID: mustBin(observationID)})
		if err != nil {
			return Observation{}, err
		}
		quoteID, err := uuid.FromBytes(sub.QuoteListingID)
		if err != nil {
			return Observation{}, err
		}
		item.Price = &Price{QuoteListingID: quoteID, PriceType: sub.PriceType, Price: compactDecimal(sub.Price), InstrumentID: optionalID(sub.InstrumentID), BaseListingID: optionalID(sub.BaseListingID)}
	case "account_metric":
		sub, err := q.GetAccountMetricObservation(ctx, sqlcgen.GetAccountMetricObservationParams{PortfolioID: mustBin(portfolioID), ObservationID: mustBin(observationID)})
		if err != nil {
			return Observation{}, err
		}
		item.AccountMetric = &AccountMetric{
			MetricCode:       sub.MetricCode,
			Unit:             sub.Unit,
			DecimalValue:     ptrString(sub.DecimalValue),
			TextValue:        ptrString(sub.TextValue),
			SemanticsVersion: sub.SemanticsVersion,
		}
	default:
		return Observation{}, ErrInvalid
	}
	return item, nil
}

func (s *Service) getAccount(ctx context.Context, portfolioID, id uuid.UUID) (Account, error) {
	row, err := s.q.GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: mustBin(portfolioID), ID: mustBin(id)})
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return accountFromRow(&row)
}

func insertSubtype(ctx context.Context, q *sqlcgen.Queries, portfolioID, observationID uuid.UUID, in *CreateInput) error {
	switch in.Kind {
	case "balance":
		if in.Balance.TotalQuantity == nil && in.Balance.AvailableQuantity == nil && in.Balance.LockedQuantity == nil {
			return ErrInvalid
		}
		return q.InsertBalanceObservation(ctx, sqlcgen.InsertBalanceObservationParams{
			PortfolioID:       mustBin(portfolioID),
			ObservationID:     mustBin(observationID),
			AssetListingID:    mustBin(in.Balance.AssetListingID),
			TotalQuantity:     nullFromPtr(in.Balance.TotalQuantity),
			AvailableQuantity: nullFromPtr(in.Balance.AvailableQuantity),
			LockedQuantity:    nullFromPtr(in.Balance.LockedQuantity),
		})
	case "position":
		return q.InsertPositionObservation(ctx, sqlcgen.InsertPositionObservationParams{
			PortfolioID:         mustBin(portfolioID),
			ObservationID:       mustBin(observationID),
			InstrumentID:        mustBin(in.Position.InstrumentID),
			PlatformPositionRef: in.Position.PlatformPositionRef,
			Side:                in.Position.Side,
			Quantity:            in.Position.Quantity,
			EntryPrice:          nullFromPtr(in.Position.EntryPrice),
			MarkPrice:           nullFromPtr(in.Position.MarkPrice),
			UnrealizedValue:     nullFromPtr(in.Position.UnrealizedValue),
			Leverage:            nullFromPtr(in.Position.Leverage),
		})
	case "reported_equity":
		if err := q.InsertReportedEquityObservation(ctx, sqlcgen.InsertReportedEquityObservationParams{
			PortfolioID:           mustBin(portfolioID),
			ObservationID:         mustBin(observationID),
			DenominationListingID: mustBin(in.ReportedEquity.DenominationListingID),
			Amount:                in.ReportedEquity.Amount,
		}); err != nil {
			return err
		}
		for _, id := range in.ReportedEquity.CoveredIdentityIDs {
			if err := q.InsertReportedEquityCoveredIdentity(ctx, sqlcgen.InsertReportedEquityCoveredIdentityParams{
				PortfolioID:   mustBin(portfolioID),
				ObservationID: mustBin(observationID),
				IdentityID:    mustBin(id),
			}); err != nil {
				return err
			}
		}
		for _, id := range in.ReportedEquity.CoveredAccountIDs {
			if err := q.InsertReportedEquityCoveredAccount(ctx, sqlcgen.InsertReportedEquityCoveredAccountParams{
				PortfolioID:   mustBin(portfolioID),
				ObservationID: mustBin(observationID),
				AccountID:     mustBin(id),
			}); err != nil {
				return err
			}
		}
		return nil
	case "price":
		params := sqlcgen.InsertPriceObservationParams{
			PortfolioID:    mustBin(portfolioID),
			ObservationID:  mustBin(observationID),
			QuoteListingID: mustBin(in.Price.QuoteListingID),
			PriceType:      in.Price.PriceType,
			Price:          in.Price.Price,
		}
		if in.Price.InstrumentID != nil {
			params.InstrumentID = mustBin(*in.Price.InstrumentID)
		}
		if in.Price.BaseListingID != nil {
			params.BaseListingID = mustBin(*in.Price.BaseListingID)
		}
		return q.InsertPriceObservation(ctx, params)
	case "account_metric":
		return q.InsertAccountMetricObservation(ctx, sqlcgen.InsertAccountMetricObservationParams{
			PortfolioID:      mustBin(portfolioID),
			ObservationID:    mustBin(observationID),
			MetricCode:       in.AccountMetric.MetricCode,
			Unit:             in.AccountMetric.Unit,
			DecimalValue:     nullFromPtr(in.AccountMetric.DecimalValue),
			TextValue:        nullString(in.AccountMetric.TextValue),
			SemanticsVersion: in.AccountMetric.SemanticsVersion,
		})
	default:
		return ErrInvalid
	}
}

func updateSubtype(ctx context.Context, q *sqlcgen.Queries, item *Observation) error {
	switch item.Kind {
	case "balance":
		return q.UpdateBalanceObservation(ctx, sqlcgen.UpdateBalanceObservationParams{
			TotalQuantity:     nullFromPtr(item.Balance.TotalQuantity),
			AvailableQuantity: nullFromPtr(item.Balance.AvailableQuantity),
			LockedQuantity:    nullFromPtr(item.Balance.LockedQuantity),
			PortfolioID:       mustBin(item.PortfolioID),
			ObservationID:     mustBin(item.ID),
		})
	case "position":
		return q.UpdatePositionObservation(ctx, sqlcgen.UpdatePositionObservationParams{
			Side:            item.Position.Side,
			Quantity:        item.Position.Quantity,
			EntryPrice:      nullFromPtr(item.Position.EntryPrice),
			MarkPrice:       nullFromPtr(item.Position.MarkPrice),
			UnrealizedValue: nullFromPtr(item.Position.UnrealizedValue),
			Leverage:        nullFromPtr(item.Position.Leverage),
			PortfolioID:     mustBin(item.PortfolioID),
			ObservationID:   mustBin(item.ID),
		})
	case "reported_equity":
		return q.UpdateReportedEquityObservation(ctx, sqlcgen.UpdateReportedEquityObservationParams{
			Amount:        item.ReportedEquity.Amount,
			PortfolioID:   mustBin(item.PortfolioID),
			ObservationID: mustBin(item.ID),
		})
	case "price":
		return q.UpdatePriceObservation(ctx, sqlcgen.UpdatePriceObservationParams{
			Price:         item.Price.Price,
			PortfolioID:   mustBin(item.PortfolioID),
			ObservationID: mustBin(item.ID),
		})
	case "account_metric":
		return q.UpdateAccountMetricObservation(ctx, sqlcgen.UpdateAccountMetricObservationParams{
			DecimalValue:  nullFromPtr(item.AccountMetric.DecimalValue),
			TextValue:     nullString(item.AccountMetric.TextValue),
			PortfolioID:   mustBin(item.PortfolioID),
			ObservationID: mustBin(item.ID),
		})
	default:
		return ErrInvalid
	}
}

func validateCreate(in *CreateInput) error {
	if in == nil {
		return ErrInvalid
	}
	count := 0
	if in.Balance != nil {
		count++
	}
	if in.Position != nil {
		count++
	}
	if in.ReportedEquity != nil {
		count++
	}
	if in.Price != nil {
		count++
	}
	if in.AccountMetric != nil {
		count++
	}
	if count != 1 {
		return ErrInvalid
	}
	switch in.Kind {
	case "balance":
		if in.Balance == nil {
			return ErrInvalid
		}
	case "position":
		if in.Position == nil || in.Position.Quantity == "" || !validSide(in.Position.Side) {
			return ErrInvalid
		}
		if _, err := normalizeDecimal(in.Position.Quantity); err != nil {
			return err
		}
	case "reported_equity":
		if in.ReportedEquity == nil {
			return ErrInvalid
		}
		if _, err := normalizeDecimal(in.ReportedEquity.Amount); err != nil {
			return err
		}
		if len(in.ReportedEquity.CoveredIdentityIDs) == 0 && len(in.ReportedEquity.CoveredAccountIDs) == 0 {
			return ErrCoverageRequired
		}
	case "price":
		if in.Price == nil || !validPriceType(in.Price.PriceType) {
			return ErrInvalid
		}
		if (in.Price.InstrumentID == nil) == (in.Price.BaseListingID == nil) {
			return ErrInvalid
		}
		if _, err := normalizeDecimal(in.Price.Price); err != nil {
			return err
		}
	case "account_metric":
		if in.AccountMetric == nil {
			return ErrInvalid
		}
		if (in.AccountMetric.DecimalValue == nil) == (in.AccountMetric.TextValue == nil) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if in.Completeness != "" && in.Completeness != "complete" && in.Completeness != "partial" {
		return ErrInvalid
	}
	return nil
}

func applyPatch(current *Observation, in *PatchInput) (Observation, error) {
	updated := *current
	if in.Completeness != nil {
		if *in.Completeness != "complete" && *in.Completeness != "partial" {
			return Observation{}, ErrInvalid
		}
		updated.Completeness = *in.Completeness
	}
	switch current.Kind {
	case "balance":
		if current.Balance == nil {
			return Observation{}, ErrInvalid
		}
		next := *current.Balance
		var err error
		if next.TotalQuantity, err = mergeDecimal(next.TotalQuantity, in.TotalQuantity); err != nil {
			return Observation{}, err
		}
		if next.AvailableQuantity, err = mergeDecimal(next.AvailableQuantity, in.AvailableQuantity); err != nil {
			return Observation{}, err
		}
		if next.LockedQuantity, err = mergeDecimal(next.LockedQuantity, in.LockedQuantity); err != nil {
			return Observation{}, err
		}
		if next.TotalQuantity == nil && next.AvailableQuantity == nil && next.LockedQuantity == nil {
			return Observation{}, ErrInvalid
		}
		if in.Side != nil || in.Quantity != nil || in.Amount != nil || in.Price != nil {
			return Observation{}, ErrInvalid
		}
		updated.Balance = &next
	case "position":
		if current.Position == nil {
			return Observation{}, ErrInvalid
		}
		next := *current.Position
		if in.Side != nil {
			if !validSide(*in.Side) {
				return Observation{}, ErrInvalid
			}
			next.Side = *in.Side
		}
		var err error
		if in.Quantity != nil {
			next.Quantity, err = normalizeDecimal(*in.Quantity)
			if err != nil {
				return Observation{}, err
			}
		}
		if next.EntryPrice, err = mergeDecimal(next.EntryPrice, in.EntryPrice); err != nil {
			return Observation{}, err
		}
		if next.MarkPrice, err = mergeDecimal(next.MarkPrice, in.MarkPrice); err != nil {
			return Observation{}, err
		}
		if next.UnrealizedValue, err = mergeDecimal(next.UnrealizedValue, in.UnrealizedValue); err != nil {
			return Observation{}, err
		}
		if next.Leverage, err = mergeDecimal(next.Leverage, in.Leverage); err != nil {
			return Observation{}, err
		}
		updated.Position = &next
	case "reported_equity":
		if current.ReportedEquity == nil {
			return Observation{}, ErrInvalid
		}
		if in.Amount == nil {
			return updated, nil
		}
		amount, err := normalizeDecimal(*in.Amount)
		if err != nil {
			return Observation{}, err
		}
		next := *current.ReportedEquity
		next.Amount = amount
		updated.ReportedEquity = &next
	case "price":
		if current.Price == nil {
			return Observation{}, ErrInvalid
		}
		if in.Price == nil {
			return updated, nil
		}
		price, err := normalizeDecimal(*in.Price)
		if err != nil {
			return Observation{}, err
		}
		next := *current.Price
		next.Price = price
		updated.Price = &next
	case "account_metric":
		if current.AccountMetric == nil {
			return Observation{}, ErrInvalid
		}
		next := *current.AccountMetric
		if in.DecimalValue != nil {
			value, err := normalizeDecimal(*in.DecimalValue)
			if err != nil {
				return Observation{}, err
			}
			next.DecimalValue = &value
			next.TextValue = nil
		}
		if in.TextValue != nil {
			text := strings.TrimSpace(*in.TextValue)
			if text == "" {
				return Observation{}, ErrInvalid
			}
			next.TextValue = &text
			next.DecimalValue = nil
		}
		if next.DecimalValue == nil && next.TextValue == nil {
			return Observation{}, ErrInvalid
		}
		updated.AccountMetric = &next
	default:
		return Observation{}, ErrInvalid
	}
	return updated, nil
}

func (in *PatchInput) empty() bool {
	return in.Completeness == nil && in.TotalQuantity == nil && in.AvailableQuantity == nil && in.LockedQuantity == nil &&
		in.Side == nil && in.Quantity == nil && in.EntryPrice == nil && in.MarkPrice == nil && in.UnrealizedValue == nil &&
		in.Leverage == nil && in.Amount == nil && in.Price == nil && in.DecimalValue == nil && in.TextValue == nil
}

type canonicalObservation struct {
	Kind           string          `json:"kind"`
	Completeness   string          `json:"completeness"`
	Balance        *Balance        `json:"balance,omitempty"`
	Position       *Position       `json:"position,omitempty"`
	ReportedEquity *ReportedEquity `json:"reportedEquity,omitempty"`
	Price          *Price          `json:"price,omitempty"`
	AccountMetric  *AccountMetric  `json:"accountMetric,omitempty"`
}

func canonical(item *Observation) canonicalObservation {
	return canonicalObservation{
		Kind:           item.Kind,
		Completeness:   item.Completeness,
		Balance:        item.Balance,
		Position:       item.Position,
		ReportedEquity: item.ReportedEquity,
		Price:          item.Price,
		AccountMetric:  item.AccountMetric,
	}
}

func observationFromHeader(row *sqlcgen.Observation) (Observation, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Observation{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Observation{}, err
	}
	snapshotID, err := uuid.FromBytes(row.IdentitySnapshotID)
	if err != nil {
		return Observation{}, err
	}
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return Observation{}, err
	}
	return Observation{
		ID:                 id,
		PortfolioID:        portfolioID,
		IdentitySnapshotID: snapshotID,
		IdentityID:         identityID,
		AccountID:          optionalID(row.AccountID),
		Kind:               row.Kind,
		SourceObservedAt:   row.SourceObservedAt,
		CollectedAt:        row.CollectedAt,
		Completeness:       row.Completeness,
		AdapterName:        row.AdapterName,
		AdapterVersion:     row.AdapterVersion,
		SchemaVersion:      row.SchemaVersion,
		Version:            row.Version,
	}, nil
}

func accountFromRow(row *sqlcgen.Account) (Account, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Account{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Account{}, err
	}
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return Account{}, err
	}
	return Account{
		ID:          id,
		PortfolioID: portfolioID,
		IdentityID:  identityID,
		ExternalRef: row.ExternalRef,
		DisplayName: row.DisplayName,
		AccountType: row.AccountType,
		Status:      row.Status,
	}, nil
}

func normalizePage(offset, limit int) (int, int, error) {
	if offset < 0 {
		return 0, 0, ErrInvalidPage
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return 0, 0, ErrInvalidPage
	}
	return offset, limit, nil
}

func normalizeDecimal(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !decimalPattern.MatchString(value) {
		return "", ErrInvalid
	}
	return value, nil
}

func mergeDecimal(current *string, patch *string) (*string, error) {
	if patch == nil {
		return current, nil
	}
	value, err := normalizeDecimal(*patch)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func ptrString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := compactDecimal(v.String)
	return &s
}

func compactDecimal(value string) string {
	if value == "" || !strings.Contains(value, ".") {
		return value
	}
	trimmed := strings.TrimRight(value, "0")
	trimmed = strings.TrimRight(trimmed, ".")
	if trimmed == "" || trimmed == "-" {
		return "0"
	}
	return trimmed
}

func nullFromPtr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func nullString(v *string) sql.NullString {
	if v == nil || *v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
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

func idsFromBytes(values [][]byte) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(values))
	for _, raw := range values {
		id, err := uuid.FromBytes(raw)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

func mustBin(id uuid.UUID) []byte {
	b, err := id.MarshalBinary()
	if err != nil {
		return id[:]
	}
	return b
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func validSide(side string) bool {
	return side == "long" || side == "short" || side == "net"
}

func validPriceType(value string) bool {
	return value == "market" || value == "mark" || value == "index" || value == "exchange_rate"
}

func validAccountStatus(status string) bool {
	return status == "active" || status == "inactive" || status == "disappeared"
}
