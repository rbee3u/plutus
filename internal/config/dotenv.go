package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const envFileName = ".env"

const goModFileName = "go.mod"

// Lookup reports a process environment value, falling back to `.env` in the working directory
// or, when tests or processes run from a subdirectory, the Go module root.
// Existing process environment values win. A missing `.env` is ignored. The nearest `.env` wins.
func Lookup(key string) (string, error) {
	return lookup(key, Options{})
}

func lookup(key string, opts Options) (string, error) {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	if err := applyDotEnv(&opts); err != nil {
		return "", err
	}
	value, _ := opts.LookupEnv(key)
	return value, nil
}

func applyDotEnv(opts *Options) error {
	raw, err := readNearestDotEnv(opts)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}
	values, err := parseDotEnv(string(raw))
	if err != nil {
		return fmt.Errorf("parse %s: %w", envFileName, err)
	}
	if len(values) == 0 {
		return nil
	}
	base := opts.LookupEnv
	opts.LookupEnv = func(key string) (string, bool) {
		if value, ok := base(key); ok {
			return value, true
		}
		value, ok := values[key]
		return value, ok
	}
	return nil
}

func readNearestDotEnv(opts *Options) ([]byte, error) {
	for _, path := range dotEnvPaths(opts) {
		raw, err := opts.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", envFileName, err)
		}
		return raw, nil
	}
	return nil, nil
}

func dotEnvPaths(opts *Options) []string {
	paths := []string{envFileName}
	getwd := os.Getwd
	if opts.getwd != nil {
		getwd = opts.getwd
	}
	dir, err := getwd()
	if err != nil || dir == "" {
		return paths
	}
	var chain []string
	for {
		chain = append(chain, dir)
		if _, err := opts.ReadFile(filepath.Join(dir, goModFileName)); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return paths
		}
		dir = parent
	}
	seen := map[string]struct{}{envFileName: {}}
	for _, d := range chain {
		path := filepath.Join(d, envFileName)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func parseDotEnv(src string) (map[string]string, error) {
	out := make(map[string]string)
	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "export ")
		trimmed = strings.TrimSpace(trimmed)
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", i+1)
		}
		key = strings.TrimSpace(key)
		if !validDotEnvKey(key) {
			return nil, fmt.Errorf("line %d: invalid variable name", i+1)
		}
		parsed, err := unquoteDotEnvValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		out[key] = parsed
	}
	return out, nil
}

func validDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 && unicode.IsDigit(r) {
			return false
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func unquoteDotEnvValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}
	switch value[0] {
	case '"':
		if value[len(value)-1] != '"' {
			return "", errors.New("unclosed double quote")
		}
		return unescapeDoubleQuoted(value)
	case '\'':
		if value[len(value)-1] != '\'' {
			return "", errors.New("unclosed single quote")
		}
		return value[1 : len(value)-1], nil
	default:
		return value, nil
	}
}

func unescapeDoubleQuoted(value string) (string, error) {
	var b strings.Builder
	b.Grow(len(value) - 2)
	escaped := false
	for _, r := range value[1 : len(value)-1] {
		if escaped {
			switch r {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '"', '\\':
				b.WriteRune(r)
			default:
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		return "", errors.New("trailing backslash")
	}
	return b.String(), nil
}
