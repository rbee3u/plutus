package seeds

import "embed"

const (
	// ProfileMinimal is the complete happy-path source-fact profile.
	ProfileMinimal = "minimal"
	// ProfileEdgeCases is the incomplete and overlapping-coverage profile.
	ProfileEdgeCases = "edge-cases"
)

//go:embed minimal.sql edge-cases.sql
var files embed.FS

// MinimalSQL returns the deterministic happy-path source-fact script.
func MinimalSQL() (string, error) {
	return read("minimal.sql")
}

// EdgeCasesSQL returns the deterministic incomplete-coverage source-fact script.
func EdgeCasesSQL() (string, error) {
	return read("edge-cases.sql")
}

func read(name string) (string, error) {
	raw, err := files.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
