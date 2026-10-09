package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/pressly/goose/v3"

	"github.com/rbee3u/plutus/db/migrations"
)

// ExpectedSchemaVersion is the latest Goose migration this build requires.
const ExpectedSchemaVersion int64 = 20261009020000

func prepareGoose() error {
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	return nil
}

// migrationLockName is the MySQL advisory lock that serializes concurrent Up callers.
const migrationLockName = "plutus_migrate"

// Up applies all pending migrations. Concurrent callers sharing one database (for example
// test packages) serialize on a MySQL advisory lock so the Goose version table is created once.
func Up(db *sql.DB) error {
	if err := prepareGoose(); err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	defer func() { _ = conn.Close() }()
	var locked sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 120)", migrationLockName).Scan(&locked); err != nil {
		return fmt.Errorf("migrate up: acquire lock: %w", err)
	}
	if !locked.Valid || locked.Int64 != 1 {
		return errors.New("migrate up: the migration lock is held by another process")
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", migrationLockName) }()
	if err := guardCatalogMigration(ctx, db); err != nil {
		return err
	}
	if err := guardDualMigration(ctx, db); err != nil {
		return err
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Status writes migration status to w.
func Status(db *sql.DB, w io.Writer) error {
	if err := prepareGoose(); err != nil {
		return err
	}
	goose.SetLogger(&writerLogger{w: w})
	if err := goose.Status(db, "."); err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}
	return nil
}

// CurrentVersion returns the applied Goose version.
func CurrentVersion(db *sql.DB) (int64, error) {
	if err := prepareGoose(); err != nil {
		return 0, err
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	return version, nil
}

type writerLogger struct {
	w io.Writer
}

// SetMigrationLogger routes operational migration logs for the current process.
// Status deliberately replaces this logger with its command-result writer.
func SetMigrationLogger(logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	goose.SetLogger(&migrationLogger{logger: logger.With("source", "goose")})
}

type migrationLogger struct{ logger *slog.Logger }

func (l *migrationLogger) Printf(format string, values ...any) {
	l.logger.Info(fmt.Sprintf(format, values...))
}

func (l *migrationLogger) Fatalf(format string, values ...any) {
	l.logger.Error(fmt.Sprintf(format, values...))
}

func (l *writerLogger) Fatalf(format string, v ...any) {
	l.Printf(format, v...)
}

func (l *writerLogger) Printf(format string, v ...any) {
	message := fmt.Sprintf(format, v...)
	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}
	_, _ = io.WriteString(l.w, message)
}

// A stopped worker may leave a partially collected cycle. It must be drained
// before the split so no cycle mixes legacy and split report scopes.
func guardDualMigration(ctx context.Context, db *sql.DB) error {
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return err
	}
	if version < 20260922120000 || version >= 20260929010100 {
		return nil
	}
	var active int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs j WHERE j.status = 'running' OR (j.status = 'queued' AND EXISTS (SELECT 1 FROM account_reports r WHERE r.job_id=j.id))`).Scan(&active)
	if err != nil {
		return err
	}
	if active > 0 {
		return errors.New("migrate up: drain running or partially collected cycles with the previous release, stop all workers, then retry the Dual Investment migration")
	}
	return nil
}

// Reject unsupported catalog history before any pending DDL changes its shape.
func guardCatalogMigration(ctx context.Context, db *sql.DB) error {
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return err
	}
	if version < 20260922120000 || version >= 20261009010000 {
		return nil
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM instruments WHERE kind = 'spot'").Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("migrate up: catalog contains spot instruments; resolve unsupported records before the Futures Contract migration")
	}
	return nil
}
