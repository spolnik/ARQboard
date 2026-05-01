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
	AuthStore  AuthStore
	TeamStore  TeamStore
	StaticFS   fs.FS
	Logger     *slog.Logger
}

type BoardStore interface {
	GetDefaultBoard(context.Context) (db.Board, error)
	ListWorkspaces(context.Context) ([]db.Workspace, error)
	ListBoards(context.Context) ([]db.BoardSummary, error)
	GetBoard(context.Context, string) (db.Board, error)
	CreateBoard(context.Context, db.CreateBoardParams) (db.Board, error)
	CreateColumn(context.Context, db.CreateColumnParams) (db.Board, error)
	UpdateColumn(context.Context, db.UpdateColumnParams) (db.Board, error)
	CreateCard(context.Context, db.CreateCardParams) (db.BoardCard, error)
	GetCardDetail(context.Context, string) (db.CardDetail, error)
	UpdateCard(context.Context, db.UpdateCardParams) (db.BoardCard, error)
	MoveCard(context.Context, db.MoveCardParams) (db.Board, error)
	CreateCardComment(context.Context, db.CreateCardCommentParams) (db.CardDetail, error)
	ListWikiPages(context.Context) ([]db.WikiPage, error)
	GetWikiPage(context.Context, string) (db.WikiPage, error)
	CreateWikiPage(context.Context, db.CreateWikiPageParams) (db.WikiPage, error)
	UpdateWikiPage(context.Context, db.UpdateWikiPageParams) (db.WikiPage, error)
	GetPlanningDashboard(context.Context, string) (db.PlanningDashboard, error)
	CreateSprint(context.Context, db.CreateSprintParams) (db.Sprint, error)
	StartSprint(context.Context, string) (db.Sprint, error)
	CompleteSprint(context.Context, db.CompleteSprintParams) (db.Sprint, error)
	AssignCardToSprint(context.Context, db.AssignCardToSprintParams) (db.BoardCard, error)
}

type AuthStore interface {
	Login(context.Context, db.LoginParams) (db.LoginSession, error)
	CurrentUser(context.Context, string) (db.User, error)
	Logout(context.Context, string) error
}

type TeamStore interface {
	ListWorkspaceMembers(context.Context) ([]db.WorkspaceMember, error)
	CreateWorkspaceMember(context.Context, db.CreateWorkspaceMemberParams) (db.WorkspaceMember, error)
	UpdateWorkspaceMember(context.Context, db.UpdateWorkspaceMemberParams) (db.WorkspaceMember, error)
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
			currentUser(opts.AuthStore)(w, r)
		})
		r.Post("/auth/login", login(opts.AuthStore))
		r.Post("/auth/logout", logout(opts.AuthStore))
		r.Group(func(r chi.Router) {
			if opts.AuthStore != nil {
				r.Use(requireAuth(opts.AuthStore))
			}
			r.Get("/workspaces", listWorkspaces(opts.BoardStore))
			r.Get("/boards", listBoards(opts.BoardStore))
			r.Post("/boards", createBoard(opts.BoardStore))
			r.Get("/boards/default", defaultBoard(opts.BoardStore))
			r.Get("/boards/{boardID}", boardByID(opts.BoardStore))
			r.Post("/boards/{boardID}/columns", createColumn(opts.BoardStore))
			r.Get("/planning", planningDashboard(opts.BoardStore))
			r.Post("/sprints", createSprint(opts.BoardStore))
			r.Post("/sprints/{sprintID}/start", startSprint(opts.BoardStore))
			r.Post("/sprints/{sprintID}/complete", completeSprint(opts.BoardStore))
			r.Patch("/columns/{columnID}", updateColumn(opts.BoardStore))
			r.Post("/cards", createCard(opts.BoardStore))
			r.Get("/cards/{cardID}", cardDetail(opts.BoardStore))
			r.Patch("/cards/{cardID}", updateCard(opts.BoardStore))
			r.Patch("/cards/{cardID}/sprint", assignCardToSprint(opts.BoardStore))
			r.Patch("/cards/{cardID}/move", moveCard(opts.BoardStore))
			r.Post("/cards/{cardID}/move", moveCard(opts.BoardStore))
			r.Get("/cards/{cardID}/comments", cardComments(opts.BoardStore))
			r.Post("/cards/{cardID}/comments", createCardComment(opts.BoardStore))
			r.Get("/wiki", listWikiPages(opts.BoardStore))
			r.Post("/wiki", createWikiPage(opts.BoardStore))
			r.Get("/wiki/{pageID}", wikiPage(opts.BoardStore))
			r.Patch("/wiki/{pageID}", updateWikiPage(opts.BoardStore))
			r.Group(func(r chi.Router) {
				r.Use(requireAdmin(opts.AuthStore))
				r.Get("/members", listMembers(opts.TeamStore))
				r.Post("/members", createMember(opts.TeamStore))
				r.Patch("/members/{memberID}", updateMember(opts.TeamStore))
			})
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

type createCardRequest struct {
	ColumnID      string `json:"columnId"`
	Title         string `json:"title"`
	OwnerInitials string `json:"ownerInitials"`
}

type boardRequest struct {
	Name string `json:"name"`
}

type columnRequest struct {
	Title string `json:"title"`
}

type updateCardRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Priority      string `json:"priority"`
	OwnerInitials string `json:"ownerInitials"`
	Due           string `json:"due"`
}

type moveCardRequest struct {
	ColumnID string `json:"columnId"`
	Position int    `json:"position"`
}

type sprintRequest struct {
	BoardID  string `json:"boardId"`
	Name     string `json:"name"`
	Goal     string `json:"goal"`
	StartsOn string `json:"startsOn"`
	EndsOn   string `json:"endsOn"`
}

type completeSprintRequest struct {
	Rollover []sprintRolloverRequest `json:"rollover"`
}

type sprintRolloverRequest struct {
	CardID   string `json:"cardId"`
	SprintID string `json:"sprintId"`
}

type assignSprintRequest struct {
	SprintID string `json:"sprintId"`
}

type createCardCommentRequest struct {
	Body string `json:"body"`
}

type wikiPageRequest struct {
	Title        string `json:"title"`
	BodyMarkdown string `json:"bodyMarkdown"`
}

type memberRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type memberRoleRequest struct {
	Role string `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const sessionCookieName = "arqboard_session"

type loggerContextKey struct{}

func login(store AuthStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "auth store is unavailable")
			return
		}

		var payload loginRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Email) == "" || strings.TrimSpace(payload.Password) == "" {
			writeError(w, http.StatusBadRequest, "invalid_login", "email and password are required")
			return
		}

		session, err := store.Login(r.Context(), db.LoginParams{
			Email:    payload.Email,
			Password: payload.Password,
		})
		if err != nil {
			writeAuthError(w, r, err)
			return
		}

		http.SetCookie(w, sessionCookie(r, session.Token, session.ExpiresAt))
		writeJSON(w, http.StatusOK, session.User)
	}
}

func currentUser(store AuthStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "auth store is unavailable")
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}

		user, err := store.CurrentUser(r.Context(), cookie.Value)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, user)
	}
}

func logout(store AuthStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, cookieErr := r.Cookie(sessionCookieName)
		if cookieErr == nil && strings.TrimSpace(cookie.Value) != "" {
			if store == nil {
				writeError(w, http.StatusServiceUnavailable, "store_unavailable", "auth store is unavailable")
				return
			}
			if err := store.Logout(r.Context(), cookie.Value); err != nil {
				writeAuthError(w, r, err)
				return
			}
		}

		http.SetCookie(w, expiredSessionCookie(r))
		w.WriteHeader(http.StatusNoContent)
	}
}

func requireAuth(store AuthStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || strings.TrimSpace(cookie.Value) == "" {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			if _, err := store.CurrentUser(r.Context(), cookie.Value); err != nil {
				writeAuthError(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func requireAdmin(store AuthStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				writeError(w, http.StatusServiceUnavailable, "store_unavailable", "auth store is unavailable")
				return
			}

			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || strings.TrimSpace(cookie.Value) == "" {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}

			user, err := store.CurrentUser(r.Context(), cookie.Value)
			if err != nil {
				writeAuthError(w, r, err)
				return
			}
			if !user.IsAdmin {
				writeError(w, http.StatusForbidden, "forbidden", "admin access required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func defaultBoard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		board, err := store.GetDefaultBoard(r.Context())
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, board)
	}
}

func listWorkspaces(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		workspaces, err := store.ListWorkspaces(r.Context())
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, workspaces)
	}
}

func listBoards(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		boards, err := store.ListBoards(r.Context())
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, boards)
	}
}

func boardByID(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		board, err := store.GetBoard(r.Context(), chi.URLParam(r, "boardID"))
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, board)
	}
}

func createBoard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload boardRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid_board", "name is required")
			return
		}

		board, err := store.CreateBoard(r.Context(), db.CreateBoardParams{Name: payload.Name})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, board)
	}
}

func createColumn(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload columnRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Title) == "" {
			writeError(w, http.StatusBadRequest, "invalid_column", "title is required")
			return
		}

		board, err := store.CreateColumn(r.Context(), db.CreateColumnParams{
			BoardID: chi.URLParam(r, "boardID"),
			Title:   payload.Title,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, board)
	}
}

func planningDashboard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		dashboard, err := store.GetPlanningDashboard(r.Context(), r.URL.Query().Get("boardId"))
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, dashboard)
	}
}

func createSprint(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload sprintRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid_sprint", "name is required")
			return
		}

		sprint, err := store.CreateSprint(r.Context(), db.CreateSprintParams{
			BoardID:  payload.BoardID,
			Name:     payload.Name,
			Goal:     payload.Goal,
			StartsOn: payload.StartsOn,
			EndsOn:   payload.EndsOn,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, sprint)
	}
}

func startSprint(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		sprint, err := store.StartSprint(r.Context(), chi.URLParam(r, "sprintID"))
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, sprint)
	}
}

func completeSprint(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload completeSprintRequest
		if r.ContentLength != 0 {
			if err := decodeJSON(w, r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
				return
			}
		}
		rollover := make([]db.SprintRolloverDecision, 0, len(payload.Rollover))
		for _, decision := range payload.Rollover {
			rollover = append(rollover, db.SprintRolloverDecision{
				CardID:   decision.CardID,
				SprintID: decision.SprintID,
			})
		}

		sprint, err := store.CompleteSprint(r.Context(), db.CompleteSprintParams{
			SprintID: chi.URLParam(r, "sprintID"),
			Rollover: rollover,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, sprint)
	}
}

func updateColumn(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload columnRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Title) == "" {
			writeError(w, http.StatusBadRequest, "invalid_column", "title is required")
			return
		}

		board, err := store.UpdateColumn(r.Context(), db.UpdateColumnParams{
			ColumnID: chi.URLParam(r, "columnID"),
			Title:    payload.Title,
		})
		if err != nil {
			writeStoreError(w, r, err)
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
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, card)
	}
}

func cardDetail(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		detail, err := store.GetCardDetail(r.Context(), chi.URLParam(r, "cardID"))
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, detail)
	}
}

func updateCard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload updateCardRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Title) == "" {
			writeError(w, http.StatusBadRequest, "invalid_card", "title is required")
			return
		}

		card, err := store.UpdateCard(r.Context(), db.UpdateCardParams{
			CardID:        chi.URLParam(r, "cardID"),
			Title:         payload.Title,
			Description:   payload.Description,
			Priority:      payload.Priority,
			OwnerInitials: payload.OwnerInitials,
			Due:           payload.Due,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, card)
	}
}

func assignCardToSprint(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload assignSprintRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}

		card, err := store.AssignCardToSprint(r.Context(), db.AssignCardToSprintParams{
			CardID:   chi.URLParam(r, "cardID"),
			SprintID: payload.SprintID,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, card)
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
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, board)
	}
}

func cardComments(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		detail, err := store.GetCardDetail(r.Context(), chi.URLParam(r, "cardID"))
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, detail.Comments)
	}
}

func createCardComment(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload createCardCommentRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Body) == "" {
			writeError(w, http.StatusBadRequest, "invalid_comment", "body is required")
			return
		}

		detail, err := store.CreateCardComment(r.Context(), db.CreateCardCommentParams{
			CardID: chi.URLParam(r, "cardID"),
			Body:   payload.Body,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, detail)
	}
}

func listWikiPages(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		pages, err := store.ListWikiPages(r.Context())
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, pages)
	}
}

func wikiPage(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		page, err := store.GetWikiPage(r.Context(), chi.URLParam(r, "pageID"))
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, page)
	}
}

func createWikiPage(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload wikiPageRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Title) == "" {
			writeError(w, http.StatusBadRequest, "invalid_wiki_page", "title is required")
			return
		}

		page, err := store.CreateWikiPage(r.Context(), db.CreateWikiPageParams{
			Title:        payload.Title,
			BodyMarkdown: payload.BodyMarkdown,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, page)
	}
}

func updateWikiPage(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "board store is unavailable")
			return
		}

		var payload wikiPageRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Title) == "" {
			writeError(w, http.StatusBadRequest, "invalid_wiki_page", "title is required")
			return
		}

		page, err := store.UpdateWikiPage(r.Context(), db.UpdateWikiPageParams{
			PageID:       chi.URLParam(r, "pageID"),
			Title:        payload.Title,
			BodyMarkdown: payload.BodyMarkdown,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, page)
	}
}

func listMembers(store TeamStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "team store is unavailable")
			return
		}

		members, err := store.ListWorkspaceMembers(r.Context())
		if err != nil {
			writeTeamStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, members)
	}
}

func createMember(store TeamStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "team store is unavailable")
			return
		}

		var payload memberRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Email) == "" || strings.TrimSpace(payload.Password) == "" {
			writeError(w, http.StatusBadRequest, "invalid_member", "email and temporary password are required")
			return
		}

		member, err := store.CreateWorkspaceMember(r.Context(), db.CreateWorkspaceMemberParams{
			Email:       payload.Email,
			DisplayName: payload.DisplayName,
			Password:    payload.Password,
			Role:        payload.Role,
		})
		if err != nil {
			writeTeamStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, member)
	}
}

func updateMember(store TeamStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "team store is unavailable")
			return
		}

		var payload memberRoleRequest
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(payload.Role) == "" {
			writeError(w, http.StatusBadRequest, "invalid_member", "role is required")
			return
		}

		member, err := store.UpdateWorkspaceMember(r.Context(), db.UpdateWorkspaceMemberParams{
			MemberID: chi.URLParam(r, "memberID"),
			Role:     payload.Role,
		})
		if err != nil {
			writeTeamStoreError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, member)
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
			logRequestFailure(r, "readiness", http.StatusServiceUnavailable, "not_ready", err)
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
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			ctx := context.WithValue(r.Context(), loggerContextKey{}, logger)
			next.ServeHTTP(ww, r.WithContext(ctx))
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", middleware.GetReqID(r.Context()),
				"status", status,
				"bytes", ww.BytesWritten(),
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

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	writeDataStoreError(w, r, err, "board_store", "board store is unavailable")
}

func writeTeamStoreError(w http.ResponseWriter, r *http.Request, err error) {
	writeDataStoreError(w, r, err, "team_store", "team store is unavailable")
}

func writeDataStoreError(w http.ResponseWriter, r *http.Request, err error, component string, unavailableMessage string) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "request failed"
	switch {
	case errors.Is(err, db.ErrValidation):
		status = http.StatusBadRequest
		code = "invalid_request"
		message = "request validation failed"
	case errors.Is(err, db.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		message = "resource not found"
	case errors.Is(err, db.ErrDatabaseUnavailable):
		status = http.StatusServiceUnavailable
		code = "store_unavailable"
		message = unavailableMessage
	}
	logRequestFailure(r, component, status, code, err)
	writeError(w, status, code, message)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "request failed"
	switch {
	case errors.Is(err, db.ErrInvalidCredentials), errors.Is(err, db.ErrUnauthenticated):
		status = http.StatusUnauthorized
		code = "unauthenticated"
		message = "invalid credentials"
	case errors.Is(err, db.ErrDatabaseUnavailable):
		status = http.StatusServiceUnavailable
		code = "store_unavailable"
		message = "auth store is unavailable"
	}
	logRequestFailure(r, "auth_store", status, code, err)
	writeError(w, status, code, message)
}

func logRequestFailure(r *http.Request, component string, status int, code string, err error) {
	if status < http.StatusInternalServerError || r == nil || err == nil {
		return
	}
	logger, ok := r.Context().Value(loggerContextKey{}).(*slog.Logger)
	if !ok || logger == nil {
		return
	}
	logger.Error("request failed",
		"component", component,
		"status", status,
		"code", code,
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", middleware.GetReqID(r.Context()),
		"error", err.Error(),
	)
}

func sessionCookie(r *http.Request, token string, expiresAt time.Time) *http.Cookie {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}

	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	}
}

func expiredSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
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
