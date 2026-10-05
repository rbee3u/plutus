package valuation

import (
	"testing"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/platform/offline"
)

func TestMain(m *testing.M) { offline.Install(); mysql.RunTestMain(m) }
