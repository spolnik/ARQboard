package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spolnik/arqboard/internal/db"
)

type Readiness interface {
	Ready(context.Context) error
}

type Options struct {
	Readiness  Readiness
	BoardStore BoardStore
	StaticFS   fs.FS
	Logger     *slog.Logger
}

type BoardStore interface {
	GetDefaultBoard(context.Context) (db.Board, error)
	CreateCard(context.Context, db.CreateCardParams) (db.BoardCard, error)
	MoveCard(context.Context, db.MoveCardParams) (db.Board, error)
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
		r.Get("/boards/default", defaultBoard(opts.BoardStore))
		r.Post("/cards", createCard(opts.BoardStore))
		r.Patch("/cards/{cardID}/move", moveCard(opts.BoardStore))
		r.Post("/cards/{cardID}/move", moveCard(opts.BoardStore))
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

type createCardRequest struct {
	ColumnID      string `json:"columnId"`
	Title         string `json:"title"`
	OwnerInitials string `json:"ownerInitials"`
}

type moveCardRequest struct {
	ColumnID string `json:"columnId"`
	Position int    `json:"position"`
}

func defaultBoard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		board, err := store.GetDefaultBoard(r.Context())
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, board)
	}
}

func createCard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload createCardRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.ColumnID) == "" || strings.TrimSpace(payload.Title) == "" {
			writeError(w, http.StatusBadRequest, "invalid_card", "columnId and title are required")
			return
		}

		card, err := store.CreateCard(r.Context(), db.CreateCardParams{
			ColumnID:      payload.ColumnID,
			Title:         payload.Title,
			OwnerInitials: payload.OwnerInitials,
		})
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, card)
	}
}

func moveCard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload moveCardRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.ColumnID) == "" {
			writeError(w, http.StatusBadRequest, "invalid_move", "columnId is required")
			return
		}

		board, err := store.MoveCard(r.Context(), db.MoveCardParams{
			CardID:   chi.URLParam(r, "cardID"),
			ColumnID: payload.ColumnID,
			Position: payload.Position,
		})
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, board)
	}
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

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrValidation):
		writeError(w, http.StatusBadRequest, "invalid_request", "request validation failed")
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, db.ErrDatabaseUnavailable):
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "request failed")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
