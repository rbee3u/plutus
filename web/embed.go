package web

import "embed"

// Dist is the production Vite build. `npm --prefix web run build` refreshes it.
//
//go:embed all:dist
var Dist embed.FS
