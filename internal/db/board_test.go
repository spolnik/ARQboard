package db

import (
	"context"
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
	if len(board.Columns) != 3 {
		t.Fatalf("len(board.Columns) = %d, want 3", len(board.Columns))
	}

	planned := findColumn(t, board, "Planned")
	review := findColumn(t, board, "Ready for review")
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
