package valuation

import (
	"testing"

	"github.com/rbee3u/plutus/internal/adapters/offline"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func TestMain(m *testing.M) { offline.Install(); mysql.RunTestMain(m) }
