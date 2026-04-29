package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spolnik/arqboard/internal/db"
)

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ready(ctx context.Context) error {
	return fn(ctx)
}

func TestHealthzDoesNotCallReadinessChecker(t *testing.T) {
	called := false
	router := NewRouter(Options{
		Readiness: readinessFunc(func(context.Context) error {
			called = true
			return nil
		}),
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if called {
		t.Fatal("healthz called readiness checker")
	}
}

func TestReadyzReportsDatabaseReadiness(t *testing.T) {
	router := NewRouter(Options{
		Readiness: readinessFunc(func(context.Context) error {
			return errors.New("database unavailable")
		}),
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestReadyzSucceedsWhenCheckerSucceeds(t *testing.T) {
	router := NewRouter(Options{
		Readiness: readinessFunc(func(context.Context) error {
			return nil
		}),
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestRouterServesReactAppAndFallback(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<main>ARQboard app</main>")},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('ok')"),
			Mode: fs.ModePerm,
		},
	}
	router := NewRouter(Options{StaticFS: staticFS})

	for _, path := range []string{"/", "/boards/demo"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))

		if res.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, res.Code, http.StatusOK)
		}
		if res.Body.String() != "<main>ARQboard app</main>" {
			t.Fatalf("%s body = %q", path, res.Body.String())
		}
	}
}

func TestRouterReturnsDefaultBoard(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{
		board: testBoard(),
	}})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/boards/default", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}

	var board db.Board
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if board.Name != "Platform Board" {
		t.Fatalf("board.Name = %q, want Platform Board", board.Name)
	}
}

func TestRouterCreatesCards(t *testing.T) {
	store := fakeBoardStore{
		card: db.BoardCard{
			ID:       "card-new",
			ColumnID: "column-planned",
			Title:    "Run smoke test",
			Owner:    "QA",
			Priority: "Normal",
			Due:      "Later",
		},
	}
	router := NewRouter(Options{BoardStore: store})
	body := bytes.NewBufferString(`{"columnId":"column-planned","title":"Run smoke test","ownerInitials":"qa"}`)

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/cards", body))

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
	}

	var card db.BoardCard
	if err := json.NewDecoder(res.Body).Decode(&card); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if card.ID != "card-new" {
		t.Fatalf("card.ID = %q, want card-new", card.ID)
	}
}

func TestRouterMovesCards(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{
		board: testBoard(),
	}})
	body := bytes.NewBufferString(`{"columnId":"column-review","position":0}`)

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPatch, "/api/cards/card-1/move", body))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}

	var board db.Board
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if board.ID != "board-1" {
		t.Fatalf("board.ID = %q, want board-1", board.ID)
	}
}

func TestRouterUpdatesCardsAndCreatesComments(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{
		card: db.BoardCard{ID: "card-1", Title: "Updated card", Owner: "QA", Priority: "High", Due: "May 9"},
		detail: db.CardDetail{
			Card:     db.BoardCard{ID: "card-1", Title: "Updated card", Owner: "QA", Priority: "High", Due: "May 9"},
			Comments: []db.CardComment{{ID: "comment-1", CardID: "card-1", Body: "Looks good"}},
			Activity: []db.ActivityEvent{{ID: "event-1", CardID: "card-1", EventType: "card.commented"}},
		},
	}})

	update := httptest.NewRecorder()
	router.ServeHTTP(update, httptest.NewRequest(http.MethodPatch, "/api/cards/card-1", bytes.NewBufferString(`{
		"title":"Updated card",
		"description":"Updated body",
		"priority":"high",
		"ownerInitials":"qa",
		"due":"May 9"
	}`)))

	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", update.Code, http.StatusOK)
	}

	comment := httptest.NewRecorder()
	router.ServeHTTP(comment, httptest.NewRequest(http.MethodPost, "/api/cards/card-1/comments", bytes.NewBufferString(`{"body":"Looks good"}`)))

	if comment.Code != http.StatusCreated {
		t.Fatalf("comment status = %d, want %d", comment.Code, http.StatusCreated)
	}

	var detail db.CardDetail
	if err := json.NewDecoder(comment.Body).Decode(&detail); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if len(detail.Comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(detail.Comments))
	}
}

func TestRouterReturnsCardDetailAndComments(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{
		detail: db.CardDetail{
			Card:     db.BoardCard{ID: "card-1", Title: "Updated card"},
			Comments: []db.CardComment{{ID: "comment-1", CardID: "card-1", Body: "Looks good"}},
			Activity: []db.ActivityEvent{{ID: "event-1", CardID: "card-1", EventType: "card.commented"}},
		},
	}})

	detail := httptest.NewRecorder()
	router.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/cards/card-1", nil))
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", detail.Code, http.StatusOK)
	}

	comments := httptest.NewRecorder()
	router.ServeHTTP(comments, httptest.NewRequest(http.MethodGet, "/api/cards/card-1/comments", nil))
	if comments.Code != http.StatusOK {
		t.Fatalf("comments status = %d, want %d", comments.Code, http.StatusOK)
	}

	var body []db.CardComment
	if err := json.NewDecoder(comments.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if len(body) != 1 || body[0].Body != "Looks good" {
		t.Fatalf("comments body = %#v, want one persisted comment", body)
	}
}

func TestRouterReturnsAndUpdatesWikiPages(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{
		wikiPage: db.WikiPage{
			ID:           "wiki-1",
			Title:        "Deployment checklist",
			Slug:         "deployment-checklist",
			BodyMarkdown: "# Deploy",
		},
		wikiPages: []db.WikiPage{{ID: "wiki-1", Title: "Deployment checklist", Slug: "deployment-checklist"}},
	}})

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/wiki", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", list.Code, http.StatusOK)
	}

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/wiki/wiki-1", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", get.Code, http.StatusOK)
	}

	update := httptest.NewRecorder()
	router.ServeHTTP(update, httptest.NewRequest(http.MethodPatch, "/api/wiki/wiki-1", bytes.NewBufferString(`{
		"title":"Deployment checklist",
		"bodyMarkdown":"# Deploy\n\n- Build"
	}`)))
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", update.Code, http.StatusOK)
	}

	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/wiki", bytes.NewBufferString(`{
		"title":"Release runbook",
		"bodyMarkdown":"# Release"
	}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", create.Code, http.StatusCreated)
	}
}

func TestRouterValidatesWritePayloads(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{}})

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create card missing title", method: http.MethodPost, path: "/api/cards", body: `{"columnId":"column-planned"}`},
		{name: "create card bad json", method: http.MethodPost, path: "/api/cards", body: `{`},
		{name: "update card missing title", method: http.MethodPatch, path: "/api/cards/card-1", body: `{"description":"Body"}`},
		{name: "update card bad json", method: http.MethodPatch, path: "/api/cards/card-1", body: `{`},
		{name: "move missing column", method: http.MethodPatch, path: "/api/cards/card-1/move", body: `{"position":0}`},
		{name: "move bad json", method: http.MethodPatch, path: "/api/cards/card-1/move", body: `{`},
		{name: "comment missing body", method: http.MethodPost, path: "/api/cards/card-1/comments", body: `{}`},
		{name: "comment bad json", method: http.MethodPost, path: "/api/cards/card-1/comments", body: `{`},
		{name: "create wiki missing title", method: http.MethodPost, path: "/api/wiki", body: `{"bodyMarkdown":"Body"}`},
		{name: "create wiki bad json", method: http.MethodPost, path: "/api/wiki", body: `{`},
		{name: "update wiki missing title", method: http.MethodPatch, path: "/api/wiki/wiki-1", body: `{"bodyMarkdown":"Body"}`},
		{name: "update wiki bad json", method: http.MethodPatch, path: "/api/wiki/wiki-1", body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body)))

			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRouterMapsStoreErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "validation", err: db.ErrValidation, want: http.StatusBadRequest},
		{name: "not found", err: db.ErrNotFound, want: http.StatusNotFound},
		{name: "database unavailable", err: db.ErrDatabaseUnavailable, want: http.StatusServiceUnavailable},
		{name: "internal", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(Options{BoardStore: fakeBoardStore{err: tt.err}})
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/boards/default", nil))

			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
		})
	}
}

func TestRouterHandlesMissingStoreAndStaticMisses(t *testing.T) {
	router := NewRouter(Options{StaticFS: fstest.MapFS{}})

	api := httptest.NewRecorder()
	router.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/boards/default", nil))
	if api.Code != http.StatusServiceUnavailable {
		t.Fatalf("api status = %d, want %d", api.Code, http.StatusServiceUnavailable)
	}

	asset := httptest.NewRecorder()
	router.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/missing.js", nil))
	if asset.Code != http.StatusNotFound {
		t.Fatalf("asset status = %d, want %d", asset.Code, http.StatusNotFound)
	}

	apiMiss := httptest.NewRecorder()
	router.ServeHTTP(apiMiss, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	if apiMiss.Code != http.StatusNotFound {
		t.Fatalf("api miss status = %d, want %d", apiMiss.Code, http.StatusNotFound)
	}
}

func TestRouterLogsRequests(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&log, nil))
	router := NewRouter(Options{Logger: logger})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(log.String(), `"msg":"request"`) {
		t.Fatalf("log = %q, want request entry", log.String())
	}
}

func TestRouterValidatesCreateCardPayload(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{}})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBufferString(`{"columnId":"column-planned"}`)))

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

type fakeBoardStore struct {
	board     db.Board
	card      db.BoardCard
	detail    db.CardDetail
	wikiPage  db.WikiPage
	wikiPages []db.WikiPage
	err       error
}

func (store fakeBoardStore) GetDefaultBoard(context.Context) (db.Board, error) {
	if store.err != nil {
		return db.Board{}, store.err
	}
	return store.board, nil
}

func (store fakeBoardStore) CreateCard(_ context.Context, _ db.CreateCardParams) (db.BoardCard, error) {
	if store.err != nil {
		return db.BoardCard{}, store.err
	}
	return store.card, nil
}

func (store fakeBoardStore) MoveCard(_ context.Context, _ db.MoveCardParams) (db.Board, error) {
	if store.err != nil {
		return db.Board{}, store.err
	}
	return store.board, nil
}

func (store fakeBoardStore) GetCardDetail(_ context.Context, _ string) (db.CardDetail, error) {
	if store.err != nil {
		return db.CardDetail{}, store.err
	}
	return store.detail, nil
}

func (store fakeBoardStore) UpdateCard(_ context.Context, _ db.UpdateCardParams) (db.BoardCard, error) {
	if store.err != nil {
		return db.BoardCard{}, store.err
	}
	return store.card, nil
}

func (store fakeBoardStore) CreateCardComment(_ context.Context, _ db.CreateCardCommentParams) (db.CardDetail, error) {
	if store.err != nil {
		return db.CardDetail{}, store.err
	}
	return store.detail, nil
}

func (store fakeBoardStore) ListWikiPages(context.Context) ([]db.WikiPage, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.wikiPages, nil
}

func (store fakeBoardStore) GetWikiPage(_ context.Context, _ string) (db.WikiPage, error) {
	if store.err != nil {
		return db.WikiPage{}, store.err
	}
	return store.wikiPage, nil
}

func (store fakeBoardStore) CreateWikiPage(_ context.Context, _ db.CreateWikiPageParams) (db.WikiPage, error) {
	if store.err != nil {
		return db.WikiPage{}, store.err
	}
	return store.wikiPage, nil
}

func (store fakeBoardStore) UpdateWikiPage(_ context.Context, _ db.UpdateWikiPageParams) (db.WikiPage, error) {
	if store.err != nil {
		return db.WikiPage{}, store.err
	}
	return store.wikiPage, nil
}

func testBoard() db.Board {
	return db.Board{
		ID:   "board-1",
		Name: "Platform Board",
		Slug: "platform",
		Columns: []db.BoardColumn{
			{
				ID:       "column-planned",
				Title:    "Planned",
				Position: 0,
				Cards: []db.BoardCard{
					{ID: "card-1", ColumnID: "column-planned", Title: "Wire auth session cookie flow", Owner: "MS", Priority: "High", Due: "Apr 30"},
				},
			},
			{ID: "column-review", Title: "Ready for review", Position: 1},
		},
	}
}
