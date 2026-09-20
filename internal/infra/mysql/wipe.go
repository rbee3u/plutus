package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var allowedDevDatabase = regexp.MustCompile(`^plutus(_[a-z0-9]+)?$`)

// ErrDevResetRefused means the current database is not a permitted reset target.
var ErrDevResetRefused = errors.New("development reset refused")

// CurrentDatabase returns the connection's default schema name.
func CurrentDatabase(ctx context.Context, db *sql.DB) (string, error) {
	var name sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", fmt.Errorf("resolve database name: %w", err)
	}
	if !name.Valid || name.String == "" {
		return "", fmt.Errorf("%w: connection has no default database", ErrDevResetRefused)
	}
	return name.String, nil
}

// GuardDevReset returns the current database name after verifying it is a local development target.
func GuardDevReset(ctx context.Context, db *sql.DB) (string, error) {
	name, err := CurrentDatabase(ctx, db)
	if err != nil {
		return "", err
	}
	if err := AllowDevReset(name); err != nil {
		return "", err
	}
	return name, nil
}

// AllowDevReset reports whether name is a permitted development database.
func AllowDevReset(name string) error {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "", "mysql", "sys", "information_schema", "performance_schema":
		return fmt.Errorf("%w: %q is a system schema", ErrDevResetRefused, name)
	}
	if strings.Contains(n, "prod") {
		return fmt.Errorf("%w: %q looks production-like", ErrDevResetRefused, name)
	}
	if !allowedDevDatabase.MatchString(n) {
		return fmt.Errorf("%w: %q is not an allowed development database name", ErrDevResetRefused, name)
	}
	return nil
}

// DropAllTables drops every base table in the current schema.
func DropAllTables(ctx context.Context, db *sql.DB) error {
	tables, err := listBaseTables(ctx, db)
	if err != nil {
		return err
	}
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

// ExecSQLStatements runs each SQL statement from src in order.
func ExecSQLStatements(ctx context.Context, db *sql.DB, src string) error {
	for i, stmt := range splitSQL(src) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("statement %d: %w", i+1, err)
		}
	}
	return nil
}

func listBaseTables(ctx context.Context, db *sql.DB) ([]string, error) {
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

func splitSQL(src string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(b.String()), ";"))
			if stmt != "" {
				out = append(out, stmt)
			}
			b.Reset()
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, strings.TrimSuffix(rest, ";"))
	}
	return out
}
