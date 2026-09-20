package mysql

import (
	"database/sql"
	"fmt"
	"io"

	"github.com/pressly/goose/v3"

	"github.com/rbee3u/plutus/db/migrations"
)

// ExpectedSchemaVersion is the latest Goose migration this build requires.
const ExpectedSchemaVersion int64 = 20260920120007

func prepareGoose() error {
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	return nil
}

// Up applies all pending migrations.
func Up(db *sql.DB) error {
	if err := prepareGoose(); err != nil {
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

func (l *writerLogger) Fatalf(format string, v ...any) {
	_, _ = fmt.Fprintf(l.w, format, v...)
}

func (l *writerLogger) Printf(format string, v ...any) {
	_, _ = fmt.Fprintf(l.w, format, v...)
}
