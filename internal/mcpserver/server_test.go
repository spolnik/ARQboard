package mcpserver

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spolnik/arqboard/internal/db"
	"github.com/spolnik/arqboard/migrations"
)

func TestMCPServerExposesBoardManagementTools(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupStore(t, ctx)
	defer cleanup()

	session := connectClient(t, ctx, store)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	for _, want := range []string{
		"arqboard_list_boards",
		"arqboard_get_board",
		"arqboard_create_board",
		"arqboard_search_cards",
		"arqboard_create_card",
		"arqboard_update_card",
		"arqboard_move_card",
		"arqboard_list_wiki_pages",
		"arqboard_create_wiki_page",
		"arqboard_update_wiki_page",
	} {
		if !slices.Contains(names, want) {
			t.Fatalf("tools = %v, want %s", names, want)
		}
	}
}

func TestMCPServerSearchesPlanningCards(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupStore(t, ctx)
	defer cleanup()

	session := connectClient(t, ctx, store)
	defer session.Close()

	createdBoard := callTool[db.Board](t, ctx, session, "arqboard_create_board", map[string]any{
		"name": "MCP Search Board",
	})
	card := callTool[db.BoardCard](t, ctx, session, "arqboard_create_card", map[string]any{
		"columnId":    createdBoard.Columns[0].ID,
		"title":       "Harden UUID schema request",
		"description": "Every table has a UUID primary key and display names are not identity.",
		"priority":    "high",
		"due":         "2026-05-01",
	})

	results := callTool[searchCardsOutput](t, ctx, session, "arqboard_search_cards", map[string]any{
		"query": "uuid identity",
	})
	if len(results.Matches) != 1 {
		t.Fatalf("len(results.Matches) = %d, want 1; matches=%#v", len(results.Matches), results.Matches)
	}
	match := results.Matches[0]
	if match.Card.ID != card.ID {
		t.Fatalf("match.Card.ID = %q, want %q", match.Card.ID, card.ID)
	}
	if match.Board.ID != createdBoard.ID || match.Column.ID != createdBoard.Columns[0].ID {
		t.Fatalf("match context = %#v, want created board and column", match)
	}

	boardOnly := callTool[searchCardsOutput](t, ctx, session, "arqboard_search_cards", map[string]any{
		"boardId": createdBoard.ID,
		"query":   "harden",
		"limit":   1,
	})
	if len(boardOnly.Matches) != 1 || boardOnly.Matches[0].Card.ID != card.ID {
		t.Fatalf("board-scoped matches = %#v, want created card only", boardOnly.Matches)
	}
}

func TestMCPServerCreatesAndUpdatesPlanningCards(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupStore(t, ctx)
	defer cleanup()

	session := connectClient(t, ctx, store)
	defer session.Close()

	createdBoard := callTool[db.Board](t, ctx, session, "arqboard_create_board", map[string]any{
		"name": "MCP Roadmap",
	})
	if createdBoard.Name != "MCP Roadmap" {
		t.Fatalf("created board name = %q, want MCP Roadmap", createdBoard.Name)
	}
	if len(createdBoard.Columns) == 0 {
		t.Fatal("created board has no columns")
	}

	card := callTool[db.BoardCard](t, ctx, session, "arqboard_create_card", map[string]any{
		"columnId":      createdBoard.Columns[0].ID,
		"title":         "Plan with MCP",
		"description":   "Let an MCP client create and update ARQboard planning work.",
		"priority":      "high",
		"ownerInitials": "AI",
		"due":           "2026-05-08",
	})
	if card.Title != "Plan with MCP" || card.Priority != "High" || card.Owner != "AI" {
		t.Fatalf("created card = %#v, want updated planning card", card)
	}

	updated := callTool[db.BoardCard](t, ctx, session, "arqboard_update_card", map[string]any{
		"cardId":        card.ID,
		"title":         "Plan ARQboard with MCP",
		"description":   "MCP clients can now manage boards and cards through the local server.",
		"priority":      "urgent",
		"ownerInitials": "OPS",
		"due":           "2026-05-15",
	})
	if updated.Title != "Plan ARQboard with MCP" || updated.Priority != "Urgent" || updated.Owner != "OPS" {
		t.Fatalf("updated card = %#v, want persisted MCP update", updated)
	}

	moved := callTool[db.Board](t, ctx, session, "arqboard_move_card", map[string]any{
		"cardId":   card.ID,
		"columnId": createdBoard.Columns[1].ID,
		"position": 0,
	})
	if moved.Columns[1].Cards[0].ID != card.ID {
		t.Fatalf("moved board second column cards = %#v, want moved card first", moved.Columns[1].Cards)
	}
}

func TestMCPServerCreatesAndUpdatesWikiPages(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupStore(t, ctx)
	defer cleanup()

	session := connectClient(t, ctx, store)
	defer session.Close()

	page := callTool[db.WikiPage](t, ctx, session, "arqboard_create_wiki_page", map[string]any{
		"title":        "MCP operations",
		"bodyMarkdown": "# MCP operations\n\nUse local MCP tools to manage ARQboard.",
	})
	if page.Title != "MCP operations" {
		t.Fatalf("created page = %#v, want MCP operations", page)
	}

	updated := callTool[db.WikiPage](t, ctx, session, "arqboard_update_wiki_page", map[string]any{
		"pageId":       page.ID,
		"title":        "MCP runbook",
		"bodyMarkdown": "# MCP runbook\n\nKeep tool calls explicit and logged.",
	})
	if updated.Title != "MCP runbook" || updated.BodyMarkdown == page.BodyMarkdown {
		t.Fatalf("updated page = %#v, want changed title and body", updated)
	}
}

func setupStore(t *testing.T, ctx context.Context) (db.BoardStore, func()) {
	t.Helper()

	databaseURL := "sqlite://" + t.TempDir() + "/arqboard.db"
	migrationFS, err := migrations.ForDriver(string(db.DriverSQLite))
	if err != nil {
		t.Fatalf("ForDriver returned error: %v", err)
	}
	if err := db.MigrateUp(ctx, databaseURL, migrationFS); err != nil {
		t.Fatalf("MigrateUp returned error: %v", err)
	}
	conn, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return db.BoardStore{Conn: conn}, conn.Close
}

func connectClient(t *testing.T, ctx context.Context, store db.BoardStore) *sdkmcp.ClientSession {
	t.Helper()

	server := New(store)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "arqboard-test-client", Version: "v0.0.0"}, nil)
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect returned error: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect returned error: %v", err)
	}
	return session
}

func callTool[T any](t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) T {
	t.Helper()

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("%s CallTool returned protocol error: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("%s CallTool returned tool error: %#v", name, result.Content)
	}

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("%s structured content marshal returned error: %v", name, err)
	}
	var output T
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("%s structured content unmarshal returned error: %v; data=%s", name, err, string(data))
	}
	return output
}
