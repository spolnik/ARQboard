package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Readiness interface {
	Ready(context.Context) error
}

type Options struct {
	Readiness Readiness
	StaticFS  fs.FS
	Logger    *slog.Logger
}

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(opts Options) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	if opts.Logger != nil {
		r.Use(requestLogger(opts.Logger))
	}

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", readyz(opts.Readiness))

	r.Route("/api", func(r chi.Router) {
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		})
	})

	if opts.StaticFS != nil {
		r.NotFound(staticHandler(opts.StaticFS))
	} else {
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
		})
	}

	return r
}

func readyz(readiness Readiness) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if readiness == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := readiness.Ready(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "service is not ready")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func staticHandler(staticFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if exists(staticFS, path) {
			serveStaticFile(staticFS, w, r, path)
			return
		}

		if exists(staticFS, "index.html") {
			serveStaticFile(staticFS, w, r, "index.html")
			return
		}

		writeError(w, http.StatusNotFound, "not_found", "static app not found")
	}
}

func exists(staticFS fs.FS, path string) bool {
	info, err := fs.Stat(staticFS, path)
	return err == nil && !info.IsDir()
}

func serveStaticFile(staticFS fs.FS, w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(staticFS, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "static asset not found")
		return
	}

	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)

	info, err := fs.Stat(staticFS, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "static asset not found")
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(data))
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorBody{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
