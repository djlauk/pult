package main

import (
	"net/http"

	"pult/internal/brand"
)

var faviconICO = brand.ICO()

// handleFavicon serves the tray icon as the web UI's favicon, so both
// always look the same.
func handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(faviconICO)
}
