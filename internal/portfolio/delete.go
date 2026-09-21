package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

var identName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func (s *Service) deleteCascade(ctx context.Context, userID, portfolioID []byte, version uint64) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return fmt.Errorf("disable foreign key checks: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1") }()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	tables, err := listPortfolioChildTables(ctx, tx)
	if err != nil {
		return err
	}
	for _, table := range tables {
		query, err := deleteByPortfolioQuery(table)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, query, portfolioID); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}
	n, err := sqlcgen.New(tx).DeletePortfolio(ctx, sqlcgen.DeletePortfolioParams{
		ID:      portfolioID,
		UserID:  userID,
		Version: version,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPrecondition
	}
	return tx.Commit()
}

func listPortfolioChildTables(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT TABLE_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND COLUMN_NAME = 'portfolio_id'
		  AND TABLE_NAME <> 'portfolios'
		ORDER BY TABLE_NAME
	`)
	if err != nil {
		return nil, fmt.Errorf("list portfolio child tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("scan child table: %w", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list portfolio child tables: %w", err)
	}
	return tables, nil
}

func deleteByPortfolioQuery(table string) (string, error) {
	if !identName.MatchString(table) {
		return "", fmt.Errorf("invalid table name %q", table)
	}
	return "DELETE FROM `" + table + "` WHERE portfolio_id = ?", nil
}
