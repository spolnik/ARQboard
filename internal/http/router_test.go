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
	"time"

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

func TestRouterListsCreatesLoadsBoardsAndColumns(t *testing.T) {
	board := testBoard()
	board.Columns = append(board.Columns, db.BoardColumn{ID: "column-done", Title: "Done", Position: 2})
	renamed := board
	renamed.Columns[0].Title = "Backlog"
	store := fakeBoardStore{
		board: board,
		boards: []db.BoardSummary{
			{ID: "board-1", WorkspaceID: "workspace-1", Name: "Platform Board", Slug: "platform", ColumnCount: 3, CardCount: 1},
		},
		workspaces: []db.Workspace{{ID: "workspace-1", Name: "Platform Engineering", Slug: "platform-engineering"}},
		renamed:    renamed,
	}
	router := NewRouter(Options{BoardStore: store})

	workspaces := httptest.NewRecorder()
	router.ServeHTTP(workspaces, httptest.NewRequest(http.MethodGet, "/api/workspaces", nil))
	if workspaces.Code != http.StatusOK {
		t.Fatalf("workspaces status = %d, want %d", workspaces.Code, http.StatusOK)
	}

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/boards", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", list.Code, http.StatusOK)
	}
	var summaries []db.BoardSummary
	if err := json.NewDecoder(list.Body).Decode(&summaries); err != nil {
		t.Fatalf("Decode list returned error: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ColumnCount != 3 {
		t.Fatalf("summaries = %#v, want one board with columns", summaries)
	}

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/boards/board-1", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", get.Code, http.StatusOK)
	}

	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/boards", bytes.NewBufferString(`{"name":"Release Train"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", create.Code, http.StatusCreated)
	}

	column := httptest.NewRecorder()
	router.ServeHTTP(column, httptest.NewRequest(http.MethodPost, "/api/boards/board-1/columns", bytes.NewBufferString(`{"title":"Blocked"}`)))
	if column.Code != http.StatusCreated {
		t.Fatalf("create column status = %d, want %d", column.Code, http.StatusCreated)
	}

	rename := httptest.NewRecorder()
	router.ServeHTTP(rename, httptest.NewRequest(http.MethodPatch, "/api/columns/column-planned", bytes.NewBufferString(`{"title":"Backlog"}`)))
	if rename.Code != http.StatusOK {
		t.Fatalf("rename column status = %d, want %d", rename.Code, http.StatusOK)
	}
	var renamedBoard db.Board
	if err := json.NewDecoder(rename.Body).Decode(&renamedBoard); err != nil {
		t.Fatalf("Decode rename returned error: %v", err)
	}
	if renamedBoard.Columns[0].Title != "Backlog" {
		t.Fatalf("renamed column = %q, want Backlog", renamedBoard.Columns[0].Title)
	}
}

func TestRouterLoginSetsSessionCookieAndReturnsUser(t *testing.T) {
	user := testUser()
	router := NewRouter(Options{AuthStore: &fakeAuthStore{
		session: db.LoginSession{
			User:      user,
			Token:     "token-123",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{
		"email":"admin@example.com",
		"password":"correct horse battery staple"
	}`)))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	cookie := res.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "arqboard_session=token-123") {
		t.Fatalf("Set-Cookie = %q, want session token", cookie)
	}
	if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Lax") {
		t.Fatalf("Set-Cookie = %q, want HttpOnly SameSite=Lax", cookie)
	}

	var body db.User
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if body.Email != user.Email {
		t.Fatalf("body.Email = %q, want %q", body.Email, user.Email)
	}
}

func TestRouterMeReturnsAuthenticatedUser(t *testing.T) {
	store := &fakeAuthStore{user: testUser()}
	router := NewRouter(Options{AuthStore: store})
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if store.currentUserToken != "token-123" {
		t.Fatalf("currentUserToken = %q, want token-123", store.currentUserToken)
	}
	var body db.User
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if body.ID != "user-1" {
		t.Fatalf("body.ID = %q, want user-1", body.ID)
	}
}

func TestRouterLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	store := &fakeAuthStore{}
	router := NewRouter(Options{AuthStore: store})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if store.loggedOutToken != "token-123" {
		t.Fatalf("loggedOutToken = %q, want token-123", store.loggedOutToken)
	}
	cookie := res.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "arqboard_session=") || !strings.Contains(cookie, "Max-Age=0") {
		t.Fatalf("Set-Cookie = %q, want cleared session cookie", cookie)
	}
}

func TestRouterAuthRejectsMissingOrInvalidCredentials(t *testing.T) {
	tests := []struct {
		name      string
		store     *fakeAuthStore
		method    string
		path      string
		body      string
		addCookie bool
		want      int
	}{
		{name: "missing me cookie", store: &fakeAuthStore{}, method: http.MethodGet, path: "/api/me", want: http.StatusUnauthorized},
		{name: "bad login json", store: &fakeAuthStore{}, method: http.MethodPost, path: "/api/auth/login", body: `{`, want: http.StatusBadRequest},
		{name: "blank login fields", store: &fakeAuthStore{}, method: http.MethodPost, path: "/api/auth/login", body: `{"email":"","password":""}`, want: http.StatusBadRequest},
		{name: "invalid login", store: &fakeAuthStore{err: db.ErrInvalidCredentials}, method: http.MethodPost, path: "/api/auth/login", body: `{"email":"admin@example.com","password":"wrong"}`, want: http.StatusUnauthorized},
		{name: "revoked me session", store: &fakeAuthStore{err: db.ErrUnauthenticated}, method: http.MethodGet, path: "/api/me", addCookie: true, want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(Options{AuthStore: tt.store})
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.addCookie {
				req.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)

			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
		})
	}
}

func TestRouterProtectsWorkspaceAPIsWhenAuthStoreIsConfigured(t *testing.T) {
	store := &fakeAuthStore{user: testUser()}
	router := NewRouter(Options{
		AuthStore:  store,
		BoardStore: fakeBoardStore{board: testBoard()},
	})

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/boards/default", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/boards/default", nil)
	req.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})
	authenticated := httptest.NewRecorder()
	router.ServeHTTP(authenticated, req)
	if authenticated.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want %d", authenticated.Code, http.StatusOK)
	}
	if store.currentUserToken != "token-123" {
		t.Fatalf("currentUserToken = %q, want token-123", store.currentUserToken)
	}
}

func TestRouterManagesWorkspaceMembersForAdmins(t *testing.T) {
	store := &fakeTeamStore{
		members: []db.WorkspaceMember{
			{ID: "member-1", UserID: "user-1", Email: "admin@example.com", DisplayName: "Admin", Role: "owner", IsAdmin: true},
			{ID: "member-2", UserID: "user-2", Email: "dev@example.com", DisplayName: "Dev", Role: "member"},
		},
		member: db.WorkspaceMember{
			ID:          "member-3",
			UserID:      "user-3",
			Email:       "qa@example.com",
			DisplayName: "QA",
			Role:        "viewer",
		},
		updated: db.WorkspaceMember{
			ID:          "member-2",
			UserID:      "user-2",
			Email:       "dev@example.com",
			DisplayName: "Dev",
			Role:        "admin",
		},
	}
	router := NewRouter(Options{
		AuthStore: &fakeAuthStore{user: testUser()},
		TeamStore: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/members", nil)
	listReq.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})
	list := httptest.NewRecorder()
	router.ServeHTTP(list, listReq)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", list.Code, http.StatusOK)
	}
	var members []db.WorkspaceMember
	if err := json.NewDecoder(list.Body).Decode(&members); err != nil {
		t.Fatalf("Decode members returned error: %v", err)
	}
	if len(members) != 2 || members[0].Role != "owner" {
		t.Fatalf("members = %#v, want owner and member", members)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/members", bytes.NewBufferString(`{
		"email":"qa@example.com",
		"displayName":"QA",
		"password":"correct horse battery qa",
		"role":"viewer"
	}`))
	createReq.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})
	create := httptest.NewRecorder()
	router.ServeHTTP(create, createReq)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", create.Code, http.StatusCreated)
	}
	if store.created.Email != "qa@example.com" || store.created.Role != "viewer" {
		t.Fatalf("created params = %#v, want qa viewer", store.created)
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/members/member-2", bytes.NewBufferString(`{"role":"admin"}`))
	updateReq.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})
	update := httptest.NewRecorder()
	router.ServeHTTP(update, updateReq)
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", update.Code, http.StatusOK)
	}
	if store.updatedParams.MemberID != "member-2" || store.updatedParams.Role != "admin" {
		t.Fatalf("updated params = %#v, want member-2 admin", store.updatedParams)
	}
}

func TestRouterRejectsWorkspaceMemberManagementForNonAdmins(t *testing.T) {
	user := testUser()
	user.IsAdmin = false
	router := NewRouter(Options{
		AuthStore: &fakeAuthStore{user: user},
		TeamStore: &fakeTeamStore{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/members", bytes.NewBufferString(`{
		"email":"qa@example.com",
		"password":"correct horse battery qa",
		"role":"viewer"
	}`))
	req.AddCookie(&http.Cookie{Name: "arqboard_session", Value: "token-123"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
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
			Due:      "2026-05-08",
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
		card: db.BoardCard{ID: "card-1", Title: "Updated card", Owner: "QA", Priority: "High", Due: "2026-05-09"},
		detail: db.CardDetail{
			Card:     db.BoardCard{ID: "card-1", Title: "Updated card", Owner: "QA", Priority: "High", Due: "2026-05-09"},
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
		"due":"2026-05-09"
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
		{name: "create board missing name", method: http.MethodPost, path: "/api/boards", body: `{}`},
		{name: "create board bad json", method: http.MethodPost, path: "/api/boards", body: `{`},
		{name: "create column missing title", method: http.MethodPost, path: "/api/boards/board-1/columns", body: `{}`},
		{name: "create column bad json", method: http.MethodPost, path: "/api/boards/board-1/columns", body: `{`},
		{name: "rename column missing title", method: http.MethodPatch, path: "/api/columns/column-1", body: `{}`},
		{name: "rename column bad json", method: http.MethodPatch, path: "/api/columns/column-1", body: `{`},
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
	if !strings.Contains(log.String(), `"status":200`) {
		t.Fatalf("log = %q, want response status", log.String())
	}
}

func TestRouterLogsUnexpectedStoreFailures(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&log, nil))
	router := NewRouter(Options{
		Logger:     logger,
		BoardStore: fakeBoardStore{err: errors.New("constraint failed: UNIQUE constraint failed: columns.board_id, columns.position")},
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/boards/default", nil))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
	logText := log.String()
	for _, want := range []string{
		`"msg":"request failed"`,
		`"component":"board_store"`,
		`"status":500`,
		`"code":"internal_error"`,
		`"error":"constraint failed: UNIQUE constraint failed: columns.board_id, columns.position"`,
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("log = %q, want %s", logText, want)
		}
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

func TestRouterManagesSprintPlanning(t *testing.T) {
	sprint := db.Sprint{
		ID:          "sprint-1",
		WorkspaceID: "workspace-1",
		BoardID:     "board-1",
		Name:        "Sprint 2026-05 Platform",
		Goal:        "Ship planning foundations",
		Status:      "planned",
		StartsOn:    "2026-05-04",
		EndsOn:      "2026-05-15",
	}
	active := sprint
	active.Status = "active"
	active.StartedAt = "2026-05-04T08:00:00Z"
	completed := active
	completed.Status = "completed"
	completed.CompletedAt = "2026-05-15T16:00:00Z"
	card := db.BoardCard{ID: "card-1", ColumnID: "column-planned", SprintID: sprint.ID, Title: "Wire auth session cookie flow", Owner: "MS", Priority: "High", Due: "2026-04-30"}
	store := fakeBoardStore{
		dashboard: db.PlanningDashboard{
			BoardID:        "board-1",
			Backlog:        []db.BoardCard{{ID: "card-1", ColumnID: "column-planned", Title: "Wire auth session cookie flow", Owner: "MS", Priority: "High", Due: "2026-04-30"}},
			PlannedSprints: []db.SprintPlan{{Sprint: sprint}},
		},
		sprint:    sprint,
		started:   active,
		completed: completed,
		assigned:  card,
	}
	router := NewRouter(Options{BoardStore: store})

	dashboard := httptest.NewRecorder()
	router.ServeHTTP(dashboard, httptest.NewRequest(http.MethodGet, "/api/planning?boardId=board-1", nil))
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want %d", dashboard.Code, http.StatusOK)
	}
	var planning db.PlanningDashboard
	if err := json.NewDecoder(dashboard.Body).Decode(&planning); err != nil {
		t.Fatalf("Decode planning returned error: %v", err)
	}
	if len(planning.Backlog) != 1 || len(planning.PlannedSprints) != 1 {
		t.Fatalf("planning = %#v, want backlog and planned sprint", planning)
	}

	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/sprints", bytes.NewBufferString(`{"boardId":"board-1","name":"Sprint 2026-05 Platform","goal":"Ship planning foundations","startsOn":"2026-05-04","endsOn":"2026-05-15"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create sprint status = %d, want %d", create.Code, http.StatusCreated)
	}

	assign := httptest.NewRecorder()
	router.ServeHTTP(assign, httptest.NewRequest(http.MethodPatch, "/api/cards/card-1/sprint", bytes.NewBufferString(`{"sprintId":"sprint-1"}`)))
	if assign.Code != http.StatusOK {
		t.Fatalf("assign sprint status = %d, want %d", assign.Code, http.StatusOK)
	}
	var assigned db.BoardCard
	if err := json.NewDecoder(assign.Body).Decode(&assigned); err != nil {
		t.Fatalf("Decode assigned card returned error: %v", err)
	}
	if assigned.SprintID != "sprint-1" {
		t.Fatalf("assigned.SprintID = %q, want sprint-1", assigned.SprintID)
	}

	start := httptest.NewRecorder()
	router.ServeHTTP(start, httptest.NewRequest(http.MethodPost, "/api/sprints/sprint-1/start", nil))
	if start.Code != http.StatusOK {
		t.Fatalf("start sprint status = %d, want %d", start.Code, http.StatusOK)
	}

	complete := httptest.NewRecorder()
	router.ServeHTTP(complete, httptest.NewRequest(http.MethodPost, "/api/sprints/sprint-1/complete", bytes.NewBufferString(`{"rollover":[{"cardId":"card-1","sprintId":""}]}`)))
	if complete.Code != http.StatusOK {
		t.Fatalf("complete sprint status = %d, want %d", complete.Code, http.StatusOK)
	}
}

type fakeBoardStore struct {
	board      db.Board
	renamed    db.Board
	card       db.BoardCard
	detail     db.CardDetail
	wikiPage   db.WikiPage
	wikiPages  []db.WikiPage
	workspaces []db.Workspace
	boards     []db.BoardSummary
	dashboard  db.PlanningDashboard
	sprint     db.Sprint
	started    db.Sprint
	completed  db.Sprint
	assigned   db.BoardCard
	err        error
}

type fakeAuthStore struct {
	session          db.LoginSession
	user             db.User
	err              error
	currentUserToken string
	loggedOutToken   string
}

type fakeTeamStore struct {
	members       []db.WorkspaceMember
	member        db.WorkspaceMember
	updated       db.WorkspaceMember
	created       db.CreateWorkspaceMemberParams
	updatedParams db.UpdateWorkspaceMemberParams
	err           error
}

func (store *fakeAuthStore) Login(_ context.Context, _ db.LoginParams) (db.LoginSession, error) {
	if store.err != nil {
		return db.LoginSession{}, store.err
	}
	return store.session, nil
}

func (store *fakeAuthStore) CurrentUser(_ context.Context, token string) (db.User, error) {
	store.currentUserToken = token
	if store.err != nil {
		return db.User{}, store.err
	}
	return store.user, nil
}

func (store *fakeAuthStore) Logout(_ context.Context, token string) error {
	store.loggedOutToken = token
	if store.err != nil {
		return store.err
	}
	return nil
}

func (store *fakeTeamStore) ListWorkspaceMembers(context.Context) ([]db.WorkspaceMember, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.members, nil
}

func (store *fakeTeamStore) CreateWorkspaceMember(_ context.Context, params db.CreateWorkspaceMemberParams) (db.WorkspaceMember, error) {
	store.created = params
	if store.err != nil {
		return db.WorkspaceMember{}, store.err
	}
	return store.member, nil
}

func (store *fakeTeamStore) UpdateWorkspaceMember(_ context.Context, params db.UpdateWorkspaceMemberParams) (db.WorkspaceMember, error) {
	store.updatedParams = params
	if store.err != nil {
		return db.WorkspaceMember{}, store.err
	}
	return store.updated, nil
}

func (store fakeBoardStore) GetDefaultBoard(context.Context) (db.Board, error) {
	if store.err != nil {
		return db.Board{}, store.err
	}
	return store.board, nil
}

func (store fakeBoardStore) ListWorkspaces(context.Context) ([]db.Workspace, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.workspaces, nil
}

func (store fakeBoardStore) ListBoards(context.Context) ([]db.BoardSummary, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.boards, nil
}

func (store fakeBoardStore) GetBoard(_ context.Context, _ string) (db.Board, error) {
	if store.err != nil {
		return db.Board{}, store.err
	}
	return store.board, nil
}

func (store fakeBoardStore) CreateBoard(_ context.Context, _ db.CreateBoardParams) (db.Board, error) {
	if store.err != nil {
		return db.Board{}, store.err
	}
	return store.board, nil
}

func (store fakeBoardStore) CreateColumn(_ context.Context, _ db.CreateColumnParams) (db.Board, error) {
	if store.err != nil {
		return db.Board{}, store.err
	}
	return store.board, nil
}

func (store fakeBoardStore) UpdateColumn(_ context.Context, _ db.UpdateColumnParams) (db.Board, error) {
	if store.err != nil {
		return db.Board{}, store.err
	}
	if store.renamed.ID != "" {
		return store.renamed, nil
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

func (store fakeBoardStore) GetPlanningDashboard(context.Context, string) (db.PlanningDashboard, error) {
	if store.err != nil {
		return db.PlanningDashboard{}, store.err
	}
	return store.dashboard, nil
}

func (store fakeBoardStore) CreateSprint(_ context.Context, _ db.CreateSprintParams) (db.Sprint, error) {
	if store.err != nil {
		return db.Sprint{}, store.err
	}
	return store.sprint, nil
}

func (store fakeBoardStore) StartSprint(_ context.Context, _ string) (db.Sprint, error) {
	if store.err != nil {
		return db.Sprint{}, store.err
	}
	return store.started, nil
}

func (store fakeBoardStore) CompleteSprint(_ context.Context, _ db.CompleteSprintParams) (db.Sprint, error) {
	if store.err != nil {
		return db.Sprint{}, store.err
	}
	return store.completed, nil
}

func (store fakeBoardStore) AssignCardToSprint(_ context.Context, _ db.AssignCardToSprintParams) (db.BoardCard, error) {
	if store.err != nil {
		return db.BoardCard{}, store.err
	}
	return store.assigned, nil
}

func testUser() db.User {
	return db.User{
		ID:          "user-1",
		Email:       "admin@example.com",
		DisplayName: "Admin",
		IsAdmin:     true,
	}
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
					{ID: "card-1", ColumnID: "column-planned", Title: "Wire auth session cookie flow", Owner: "MS", Priority: "High", Due: "2026-04-30"},
				},
			},
			{ID: "column-review", Title: "Ready for review", Position: 1},
		},
	}
}
