package mysql_test

import (
	"testing"

	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/testsupport/offline"
)

func TestMain(m *testing.M) {
	offline.Install()
	mysqltest.RunTestMain(m)
}
