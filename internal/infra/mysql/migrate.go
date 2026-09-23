package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/pressly/goose/v3"

	"github.com/rbee3u/plutus/db/migrations"
)

// ExpectedSchemaVersion is the latest Goose migration this build requires.
const ExpectedSchemaVersion int64 = 20260922120000

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

func (l *writerLogger) Fatalf(format string, v ...any) {
	_, _ = fmt.Fprintf(l.w, format, v...)
}

func (l *writerLogger) Printf(format string, v ...any) {
	_, _ = fmt.Fprintf(l.w, format, v...)
}
