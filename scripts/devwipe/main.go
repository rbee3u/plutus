// Command devwipe drops every table in the local plutus database.
// It is a temporary early-development helper, not a product CLI.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

const allowedDatabase = "plutus"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "devwipe: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return err
	}
	if cfg.Database.DSN == "" {
		return errors.New("database DSN is required in plutus.toml or PLUTUS_DATABASE_DSN / PLUTUS_DATABASE_DSN_FILE")
	}
	db, err := mysql.Open(cfg.Database.DSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	return wipe(context.Background(), db)
}

func wipe(ctx context.Context, db *sql.DB) error {
	var name sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return fmt.Errorf("resolve database name: %w", err)
	}
	if !name.Valid || name.String == "" {
		return errors.New("connection has no default database")
	}
	if name.String != allowedDatabase {
		return fmt.Errorf("refusing to wipe %q; this temporary helper only targets %q", name.String, allowedDatabase)
	}

	tables, err := listTables(ctx, db)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "dropping %d table(s) in %s\n", len(tables), name.String)
	if _, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return fmt.Errorf("disable foreign key checks: %w", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1") }()
	for _, table := range tables {
		ident, err := quoteIdent(table)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+ident); err != nil {
			return fmt.Errorf("drop table %s: %w", table, err)
		}
	}
	return nil
}

func listTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT TABLE_NAME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_TYPE = 'BASE TABLE'
	`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	return tables, nil
}

func quoteIdent(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, '`') {
		return "", fmt.Errorf("invalid table name %q", name)
	}
	return "`" + name + "`", nil
}
