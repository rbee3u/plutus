package mysql

import (
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// IsDeadlock reports a transaction chosen as a deadlock victim. Callers may
// retry the whole transaction after rolling it back.
func IsDeadlock(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1213
}
