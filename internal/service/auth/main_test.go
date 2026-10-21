package auth

import (
	"testing"

	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

// TestMain creates one throwaway plutus_gotest_* database for this package (see mysqltest.RunTestMain).
func TestMain(m *testing.M) {
	mysqltest.RunTestMain(m)
}
