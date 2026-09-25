// Package fixture is the on-disk format of recorded Platform responses (ADR 0023).
//
// `plutus adapter record` writes one File per endpoint and scenario into gitignored tmp/; after
// the owner reviews them they move to internal/adapters/<platform>/testdata/, where the adapter's
// httptest replay reads them back. The header comes first so a reviewer sees provenance at once.
package fixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Header names what a fixture is and how it was altered from the raw response. A recorded
// fixture carries its recording date; a documentation-derived scenario (ADR 0027) carries no date
// and names the documentation it was built from instead.
type Header struct {
	Platform         string     `json:"platform"`
	Endpoint         string     `json:"endpoint"`
	AccountStructure string     `json:"accountStructure"`
	MarginMode       string     `json:"marginMode,omitempty"`
	Scenario         string     `json:"scenario"`
	RecordedAt       *time.Time `json:"recordedAt,omitempty"`
	Documentation    string     `json:"documentation,omitempty"`
	Trimmed          *string    `json:"trimmed"`
	Redacted         []string   `json:"redacted"`
}

const (
	// ScenarioRecorded marks a successful answer recorded from a live account. A recorded refusal
	// is named by its error scenario instead, such as "invalid_key".
	ScenarioRecorded = "recorded"
	// ScenarioDocumented marks a response built from the Platform's documentation rather than
	// recorded (ADR 0027). It proves the mapping of the documented shape, not live access.
	ScenarioDocumented = "documented"
)

// Documented reports whether the fixture is a documentation-derived scenario.
func (h *Header) Documented() bool { return h.Scenario == ScenarioDocumented }

// Recording is one recorded exchange handed over by an adapter's fixture recorder. It is produced
// only by `plutus adapter record`; at runtime a raw Platform body never leaves its adapter package.
type Recording struct {
	Name             string
	Endpoint         string
	AccountStructure string
	MarginMode       string
	Scenario         string
	Status           int
	Trimmed          *string
	Redacted         []string
	Body             json.RawMessage
}

// Name is the file stem of one recorded request. The path loses its slashes and dashes, each
// query value is appended, and a scenario comes last: "GET /api/v5/public/instruments" with
// "instType=SWAP" becomes "api_v5_public_instruments.SWAP", and "/papi/v1/um/positionRisk"
// becomes "papi_v1_um_positionRisk".
func Name(request, scenario string) string {
	path, query, _ := strings.Cut(request, "?")
	parts := []string{strings.NewReplacer("/", "_", "-", "_").Replace(strings.TrimPrefix(path, "/"))}
	if query != "" {
		for _, param := range strings.Split(query, "&") {
			if _, value, _ := strings.Cut(param, "="); value != "" {
				parts = append(parts, value)
			}
		}
	}
	if scenario != "" {
		parts = append(parts, scenario)
	}
	return strings.Join(parts, ".")
}

// File is one recorded HTTP exchange.
type File struct {
	Fixture Header          `json:"fixture"`
	Status  int             `json:"status"`
	Body    json.RawMessage `json:"body"`
}

// Write stores f as indented JSON at dir/name.json and returns the path.
func Write(dir, name string, f *File) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	if f.Fixture.Redacted == nil {
		f.Fixture.Redacted = []string{}
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode fixture %s: %w", name, err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// Read loads one fixture file.
func Read(path string) (File, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // test fixtures and recorder output are local paths chosen by the caller
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return File{}, fmt.Errorf("decode fixture %s: %w", path, err)
	}
	return f, nil
}
