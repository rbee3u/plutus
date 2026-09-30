package kraken

import (
	"os"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters/offline"
)

func TestMain(m *testing.M) { offline.Install(); os.Exit(m.Run()) }
