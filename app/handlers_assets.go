package app

import (
	"net/http"
)

func serveFavicon(w http.ResponseWriter, r *http.Request) {
	icon, err := assetsFS.ReadFile("static/favicon.svg")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(icon)
}

func serveFaviconRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/favicon.svg", http.StatusFound)
}
