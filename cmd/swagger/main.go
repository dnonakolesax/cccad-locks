package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Put swagger.json next to this file.
//
//go:embed swagger.json
var openAPIFiles embed.FS

const (
	defaultAddr    = ":8081"
	swaggerBaseURL = "/swagger"
	swaggerSpecURL = "/swagger/doc.json"
)

func main() {
	addr := os.Getenv("DOCS_ADDR")
	if strings.TrimSpace(addr) == "" {
		addr = defaultAddr
	}

	mux := http.NewServeMux()
	RegisterSwaggerDocs(mux)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Swagger UI: http://localhost%s/swagger/index.html", normalizeAddrForLog(addr))
	log.Printf("OpenAPI JSON: http://localhost%s%s", normalizeAddrForLog(addr), swaggerSpecURL)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// RegisterSwaggerDocs mounts Swagger UI and the OpenAPI JSON spec on a standard net/http mux.
//
// Routes:
//   - GET /swagger              -> redirects to /swagger/index.html
//   - GET /swagger/             -> Swagger UI assets and index
//   - GET /swagger/index.html   -> Swagger UI
//   - GET /swagger/doc.json     -> embedded OpenAPI 3.0 JSON
func RegisterSwaggerDocs(mux *http.ServeMux) {
	mux.HandleFunc(swaggerBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		http.Redirect(w, r, swaggerBaseURL+"/index.html", http.StatusFound)
	})

	mux.HandleFunc(swaggerSpecURL, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		spec, err := openAPIFiles.ReadFile("swagger.json")
		if err != nil {
			http.Error(w, "OpenAPI spec is not embedded", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(spec)
	})

	mux.Handle(swaggerBaseURL+"/", httpSwagger.Handler(
		httpSwagger.URL(swaggerSpecURL),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	))
}

func normalizeAddrForLog(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return strings.TrimPrefix(addr, "0.0.0.0")
	}
	return ":" + strings.TrimPrefix(addr, "localhost:")
}
