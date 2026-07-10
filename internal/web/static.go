package web

import (
	"embed"
	"net/http"
)

//go:embed static/favicon.svg static/favicon.ico static/apple-touch-icon.png
var staticFS embed.FS

func serveStatic(path, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(data)
	}
}
