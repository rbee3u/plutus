package web

import "embed"

// Dist is the production Vite build. `make web-build` refreshes it.
//
//go:embed all:dist
var Dist embed.FS
