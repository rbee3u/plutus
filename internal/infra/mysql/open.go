// Package mysql opens MySQL connections for Plutus.
package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// Loggers routes driver diagnostics and optional SQL execution records.
type Loggers struct {
	App *slog.Logger
	SQL *slog.Logger
}

// Open returns a pool that uses UTC session time. Callers must close it.
// Omitting loggers preserves the offline test harness's original behavior.
func Open(dsn string, loggers ...Loggers) (*sql.DB, error) {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database DSN: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Collation = "utf8mb4_0900_ai_ci"
	var logs Loggers
	if len(loggers) > 0 {
		logs = loggers[0]
	}
	if logs.App != nil {
		cfg.Logger = &driverLogger{logger: logs.App.With("source", "mysql")}
	}

	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if logs.SQL != nil {
		connector = &loggingConnector{Connector: connector, logger: logs.SQL}
	}
	db := sql.OpenDB(&utcConnector{Connector: connector})
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

type utcConnector struct {
	driver.Connector
}

func (c *utcConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	execer, ok := conn.(driver.ExecerContext)
	if !ok {
		_ = conn.Close()
		return nil, errors.New("MySQL connection does not support ExecContext")
	}
	if _, err := execer.ExecContext(ctx, "SET time_zone = '+00:00'", nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set time_zone: %w", err)
	}
	return conn, nil
}

// Ping reports whether the database accepts connections.
func Ping(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	return nil
}
