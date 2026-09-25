package httpserver

import (
	"testing"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

// TestMain creates one throwaway plutus_gotest_* database for this package (see mysql.RunTestMain).
func TestMain(m *testing.M) {
	mysql.RunTestMain(m)
}
