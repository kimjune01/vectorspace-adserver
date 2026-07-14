// Package apidocs serves the OpenAPI spec and a self-contained docs viewer, so
// the running server exposes its own contract. Agents fetch /openapi.json (or
// .yaml) and generate a client; humans open /docs (Redoc, vendored — no CDN).
//
// openapi.yaml is canonical; regenerate openapi.json from it with
// `make openapi-json` after editing.
package apidocs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPIYAML []byte

//go:embed openapi.json
var openAPIJSON []byte

//go:embed docs.html
var docsHTML []byte

//go:embed redoc.standalone.js
var redocJS []byte

// Register wires the spec and docs viewer onto mux.
func Register(mux *http.ServeMux) {
	serve := func(contentType string, body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Write(body)
		}
	}
	mux.HandleFunc("/openapi.yaml", serve("application/yaml", openAPIYAML))
	mux.HandleFunc("/openapi.json", serve("application/json", openAPIJSON))
	mux.HandleFunc("/docs", serve("text/html; charset=utf-8", docsHTML))
	mux.HandleFunc("/docs/redoc.standalone.js", serve("application/javascript", redocJS))
}
