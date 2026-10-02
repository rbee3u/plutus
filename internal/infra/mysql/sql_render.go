package mysql

import (
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// renderSQL is diagnostic only. Prepared execution sends parameters separately;
// this representation must never be used to execute a query.
func renderSQL(query string, args []driver.NamedValue) (string, error) {
	if len(args) == 0 {
		return query, nil
	}
	positions, err := placeholders(query, false)
	if err != nil {
		return "", err
	}
	other, otherErr := placeholders(query, true)
	if otherErr != nil || !slices.Equal(positions, other) {
		return "", errors.New("ambiguous placeholders under MySQL SQL modes")
	}
	if len(positions) != len(args) {
		return "", errors.New("placeholder and parameter counts differ")
	}
	var out strings.Builder
	previous := 0
	for i, position := range positions {
		literal, err := sqlLiteral(args[i].Value)
		if err != nil {
			return "", err
		}
		out.WriteString(query[previous:position])
		out.WriteString(literal)
		previous = position + 1
	}
	out.WriteString(query[previous:])
	return out.String(), nil
}

func placeholders(query string, backslashEscapes bool) ([]int, error) {
	var positions []int
	for i := 0; i < len(query); i++ {
		switch query[i] {
		case '?':
			positions = append(positions, i)
		case '\'', '"', '`':
			quote := query[i]
			closed := false
			for i++; i < len(query); i++ {
				if query[i] == '\\' && backslashEscapes && quote != '`' {
					i++
					continue
				}
				if query[i] != quote {
					continue
				}
				if i+1 < len(query) && query[i+1] == quote {
					i++
					continue
				}
				closed = true
				break
			}
			if !closed {
				return nil, errors.New("unterminated SQL quote")
			}
		case '#':
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case '-':
			if i+1 < len(query) && query[i+1] == '-' && (i+2 == len(query) || query[i+2] <= ' ') {
				for i < len(query) && query[i] != '\n' {
					i++
				}
			}
		case '/':
			if i+1 < len(query) && query[i+1] == '*' {
				end := strings.Index(query[i+2:], "*/")
				if end < 0 {
					return nil, errors.New("unterminated SQL comment")
				}
				if i+2 < len(query) && query[i+2] == '!' && strings.Contains(query[i+2:i+2+end], "?") {
					return nil, errors.New("placeholder in executable SQL comment")
				}
				i += end + 3
			}
		}
	}
	return positions, nil
}

func sqlLiteral(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "NULL", nil
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", errors.New("nonfinite SQL number")
		}
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case string:
		return sqlString(v), nil
	case json.RawMessage:
		if v == nil {
			return "NULL", nil
		}
		return sqlString(string(v)), nil
	case []byte:
		if v == nil {
			return "NULL", nil
		}
		return "X'" + hex.EncodeToString(v) + "'", nil
	case time.Time:
		if v.IsZero() {
			return "'0000-00-00'", nil
		}
		return "'" + v.UTC().Format("2006-01-02 15:04:05.999999999") + "'", nil
	default:
		return "", fmt.Errorf("unsupported SQL parameter type %T", value)
	}
}

func sqlString(value string) string {
	// Hex conversion avoids relying on the connection's NO_BACKSLASH_ESCAPES
	// status, which the MySQL driver intentionally keeps private.
	if !utf8.ValidString(value) || strings.ContainsAny(value, "\\\x00\n\r\x1a") {
		return "CONVERT(X'" + hex.EncodeToString([]byte(value)) + "' USING utf8mb4)"
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
