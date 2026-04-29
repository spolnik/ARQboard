package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/spolnik/arqboard/migrations"
)

func TestDefaultBoardSeedsOnceAndPersistsCardMoves(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	if board.Name != "Platform Board" {
		t.Fatalf("board.Name = %q, want Platform Board", board.Name)
	}
	if len(board.Columns) != 4 {
		t.Fatalf("len(board.Columns) = %d, want 4", len(board.Columns))
	}

	planned := findColumn(t, board, "Planned")
	review := findColumn(t, board, "Ready for review")
	done := findColumn(t, board, "Done")
	if done.Position != 3 {
		t.Fatalf("done.Position = %d, want 3", done.Position)
	}
	if len(planned.Cards) != 2 {
		t.Fatalf("len(planned.Cards) = %d, want seeded cards", len(planned.Cards))
	}

	again, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("second GetDefaultBoard returned error: %v", err)
	}
	if len(findColumn(t, again, "Planned").Cards) != len(planned.Cards) {
		t.Fatal("GetDefaultBoard duplicated seeded cards")
	}

	card, err := store.CreateCard(ctx, CreateCardParams{
		ColumnID:      planned.ID,
		Title:         "Run local smoke test",
		OwnerInitials: "qa",
	})
	if err != nil {
		t.Fatalf("CreateCard returned error: %v", err)
	}

	moved, err := store.MoveCard(ctx, MoveCardParams{
		CardID:   card.ID,
		ColumnID: review.ID,
		Position: 0,
	})
	if err != nil {
		t.Fatalf("MoveCard returned error: %v", err)
	}
	movedReview := findColumn(t, moved, "Ready for review")
	if movedReview.Cards[0].Title != "Run local smoke test" {
		t.Fatalf("first review card = %q, want moved card", movedReview.Cards[0].Title)
	}
	if movedReview.Cards[0].Owner != "QA" {
		t.Fatalf("moved card owner = %q, want QA", movedReview.Cards[0].Owner)
	}

	cleanup()
	reopened, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	persisted, err := reopened.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("reopened GetDefaultBoard returned error: %v", err)
	}
	persistedReview := findColumn(t, persisted, "Ready for review")
	if persistedReview.Cards[0].Title != "Run local smoke test" {
		t.Fatalf("persisted first review card = %q, want moved card", persistedReview.Cards[0].Title)
	}
}

func TestCardDetailUpdateCommentsAndActivityPersist(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	card := findColumn(t, board, "Planned").Cards[0]

	updated, err := store.UpdateCard(ctx, UpdateCardParams{
		CardID:        card.ID,
		Title:         "Wire production auth flow",
		Description:   "Document cookie boundaries and refresh behavior.",
		Priority:      "urgent",
		OwnerInitials: "qa",
		Due:           "May 9",
	})
	if err != nil {
		t.Fatalf("UpdateCard returned error: %v", err)
	}
	if updated.Title != "Wire production auth flow" {
		t.Fatalf("updated title = %q, want saved title", updated.Title)
	}
	if updated.Priority != "Urgent" {
		t.Fatalf("updated priority = %q, want Urgent", updated.Priority)
	}
	if updated.Owner != "QA" {
		t.Fatalf("updated owner = %q, want QA", updated.Owner)
	}

	detail, err := store.CreateCardComment(ctx, CreateCardCommentParams{
		CardID: card.ID,
		Body:   "This needs a rollback note before release.",
	})
	if err != nil {
		t.Fatalf("CreateCardComment returned error: %v", err)
	}
	if len(detail.Comments) != 1 {
		t.Fatalf("len(detail.Comments) = %d, want 1", len(detail.Comments))
	}
	if detail.Comments[0].Body != "This needs a rollback note before release." {
		t.Fatalf("comment body = %q", detail.Comments[0].Body)
	}
	if !hasActivity(detail.Activity, "card.commented") {
		t.Fatal("detail activity missing card.commented event")
	}
	if !hasActivity(detail.Activity, "card.updated") {
		t.Fatal("detail activity missing card.updated event")
	}

	cleanup()
	reopened, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	persisted, err := reopened.GetCardDetail(ctx, card.ID)
	if err != nil {
		t.Fatalf("reopened GetCardDetail returned error: %v", err)
	}
	if persisted.Card.Title != "Wire production auth flow" {
		t.Fatalf("persisted card title = %q, want updated title", persisted.Card.Title)
	}
	if len(persisted.Comments) != 1 {
		t.Fatalf("persisted comments = %d, want 1", len(persisted.Comments))
	}
}

func TestWikiPagesPersistMarkdown(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	if len(board.WikiPages) == 0 {
		t.Fatal("default board has no wiki pages")
	}

	page, err := store.UpdateWikiPage(ctx, UpdateWikiPageParams{
		PageID:       board.WikiPages[0].ID,
		Title:        "Deployment checklist",
		BodyMarkdown: "# Deploy\n\n- Build image\n- Run migrations",
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage returned error: %v", err)
	}
	if page.BodyMarkdown != "# Deploy\n\n- Build image\n- Run migrations" {
		t.Fatalf("updated body = %q", page.BodyMarkdown)
	}

	created, err := store.CreateWikiPage(ctx, CreateWikiPageParams{
		Title:        "Release runbook",
		BodyMarkdown: "# Release runbook\n\nShip carefully.",
	})
	if err != nil {
		t.Fatalf("CreateWikiPage returned error: %v", err)
	}
	if created.Slug != "release-runbook" {
		t.Fatalf("created slug = %q, want release-runbook", created.Slug)
	}

	cleanup()
	reopened, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	persisted, err := reopened.GetWikiPage(ctx, created.ID)
	if err != nil {
		t.Fatalf("reopened GetWikiPage returned error: %v", err)
	}
	if persisted.BodyMarkdown != "# Release runbook\n\nShip carefully." {
		t.Fatalf("persisted wiki body = %q", persisted.BodyMarkdown)
	}
}

func TestBoardStoreValidationAndNotFoundPaths(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	card := findColumn(t, board, "Planned").Cards[0]

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "missing database",
			err: func() error {
				_, err := (BoardStore{}).GetDefaultBoard(ctx)
				return err
			}(),
			want: ErrDatabaseUnavailable,
		},
		{
			name: "create card missing title",
			err: func() error {
				_, err := store.CreateCard(ctx, CreateCardParams{ColumnID: "column"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "create card missing column",
			err: func() error {
				_, err := store.CreateCard(ctx, CreateCardParams{Title: "Card"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "create card unknown column",
			err: func() error {
				_, err := store.CreateCard(ctx, CreateCardParams{ColumnID: "missing", Title: "Card"})
				return err
			}(),
			want: ErrNotFound,
		},
		{
			name: "get card detail missing id",
			err: func() error {
				_, err := store.GetCardDetail(ctx, "")
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "get card detail unknown id",
			err: func() error {
				_, err := store.GetCardDetail(ctx, "missing")
				return err
			}(),
			want: ErrNotFound,
		},
		{
			name: "update card missing id",
			err: func() error {
				_, err := store.UpdateCard(ctx, UpdateCardParams{Title: "Card"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "update card missing title",
			err: func() error {
				_, err := store.UpdateCard(ctx, UpdateCardParams{CardID: card.ID})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "update card invalid priority",
			err: func() error {
				_, err := store.UpdateCard(ctx, UpdateCardParams{CardID: card.ID, Title: "Card", Priority: "blocker"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "update card unknown id",
			err: func() error {
				_, err := store.UpdateCard(ctx, UpdateCardParams{CardID: "missing", Title: "Card"})
				return err
			}(),
			want: ErrNotFound,
		},
		{
			name: "move card missing id",
			err: func() error {
				_, err := store.MoveCard(ctx, MoveCardParams{ColumnID: "column"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "move card missing column",
			err: func() error {
				_, err := store.MoveCard(ctx, MoveCardParams{CardID: card.ID})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "move card negative position",
			err: func() error {
				_, err := store.MoveCard(ctx, MoveCardParams{CardID: card.ID, ColumnID: card.ColumnID, Position: -1})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "move card unknown id",
			err: func() error {
				_, err := store.MoveCard(ctx, MoveCardParams{CardID: "missing", ColumnID: card.ColumnID})
				return err
			}(),
			want: ErrNotFound,
		},
		{
			name: "comment missing card",
			err: func() error {
				_, err := store.CreateCardComment(ctx, CreateCardCommentParams{Body: "Comment"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "comment missing body",
			err: func() error {
				_, err := store.CreateCardComment(ctx, CreateCardCommentParams{CardID: card.ID})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "comment unknown card",
			err: func() error {
				_, err := store.CreateCardComment(ctx, CreateCardCommentParams{CardID: "missing", Body: "Comment"})
				return err
			}(),
			want: ErrNotFound,
		},
		{
			name: "get wiki missing page",
			err: func() error {
				_, err := store.GetWikiPage(ctx, "")
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "get wiki unknown page",
			err: func() error {
				_, err := store.GetWikiPage(ctx, "missing")
				return err
			}(),
			want: ErrNotFound,
		},
		{
			name: "create wiki missing title",
			err: func() error {
				_, err := store.CreateWikiPage(ctx, CreateWikiPageParams{BodyMarkdown: "Body"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "update wiki missing page",
			err: func() error {
				_, err := store.UpdateWikiPage(ctx, UpdateWikiPageParams{Title: "Title"})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "update wiki missing title",
			err: func() error {
				_, err := store.UpdateWikiPage(ctx, UpdateWikiPageParams{PageID: board.WikiPages[0].ID})
				return err
			}(),
			want: ErrValidation,
		},
		{
			name: "update wiki unknown page",
			err: func() error {
				_, err := store.UpdateWikiPage(ctx, UpdateWikiPageParams{PageID: "missing", Title: "Title"})
				return err
			}(),
			want: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.want) {
				t.Fatalf("error = %v, want %v", tt.err, tt.want)
			}
		})
	}
}

func TestWikiListAndSlugCollisions(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	pages, err := store.ListWikiPages(ctx)
	if err != nil {
		t.Fatalf("ListWikiPages returned error: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("len(pages) = %d, want seeded pages", len(pages))
	}

	first, err := store.CreateWikiPage(ctx, CreateWikiPageParams{Title: "Release runbook", BodyMarkdown: "# Release"})
	if err != nil {
		t.Fatalf("first CreateWikiPage returned error: %v", err)
	}
	second, err := store.CreateWikiPage(ctx, CreateWikiPageParams{Title: "Release runbook", BodyMarkdown: "# Release again"})
	if err != nil {
		t.Fatalf("second CreateWikiPage returned error: %v", err)
	}
	if first.Slug != "release-runbook" {
		t.Fatalf("first slug = %q, want release-runbook", first.Slug)
	}
	if second.Slug != "release-runbook-2" {
		t.Fatalf("second slug = %q, want release-runbook-2", second.Slug)
	}

	updated, err := store.UpdateWikiPage(ctx, UpdateWikiPageParams{
		PageID:       second.ID,
		Title:        "Release runbook",
		BodyMarkdown: "# Updated",
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage returned error: %v", err)
	}
	if updated.Slug != "release-runbook-2" {
		t.Fatalf("updated slug = %q, want stable collision suffix", updated.Slug)
	}
}

func TestMoveCardClampsPositionIntoDoneColumn(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	card := findColumn(t, board, "Planned").Cards[0]
	done := findColumn(t, board, "Done")

	moved, err := store.MoveCard(ctx, MoveCardParams{
		CardID:   card.ID,
		ColumnID: done.ID,
		Position: 99,
	})
	if err != nil {
		t.Fatalf("MoveCard returned error: %v", err)
	}

	movedDone := findColumn(t, moved, "Done")
	if len(movedDone.Cards) != 1 {
		t.Fatalf("len(done.Cards) = %d, want 1", len(movedDone.Cards))
	}
	if movedDone.Cards[0].ID != card.ID {
		t.Fatalf("done card = %q, want moved card %q", movedDone.Cards[0].ID, card.ID)
	}
}

func TestBoardHelpers(t *testing.T) {
	for _, value := range []string{"", "low", "normal", "high", "urgent"} {
		if _, err := normalizePriority(value); err != nil {
			t.Fatalf("normalizePriority(%q) returned error: %v", value, err)
		}
	}
	if _, err := normalizePriority("blocker"); !errors.Is(err, ErrValidation) {
		t.Fatalf("normalizePriority invalid error = %v, want ErrValidation", err)
	}
	if ownerInitials("team") != "TEA" {
		t.Fatalf("ownerInitials truncated incorrectly")
	}
	if displayPriority("low") != "Low" || displayPriority("high") != "High" || displayPriority("urgent") != "Urgent" {
		t.Fatalf("displayPriority did not format explicit priorities")
	}
	if activitySummary("card.created") != "Card created" || activitySummary("unknown") != "Activity recorded" {
		t.Fatalf("activitySummary returned unexpected values")
	}
	if currentTimestamp(DriverPostgres) != "now()" || currentTimestamp(DriverSQLite) != "datetime('now')" {
		t.Fatalf("currentTimestamp returned unexpected values")
	}
	if timeText(DriverPostgres, "created_at") != "created_at::text" || timeText(DriverSQLite, "created_at") != "created_at" {
		t.Fatalf("timeText returned unexpected values")
	}
	if jsonPlaceholder(DriverPostgres, 3) != "$3::jsonb" || jsonPlaceholder(DriverSQLite, 3) != "?" {
		t.Fatalf("jsonPlaceholder returned unexpected values")
	}
	if placeholder(DriverPostgres, 2) != "$2" || placeholder(DriverSQLite, 2) != "?" {
		t.Fatalf("placeholder returned unexpected values")
	}
}

func setupBoardStore(t *testing.T, ctx context.Context, databaseURL string) (BoardStore, func()) {
	t.Helper()

	migrationFS, err := migrations.ForDriver(string(DriverSQLite))
	if err != nil {
		t.Fatalf("ForDriver returned error: %v", err)
	}
	if err := MigrateUp(ctx, databaseURL, migrationFS); err != nil {
		t.Fatalf("MigrateUp returned error: %v", err)
	}

	conn, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	return BoardStore{Conn: conn}, conn.Close
}

func findColumn(t *testing.T, board Board, title string) BoardColumn {
	t.Helper()

	for _, column := range board.Columns {
		if column.Title == title {
			return column
		}
	}
	t.Fatalf("column %q not found", title)
	return BoardColumn{}
}

func hasActivity(events []ActivityEvent, eventType string) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}
