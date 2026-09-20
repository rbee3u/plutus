package mysql

import (
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// IsDuplicate reports a unique-constraint violation.
func IsDuplicate(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
