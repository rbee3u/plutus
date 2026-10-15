package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

var (
	// ErrDatabaseUnavailable means the process cannot ping MySQL.
	ErrDatabaseUnavailable = errors.New("database_unavailable")
	// ErrSchemaMismatch means applied Goose migrations are not the expected version.
	ErrSchemaMismatch = errors.New("schema_mismatch")
	// ErrMySQLIncompatible means version, sql_mode, charset, or timezone is wrong.
	ErrMySQLIncompatible = errors.New("mysql_incompatible")
	// ErrMasterKeyRequired means encrypted identity credentials exist without a configured master key.
	ErrMasterKeyRequired = errors.New("master_key_required")
)

// CheckReady pings MySQL, validates session settings, and checks the schema version.
func CheckReady(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrDatabaseUnavailable
	}
	if err := Ping(ctx, db); err != nil {
		return ErrDatabaseUnavailable
	}
	if err := CheckEnvironment(ctx, db); err != nil {
		return err
	}
	version, err := CurrentVersion(db)
	if err != nil {
		return ErrSchemaMismatch
	}
	if version != ExpectedSchemaVersion {
		return ErrSchemaMismatch
	}
	return nil
}

// CheckEnvironment enforces MySQL 8.4, strict mode, utf8mb4, and UTC.
func CheckEnvironment(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "SET time_zone = '+00:00'"); err != nil {
		return fmt.Errorf("%w: set time_zone: %w", ErrMySQLIncompatible, err)
	}

	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return fmt.Errorf("%w: version: %w", ErrMySQLIncompatible, err)
	}
	if !strings.HasPrefix(version, "8.4.") && !strings.HasPrefix(version, "8.4") {
		return fmt.Errorf("%w: require MySQL 8.4, got %s", ErrMySQLIncompatible, version)
	}

	var sqlMode string
	if err := db.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&sqlMode); err != nil {
		return fmt.Errorf("%w: sql_mode: %w", ErrMySQLIncompatible, err)
	}
	if !strings.Contains(sqlMode, "STRICT_TRANS_TABLES") && !strings.Contains(sqlMode, "STRICT_ALL_TABLES") {
		return fmt.Errorf("%w: strict sql_mode is required", ErrMySQLIncompatible)
	}

	var charset string
	if err := db.QueryRowContext(ctx, "SELECT @@character_set_connection").Scan(&charset); err != nil {
		return fmt.Errorf("%w: charset: %w", ErrMySQLIncompatible, err)
	}
	if !strings.HasPrefix(strings.ToLower(charset), "utf8mb4") {
		return fmt.Errorf("%w: utf8mb4 is required, got %s", ErrMySQLIncompatible, charset)
	}

	var tz string
	if err := db.QueryRowContext(ctx, "SELECT @@session.time_zone").Scan(&tz); err != nil {
		return fmt.Errorf("%w: time_zone: %w", ErrMySQLIncompatible, err)
	}
	if tz != "+00:00" && !strings.EqualFold(tz, "UTC") {
		return fmt.Errorf("%w: session time_zone must be UTC, got %s", ErrMySQLIncompatible, tz)
	}
	return nil
}

// CheckEncryptedCredentials requires a master key when any identity credential ciphertext exists.
func CheckEncryptedCredentials(ctx context.Context, db *sql.DB, hasMasterKey bool) error {
	if hasMasterKey {
		return nil
	}
	if db == nil {
		return ErrDatabaseUnavailable
	}
	n, err := sqlcgen.New(db).CountIdentitiesWithCiphertext(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrMasterKeyRequired
	}
	return nil
}
