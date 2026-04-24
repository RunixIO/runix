package web

import (
	"net/http"

	"github.com/runixio/runix/internal/httputil"
)

// writeJSON is a package-level convenience wrapper around httputil.WriteJSON.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	httputil.WriteJSON(w, status, v)
}
