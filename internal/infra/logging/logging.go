// Package logging owns process log sinks without depending on application frameworks.
package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Options is the complete policy for one process's log directory.
type Options struct {
	Directory  string
	Level      slog.Level
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	SQLEnabled bool
}

// System owns independent application, access, and optional SQL loggers.
type System struct {
	App    *slog.Logger
	Access *slog.Logger
	SQL    *slog.Logger
	sinks  []*sink
}

// New opens all enabled files before returning. The caller must close the system
// after stopping all producers. A directory may be owned by only one process.
func New(opts Options, stderr io.Writer) (*System, error) {
	if opts.Directory == "" || opts.MaxSizeMB <= 0 || opts.MaxSizeMB > int(^uint(0)>>1)/(1024*1024) || opts.MaxBackups < 0 || opts.MaxAgeDays < 0 || int64(opts.MaxAgeDays) > int64(^uint64(0)>>1)/int64(24*time.Hour) {
		return nil, errors.New("invalid logging directory or rotation policy")
	}
	if stderr == nil {
		return nil, errors.New("logging requires a stderr writer")
	}
	if err := os.MkdirAll(opts.Directory, 0o750); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	system := &System{}
	console := &lockedWriter{writer: stderr}
	for _, name := range []string{"app", "access", "sql"} {
		if name == "sql" && !opts.SQLEnabled {
			continue
		}
		path := filepath.Join(opts.Directory, name+".log")
		if err := checkFile(path); err != nil {
			_ = system.Close()
			return nil, err
		}
		file := &lumberjack.Logger{Filename: path, MaxSize: opts.MaxSizeMB, MaxBackups: opts.MaxBackups, MaxAge: opts.MaxAgeDays, Compress: opts.Compress}
		// Lumberjack opens lazily. Force opening now, so startup fails reliably.
		if _, err := file.Write(nil); err != nil {
			_ = file.Close()
			_ = system.Close()
			return nil, fmt.Errorf("open %s log: %w", name, err)
		}
		output := &sink{file: file, console: console, mirror: name == "app", name: name}
		system.sinks = append(system.sinks, output)
		level := slog.LevelInfo
		if name == "app" {
			level = opts.Level
		}
		logger := slog.New(&contextHandler{Handler: slog.NewTextHandler(output, &slog.HandlerOptions{Level: level})})
		switch name {
		case "app":
			system.App = logger
		case "access":
			system.Access = logger
		case "sql":
			system.SQL = logger
		}
	}
	return system, nil
}

func checkFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil {
		return statErr
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("log file %s is not a regular file", path)
	}
	return closeErr
}

// Close closes active files. Lumberjack's backup maintenance is asynchronous;
// short commands do not wait for compression and subsequent runs resume it.
func (s *System) Close() error {
	var errs []error
	for _, output := range s.sinks {
		if err := output.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

type sink struct {
	mu      sync.Mutex
	file    io.WriteCloser
	console io.Writer
	mirror  bool
	name    string
	closed  bool
}

func (s *sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	if s.closed {
		err = os.ErrClosed
	} else {
		var n int
		n, err = s.file.Write(p)
		if err == nil && n != len(p) {
			err = io.ErrShortWrite
		}
	}
	if err != nil {
		// Do not log through a logger here: that would recurse into this sink.
		_, _ = fmt.Fprintf(s.console, "log file write failed: channel=%s error=%q\n", s.name, err)
	}
	if s.mirror || err != nil {
		if _, consoleErr := s.console.Write(p); consoleErr != nil {
			return 0, consoleErr
		}
	}
	return len(p), nil
}

func (s *sink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.file.Close()
}
