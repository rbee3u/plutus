package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

type fieldsKey struct{}

// WithFields adds operational correlation fields without importing domain types.
// Later fields replace earlier fields with the same key.
func WithFields(ctx context.Context, attrs ...slog.Attr) context.Context {
	previous, _ := ctx.Value(fieldsKey{}).([]slog.Attr)
	fields := append([]slog.Attr(nil), previous...)
	for _, attr := range attrs {
		replaced := false
		for i := range fields {
			if fields[i].Key == attr.Key {
				fields[i] = attr
				replaced = true
				break
			}
		}
		if !replaced {
			fields = append(fields, attr)
		}
	}
	return context.WithValue(ctx, fieldsKey{}, fields)
}

// BindContext snapshots correlation fields onto a logger for operations that
// outlive the request context, such as a driver's automatic transaction rollback.
func BindContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	fields, _ := ctx.Value(fieldsKey{}).([]slog.Attr)
	attrs := make([]any, len(fields))
	for i, field := range fields {
		attrs[i] = field
	}
	return logger.With(attrs...)
}

type contextHandler struct{ slog.Handler }

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	fields, _ := ctx.Value(fieldsKey{}).([]slog.Attr)
	if len(fields) > 0 {
		record = record.Clone()
	}
	for _, field := range fields {
		exists := false
		record.Attrs(func(attr slog.Attr) bool {
			if attr.Key == field.Key {
				exists = true
			}
			return !exists
		})
		if !exists {
			record.AddAttrs(field)
		}
	}
	return h.Handler.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}

// Writer bridges framework writes into a logger. Each write is one event;
// TextHandler quotes embedded newlines, so a framework dump stays on one line.
func Writer(logger *slog.Logger, level slog.Level) io.Writer {
	return &logWriter{logger: logger, level: level}
}

type logWriter struct {
	logger *slog.Logger
	level  slog.Level
}

func (w *logWriter) Write(p []byte) (int, error) {
	if message := strings.TrimSpace(string(p)); message != "" {
		w.logger.Log(context.Background(), w.level, message)
	}
	return len(p), nil
}
