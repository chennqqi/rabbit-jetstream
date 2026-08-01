package adminui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var assets embed.FS

// Handler serves immutable embedded console assets and falls back to the app
// shell for client-side routes below /admin/.
func Handler() http.Handler {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/admin/")
		if path == "" {
			path = "index.html"
		}
		if strings.Contains(path, "..") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(root, path); err != nil {
			path = "index.html"
		}
		clone := r.Clone(r.Context())
		servePath := path
		if path == "index.html" {
			servePath = ""
		}
		clone.URL.Path = "/" + servePath
		if path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, clone)
	})
}
