// Package web embeds the anthrogo browser UI and exposes it as an http.Handler.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

// Handler returns an http.Handler serving the SPA and its static assets.
//
// Routes:
//
//	GET /           → index.html
//	GET /app.js     → app.js
//	GET /styles.css → styles.css
//	anything else   → 404
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
