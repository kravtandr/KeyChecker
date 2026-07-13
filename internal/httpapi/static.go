package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:webdist
var webDist embed.FS

// StaticHandler отдаёт собранный фронт с SPA-fallback на index.html.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(webDist, "webdist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(sub, trimSlash(r.URL.Path)); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func trimSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return "index.html"
	}
	return p
}
