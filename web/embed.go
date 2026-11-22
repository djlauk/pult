// Package web holds the built Svelte frontend.
package web

import "embed"

// Dist contains the output of `npm run build` (web/dist).
//
//go:embed all:dist
var Dist embed.FS
