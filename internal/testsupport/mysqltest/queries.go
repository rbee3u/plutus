package mysqltest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// QueryStats counts database reads and consumed rows, including prepared reads.
// Use a separate pool so fixture setup and concurrent tests cannot affect it.
type QueryStats struct {
	Queries atomic.Int64
	Writes  atomic.Int64
	Rows    atomic.Int64
}

// ObserveQueries opens an instrumented pool on the package's throwaway database.
func ObserveQueries(tb testing.TB) (*sql.DB, *QueryStats) {
	tb.Helper()
	cfg, err := mysqldriver.ParseDSN(TestDSN(tb))
	if err != nil {
		tb.Fatal(err)
	}
	cfg.ParseTime, cfg.Loc = true, time.UTC
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["time_zone"] = "'+00:00'"
	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		tb.Fatal(err)
	}
	stats := &QueryStats{}
	db := sql.OpenDB(&queryConnector{Connector: connector, stats: stats})
	tb.Cleanup(func() { _ = db.Close() })
	return db, stats
}

type queryConnector struct {
	driver.Connector
	stats *QueryStats
}

func (c *queryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	observed, ok := conn.(observedConnection)
	if !ok {
		_ = conn.Close()
		return nil, errors.New("MySQL driver lacks context interfaces")
	}
	return &queryConn{observedConnection: observed, stats: c.stats}, nil
}

type observedConnection interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ExecerContext
	driver.QueryerContext
	driver.ConnPrepareContext
}

type queryConn struct {
	observedConnection
	stats *QueryStats
}

func (c *queryConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.observedConnection.BeginTx(ctx, opts)
}

func (c *queryConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.observedConnection.ExecContext(ctx, query, args)
	if err == nil {
		c.stats.Writes.Add(1)
	}
	return result, err
}

func (c *queryConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.observedConnection.QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	c.stats.Queries.Add(1)
	return &queryRows{Rows: rows, stats: c.stats}, nil
}

func (c *queryConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *queryConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	stmt, err := c.observedConnection.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	observed, ok := stmt.(observedStatement)
	if !ok {
		_ = stmt.Close()
		return nil, errors.New("MySQL statement lacks context interfaces")
	}
	return &queryStmt{observedStatement: observed, stats: c.stats}, nil
}

type observedStatement interface {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext
}

type queryStmt struct {
	observedStatement
	stats *QueryStats
}

func (s *queryStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	result, err := s.observedStatement.ExecContext(ctx, args)
	if err == nil {
		s.stats.Writes.Add(1)
	}
	return result, err
}

func (s *queryStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := s.observedStatement.QueryContext(ctx, args)
	if err != nil {
		return nil, err
	}
	s.stats.Queries.Add(1)
	return &queryRows{Rows: rows, stats: s.stats}, nil
}

type queryRows struct {
	driver.Rows
	stats *QueryStats
}

func (r *queryRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if err == nil {
		r.stats.Rows.Add(1)
	}
	return err
}
