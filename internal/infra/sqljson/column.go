package sqljson

import (
	"database/sql/driver"
	"fmt"
)

// Column stores a MySQL JSON value and accepts SQL NULL.
type Column []byte

// Scan implements sql.Scanner.
func (c *Column) Scan(src any) error {
	if src == nil {
		*c = nil
		return nil
	}
	switch value := src.(type) {
	case []byte:
		*c = append([]byte(nil), value...)
	case string:
		*c = []byte(value)
	default:
		return fmt.Errorf("unsupported JSON column type %T", src)
	}
	return nil
}

// Value implements driver.Valuer.
func (c *Column) Value() (driver.Value, error) {
	if c == nil || *c == nil {
		return nil, nil //nolint:nilnil // SQL NULL is represented as a nil driver value.
	}
	return append([]byte(nil), *c...), nil
}
