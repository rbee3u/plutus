package seeds

import "embed"

const (
	// ProfileMinimal is the complete happy-path source-fact profile.
	ProfileMinimal = "minimal"
	// ProfileEdgeCases is the incomplete and overlapping-coverage profile.
	ProfileEdgeCases = "edge-cases"
)

//go:embed minimal.sql
var files embed.FS

// MinimalSQL returns the deterministic happy-path source-fact script.
func MinimalSQL() (string, error) {
	raw, err := files.ReadFile("minimal.sql")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
