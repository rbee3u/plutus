package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/secret"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/audit"
)

// PutCredentials encrypts and stores read-only API credentials. The connection becomes disabled until enabled.
func (s *Service) PutCredentials(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput, creds adapter.Credentials) (Identity, error) {
	if s.box == nil {
		return Identity{}, ErrMasterKeyRequired
	}
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	if err := adapter.CheckCredentials(current.Platform, current.AccountStructure, selectedTypes(current.Accounts), creds); err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	keyVersion := secret.CurrentKeyVersion()
	if keyVersion == 0 || keyVersion > math.MaxInt16 {
		return Identity{}, errors.New("credential key version exceeds the SQL driver representation")
	}
	ciphertext, nonce, err := s.box.Seal(id, portfolioID, keyVersion, secret.Credentials{APIKey: creds.APIKey, APISecret: creds.APISecret, Passphrase: creds.Passphrase, FuturesAPIKey: creds.FuturesKey(), FuturesAPISecret: creds.FuturesSecret()})
	if errors.Is(err, secret.ErrInvalidCredentials) {
		return Identity{}, ErrInvalid
	}
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		n, err := q.SetIdentityCredentials(ctx, sqlcgen.SetIdentityCredentialsParams{
			CredentialsCiphertext: ciphertext, CredentialsNonce: nonce, CredentialsKeyVersion: sql.NullInt16{Int16: int16(keyVersion), Valid: true}, // #nosec G115 -- Explicitly bounded to 1..MaxInt16 above.
			CredentialsSetAt: sql.NullTime{Time: now, Valid: true}, UpdatedAt: now,
			PortfolioID: portfolioID[:], ID: id[:], Version: version,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.credentials.set", ResourceType: "identity", ResourceID: id,
			RequestID: in.RequestID, Metadata: map[string]any{"keyVersion": keyVersion},
		})
	}); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// ClearCredentials removes stored credentials and returns the connection to unconfigured.
func (s *Service) ClearCredentials(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput) (Identity, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	now := s.now().UTC()
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		n, err := q.ClearIdentityCredentials(ctx, sqlcgen.ClearIdentityCredentialsParams{UpdatedAt: now, PortfolioID: portfolioID[:], ID: id[:], Version: version})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.credentials.clear", ResourceType: "identity", ResourceID: id, RequestID: in.RequestID,
		})
	}); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Validate runs one read-only platform call and stores the outcome without changing enablement,
// except that an authentication or authorization failure marks the connection invalid.
func (s *Service) Validate(ctx context.Context, portfolioID, id uuid.UUID, in ActionInput) (Identity, ValidationOutcome, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, ValidationOutcome{}, err
	}
	if current.Status == StatusArchived {
		return current, ValidationOutcome{}, ErrArchived
	}
	outcome, err := s.probe(ctx, &current)
	if err != nil {
		return current, ValidationOutcome{}, err
	}
	next := current.Connection.Status
	switch {
	case outcome.OK && next == ConnectionInvalid:
		next = ConnectionDisabled
	case !outcome.OK && adapter.StopsScheduling(outcome.ErrorClass):
		next = ConnectionInvalid
	}
	updated, err := s.storeOutcome(ctx, &current, current.Version, next, &outcome, in, "identity.connection.validate")
	if err != nil {
		return current, outcome, err
	}
	return updated, outcome, nil
}

// Enable validates first and then marks the connection enabled for scheduling.
func (s *Service) Enable(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput) (Identity, ValidationOutcome, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, ValidationOutcome{}, err
	}
	if current.Version != version {
		return current, ValidationOutcome{}, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ValidationOutcome{}, ErrArchived
	}
	outcome, err := s.probe(ctx, &current)
	if err != nil {
		return current, ValidationOutcome{}, err
	}
	next := ConnectionEnabled
	if !outcome.OK {
		next = current.Connection.Status
		if adapter.StopsScheduling(outcome.ErrorClass) {
			next = ConnectionInvalid
		}
	}
	updated, err := s.storeOutcome(ctx, &current, version, next, &outcome, in, "identity.connection.enable")
	if err != nil {
		return current, outcome, err
	}
	if !outcome.OK {
		return updated, outcome, ErrValidationFailed
	}
	return updated, outcome, nil
}

// Disable stops scheduling the connection.
func (s *Service) Disable(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput) (Identity, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	if !current.Connection.CredentialsConfigured {
		return current, ErrCredentialsMissing
	}
	outcome := ValidationOutcome{OK: true, ValidatedAt: time.Time{}}
	updated, err := s.storeOutcome(ctx, &current, version, ConnectionDisabled, &outcome, in, "identity.connection.disable")
	if err != nil {
		return current, err
	}
	return updated, nil
}

// probe runs one read-only validation of the stored credentials. Besides the adapter's own
// checks it refuses a key that reaches another platform account than the one the identity is
// bound to (its external reference), and a margin mode that does not belong to the structure.
func (s *Service) probe(ctx context.Context, current *Identity) (ValidationOutcome, error) {
	creds, err := s.credentials(ctx, current.PortfolioID, current.ID)
	if err != nil {
		return ValidationOutcome{}, err
	}
	platformAdapter, ok := s.adapters.Lookup(current.Platform)
	if !ok {
		return ValidationOutcome{}, ErrAdapterUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, s.validateTimeout)
	defer cancel()
	now := s.now().UTC()
	failed := func(err error) ValidationOutcome {
		class, detail := adapter.Classify(err)
		return ValidationOutcome{ErrorClass: class, ErrorDetail: detail, ValidatedAt: now}
	}
	if err := adapter.CheckCredentials(current.Platform, current.AccountStructure, selectedTypes(current.Accounts), creds); err != nil {
		return failed(err), nil
	}
	result, err := platformAdapter.Validate(callCtx, creds, current.AccountStructure)
	if err != nil {
		return failed(err), nil
	}
	if result.ExternalRef == "" {
		return failed(adapter.NewError(adapter.ClassNormalization, "The platform returned no stable login reference.")), nil
	}
	if bound := current.ExternalRef; bound != nil && result.ExternalRef != "" && result.ExternalRef != *bound {
		return failed(adapter.NewError(adapter.ClassUnsupported, fmt.Sprintf(
			"This key belongs to %s account %s, but this identity is account %s; create a new identity for the other account.",
			adapter.PlatformName(current.Platform), result.ExternalRef, *bound))), nil
	}
	if !adapter.ValidMarginMode(current.AccountStructure, result.MarginMode) {
		return failed(adapter.UnknownMarginMode(current.Platform, string(result.MarginMode))), nil
	}
	return ValidationOutcome{OK: true, ExternalRef: result.ExternalRef, MarginMode: result.MarginMode, ValidatedAt: now}, nil
}

func (s *Service) storeOutcome(ctx context.Context, current *Identity, version uint64, status string, outcome *ValidationOutcome, in ActionInput, action string) (Identity, error) {
	portfolioID, id := current.PortfolioID, current.ID
	now := s.now().UTC()
	params := sqlcgen.SetIdentityConnectionStatusParams{
		ConnectionStatus: status, UpdatedAt: now, PortfolioID: portfolioID[:], ID: id[:], Version: version,
	}
	if outcome.ExternalRef != "" {
		params.PlatformLoginRef = sql.NullString{String: outcome.ExternalRef, Valid: true}
	}
	metadata := map[string]any{"ok": outcome.OK, "status": status, "errorClass": outcome.ErrorClass}
	if outcome.OK && outcome.MarginMode != "" {
		params.LastObservedMarginMode = sql.NullString{String: string(outcome.MarginMode), Valid: true}
		metadata["marginMode"] = string(outcome.MarginMode)
		if current.MarginMode != outcome.MarginMode {
			metadata["previousMarginMode"] = nullableMode(current.MarginMode)
		}
	}
	if outcome.OK && !outcome.ValidatedAt.IsZero() {
		params.LastValidationSucceededAt = sql.NullTime{Time: outcome.ValidatedAt, Valid: true}
	}
	if !outcome.OK {
		params.LastErrorClass = sql.NullString{String: outcome.ErrorClass, Valid: true}
		params.LastErrorDetail = sql.NullString{String: outcome.ErrorDetail, Valid: true}
	}
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, current.PortfolioID[:]); err != nil {
			return err
		}
		n, err := q.SetIdentityConnectionStatus(ctx, params)
		if mysql.IsDuplicate(err) {
			return ErrLoginConflict
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: action, ResourceType: "identity", ResourceID: id, RequestID: in.RequestID,
			Metadata: metadata,
		})
	}); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

func (s *Service) credentials(ctx context.Context, portfolioID, id uuid.UUID) (adapter.Credentials, error) {
	row, err := mysql.Queries(ctx, s.db).GetIdentityCredentials(ctx, sqlcgen.GetIdentityCredentialsParams{PortfolioID: portfolioID[:], ID: id[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return adapter.Credentials{}, ErrNotFound
	}
	if err != nil {
		return adapter.Credentials{}, err
	}
	return s.open(id, portfolioID, row.CredentialsKeyVersion, row.CredentialsCiphertext, row.CredentialsNonce)
}

func (s *Service) open(id, portfolioID uuid.UUID, keyVersion sql.NullInt16, ciphertext, nonce []byte) (adapter.Credentials, error) {
	if len(ciphertext) == 0 || !keyVersion.Valid || keyVersion.Int16 <= 0 || len(nonce) != 12 {
		return adapter.Credentials{}, ErrCredentialsMissing
	}
	if s.box == nil {
		return adapter.Credentials{}, ErrMasterKeyRequired
	}
	creds, err := s.box.Open(id, portfolioID, uint16(keyVersion.Int16), ciphertext, nonce)
	if err != nil {
		return adapter.Credentials{}, err
	}
	return adapter.Credentials{APIKey: creds.APIKey, APISecret: creds.APISecret, Passphrase: creds.Passphrase, Futures: &adapter.KeyPair{APIKey: creds.FuturesAPIKey, APISecret: creds.FuturesAPISecret}}, nil
}
