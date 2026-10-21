package mysql

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/rbee3u/plutus/internal/infra/logging"
)

type loggingConnector struct {
	driver.Connector
	logger *slog.Logger
}

func (c *loggingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &loggingConn{Conn: conn, logger: c.logger}, nil
}

type loggingConn struct {
	driver.Conn
	logger *slog.Logger
}

func (c *loggingConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *loggingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	start := time.Now()
	var stmt driver.Stmt
	var err error
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		stmt, err = preparer.PrepareContext(ctx, query)
	} else {
		stmt, err = c.Conn.Prepare(query)
	}
	if err != nil {
		recordSQL(ctx, c.logger, "prepare", query, nil, start, err)
		return nil, err
	}
	return &loggingStmt{Stmt: stmt, query: query, logger: c.logger}, nil
}

func (c *loggingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	result, err := execer.ExecContext(ctx, query, args)
	if !errors.Is(err, driver.ErrSkip) {
		recordSQL(ctx, c.logger, "exec", query, args, start, err)
	}
	return result, err
}

func (c *loggingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	rows, err := queryer.QueryContext(ctx, query, args)
	if !errors.Is(err, driver.ErrSkip) {
		recordSQL(ctx, c.logger, "query", query, args, start, err)
	}
	return rows, err
}

func (c *loggingConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *loggingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	start := time.Now()
	var tx driver.Tx
	var err error
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		tx, err = beginner.BeginTx(ctx, opts)
	} else {
		if opts.Isolation != 0 || opts.ReadOnly {
			return nil, errors.New("driver does not support transaction options")
		}
		tx, err = c.Conn.Begin() //nolint:staticcheck // SA1019: preserve compatibility with legacy driver implementations.
	}
	recordSQL(ctx, c.logger, "begin", "BEGIN", nil, start, err)
	if err != nil {
		return nil, err
	}
	return &loggingTx{Tx: tx, logger: logging.BindContext(ctx, c.logger)}, nil
}

func (c *loggingConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *loggingConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *loggingConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *loggingConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type loggingStmt struct {
	driver.Stmt
	query  string
	logger *slog.Logger
}

func (s *loggingStmt) Exec(args []driver.Value) (driver.Result, error) {
	start := time.Now()
	result, err := s.Stmt.Exec(args) //nolint:staticcheck // SA1019: preserve compatibility with legacy driver implementations.
	recordSQL(context.Background(), s.logger, "exec", s.query, namedValues(args), start, err)
	return result, err
}

func (s *loggingStmt) Query(args []driver.Value) (driver.Rows, error) {
	start := time.Now()
	rows, err := s.Stmt.Query(args) //nolint:staticcheck // SA1019: preserve compatibility with legacy driver implementations.
	recordSQL(context.Background(), s.logger, "query", s.query, namedValues(args), start, err)
	return rows, err
}

func (s *loggingStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	start := time.Now()
	var result driver.Result
	var err error
	if execer, ok := s.Stmt.(driver.StmtExecContext); ok {
		result, err = execer.ExecContext(ctx, args)
	} else {
		values, conversionErr := positionalValues(args)
		if conversionErr != nil {
			return nil, conversionErr
		}
		result, err = s.Stmt.Exec(values) //nolint:staticcheck // SA1019: preserve compatibility with legacy driver implementations.
	}
	recordSQL(ctx, s.logger, "exec", s.query, args, start, err)
	return result, err
}

func (s *loggingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	start := time.Now()
	var rows driver.Rows
	var err error
	if queryer, ok := s.Stmt.(driver.StmtQueryContext); ok {
		rows, err = queryer.QueryContext(ctx, args)
	} else {
		values, conversionErr := positionalValues(args)
		if conversionErr != nil {
			return nil, conversionErr
		}
		rows, err = s.Stmt.Query(values) //nolint:staticcheck // SA1019: preserve compatibility with legacy driver implementations.
	}
	recordSQL(ctx, s.logger, "query", s.query, args, start, err)
	return rows, err
}

func (s *loggingStmt) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := s.Stmt.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func (s *loggingStmt) ColumnConverter(index int) driver.ValueConverter {
	if converter, ok := s.Stmt.(driver.ColumnConverter); ok { //nolint:staticcheck // SA1019: preserve compatibility with legacy driver implementations.
		return converter.ColumnConverter(index)
	}
	return driver.DefaultParameterConverter
}

type loggingTx struct {
	driver.Tx
	logger *slog.Logger
}

func (t *loggingTx) Commit() error {
	start := time.Now()
	err := t.Tx.Commit()
	recordSQL(context.Background(), t.logger, "commit", "COMMIT", nil, start, err)
	return err
}

func (t *loggingTx) Rollback() error {
	start := time.Now()
	err := t.Tx.Rollback()
	recordSQL(context.Background(), t.logger, "rollback", "ROLLBACK", nil, start, err)
	return err
}

func namedValues(values []driver.Value) []driver.NamedValue {
	args := make([]driver.NamedValue, len(values))
	for i, value := range values {
		args[i] = driver.NamedValue{Ordinal: i + 1, Value: value}
	}
	return args
}

func positionalValues(args []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		if arg.Name != "" {
			return nil, errors.New("driver does not support named parameters")
		}
		values[i] = arg.Value
	}
	return values, nil
}

func recordSQL(ctx context.Context, logger *slog.Logger, operation, query string, args []driver.NamedValue, start time.Time, err error) {
	duration := time.Since(start)
	rendered, renderErr := renderSQL(query, args)
	if renderErr != nil {
		rendered = query
	}
	attrs := []slog.Attr{slog.String("operation", operation), slog.String("sql", rendered), slog.Duration("duration", duration)}
	if renderErr != nil {
		attrs = append(attrs, slog.Any("args", args), slog.Any("render_error", renderErr))
	}
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelError
		attrs = append(attrs, slog.Any("err", err))
	}
	logger.LogAttrs(ctx, level, "SQL execution", attrs...)
}

type driverLogger struct{ logger *slog.Logger }

func (l *driverLogger) Print(values ...any) {
	l.logger.Error(fmt.Sprint(values...))
}
