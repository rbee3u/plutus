package collection

import (
	"testing"

	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/testsupport/offline"
)

// TestMain creates one throwaway plutus_gotest_* database for this package (see mysqltest.RunTestMain).
func TestMain(m *testing.M) {
	offline.Install()
	mysqltest.RunTestMain(m)
}
