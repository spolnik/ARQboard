package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
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

func TestRouterValidatesCreateCardPayload(t *testing.T) {
	router := NewRouter(Options{BoardStore: fakeBoardStore{}})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBufferString(`{"columnId":"column-planned"}`)))

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

type fakeBoardStore struct {
	board db.Board
	card  db.BoardCard
}

func (store fakeBoardStore) GetDefaultBoard(context.Context) (db.Board, error) {
	return store.board, nil
}

func (store fakeBoardStore) CreateCard(_ context.Context, _ db.CreateCardParams) (db.BoardCard, error) {
	return store.card, nil
}

func (store fakeBoardStore) MoveCard(_ context.Context, _ db.MoveCardParams) (db.Board, error) {
	return store.board, nil
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
