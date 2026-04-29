package mcpserver

import (
	"context"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spolnik/arqboard/internal/db"
)

type Store interface {
	ListBoards(context.Context) ([]db.BoardSummary, error)
	GetBoard(context.Context, string) (db.Board, error)
	CreateBoard(context.Context, db.CreateBoardParams) (db.Board, error)
	CreateCard(context.Context, db.CreateCardParams) (db.BoardCard, error)
	UpdateCard(context.Context, db.UpdateCardParams) (db.BoardCard, error)
	MoveCard(context.Context, db.MoveCardParams) (db.Board, error)
	ListWikiPages(context.Context) ([]db.WikiPage, error)
	CreateWikiPage(context.Context, db.CreateWikiPageParams) (db.WikiPage, error)
	UpdateWikiPage(context.Context, db.UpdateWikiPageParams) (db.WikiPage, error)
}

type listBoardsInput struct{}

type listBoardsOutput struct {
	Boards []db.BoardSummary `json:"boards" jsonschema:"Boards available in ARQboard."`
}

type getBoardInput struct {
	BoardID string `json:"boardId" jsonschema:"UUID of the board to load."`
}

type createBoardInput struct {
	Name string `json:"name" jsonschema:"Board name to create."`
}

type searchCardsInput struct {
	BoardID string `json:"boardId,omitempty" jsonschema:"Optional UUID of the board to search. When omitted, all boards are searched."`
	Query   string `json:"query" jsonschema:"Text to search for in card title, description, metadata, board, or column."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum matches to return. Defaults to 20 and caps at 50."`
}

type searchCardsOutput struct {
	Matches []searchCardMatch `json:"matches" jsonschema:"Card matches with board and column context."`
}

type searchCardMatch struct {
	Board  searchBoardRef  `json:"board" jsonschema:"Board containing the matched card."`
	Column searchColumnRef `json:"column" jsonschema:"Column containing the matched card."`
	Card   db.BoardCard    `json:"card" jsonschema:"Matched card."`
}

type searchBoardRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type searchColumnRef struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

type createCardInput struct {
	ColumnID      string `json:"columnId" jsonschema:"UUID of the column where the card should be created."`
	Title         string `json:"title" jsonschema:"Card title."`
	Description   string `json:"description,omitempty" jsonschema:"Optional card description."`
	Priority      string `json:"priority,omitempty" jsonschema:"Optional priority: low, normal, high, or urgent."`
	OwnerInitials string `json:"ownerInitials,omitempty" jsonschema:"Optional owner initials."`
	Due           string `json:"due,omitempty" jsonschema:"Optional human-readable due label."`
}

type updateCardInput struct {
	CardID        string `json:"cardId" jsonschema:"UUID of the card to update."`
	Title         string `json:"title" jsonschema:"Updated card title."`
	Description   string `json:"description" jsonschema:"Updated card description."`
	Priority      string `json:"priority,omitempty" jsonschema:"Priority: low, normal, high, or urgent."`
	OwnerInitials string `json:"ownerInitials,omitempty" jsonschema:"Owner initials."`
	Due           string `json:"due,omitempty" jsonschema:"Human-readable due label."`
}

type moveCardInput struct {
	CardID   string `json:"cardId" jsonschema:"UUID of the card to move."`
	ColumnID string `json:"columnId" jsonschema:"UUID of the destination column."`
	Position int    `json:"position" jsonschema:"Zero-based destination position."`
}

type listWikiPagesInput struct{}

type listWikiPagesOutput struct {
	Pages []db.WikiPage `json:"pages" jsonschema:"Wiki pages available in ARQboard."`
}

type createWikiPageInput struct {
	Title        string `json:"title" jsonschema:"Wiki page title."`
	BodyMarkdown string `json:"bodyMarkdown,omitempty" jsonschema:"Markdown body for the wiki page."`
}

type updateWikiPageInput struct {
	PageID       string `json:"pageId" jsonschema:"UUID of the wiki page to update."`
	Title        string `json:"title" jsonschema:"Updated wiki page title."`
	BodyMarkdown string `json:"bodyMarkdown" jsonschema:"Updated markdown body."`
}

func New(store Store) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "arqboard",
		Version: "v0.0.0",
	}, nil)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_list_boards",
		Description: "List ARQboard boards with column and card counts.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ listBoardsInput) (*sdkmcp.CallToolResult, listBoardsOutput, error) {
		boards, err := store.ListBoards(ctx)
		return nil, listBoardsOutput{Boards: boards}, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_get_board",
		Description: "Load a board with columns, cards, and linked wiki pages.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input getBoardInput) (*sdkmcp.CallToolResult, db.Board, error) {
		board, err := store.GetBoard(ctx, input.BoardID)
		return nil, board, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_create_board",
		Description: "Create a new ARQboard board with default columns.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input createBoardInput) (*sdkmcp.CallToolResult, db.Board, error) {
		board, err := store.CreateBoard(ctx, db.CreateBoardParams{Name: input.Name})
		return nil, board, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_search_cards",
		Description: "Search ARQboard cards or tickets by text and return board and column context.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input searchCardsInput) (*sdkmcp.CallToolResult, searchCardsOutput, error) {
		output, err := searchCards(ctx, store, input)
		return nil, output, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_create_card",
		Description: "Create a card in a column, optionally setting description, priority, owner, and due label.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input createCardInput) (*sdkmcp.CallToolResult, db.BoardCard, error) {
		card, err := store.CreateCard(ctx, db.CreateCardParams{
			ColumnID:      input.ColumnID,
			Title:         input.Title,
			OwnerInitials: input.OwnerInitials,
		})
		if err != nil || !createCardNeedsUpdate(input) {
			return nil, card, err
		}
		card, err = store.UpdateCard(ctx, db.UpdateCardParams{
			CardID:        card.ID,
			Title:         input.Title,
			Description:   input.Description,
			Priority:      input.Priority,
			OwnerInitials: input.OwnerInitials,
			Due:           input.Due,
		})
		return nil, card, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_update_card",
		Description: "Update card title, description, priority, owner, and due label.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input updateCardInput) (*sdkmcp.CallToolResult, db.BoardCard, error) {
		card, err := store.UpdateCard(ctx, db.UpdateCardParams{
			CardID:        input.CardID,
			Title:         input.Title,
			Description:   input.Description,
			Priority:      input.Priority,
			OwnerInitials: input.OwnerInitials,
			Due:           input.Due,
		})
		return nil, card, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_move_card",
		Description: "Move a card to another column and position on the same board.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input moveCardInput) (*sdkmcp.CallToolResult, db.Board, error) {
		board, err := store.MoveCard(ctx, db.MoveCardParams{
			CardID:   input.CardID,
			ColumnID: input.ColumnID,
			Position: input.Position,
		})
		return nil, board, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_list_wiki_pages",
		Description: "List ARQboard wiki pages.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ listWikiPagesInput) (*sdkmcp.CallToolResult, listWikiPagesOutput, error) {
		pages, err := store.ListWikiPages(ctx)
		return nil, listWikiPagesOutput{Pages: pages}, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_create_wiki_page",
		Description: "Create a markdown wiki page.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input createWikiPageInput) (*sdkmcp.CallToolResult, db.WikiPage, error) {
		page, err := store.CreateWikiPage(ctx, db.CreateWikiPageParams{
			Title:        input.Title,
			BodyMarkdown: input.BodyMarkdown,
		})
		return nil, page, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "arqboard_update_wiki_page",
		Description: "Update a markdown wiki page.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input updateWikiPageInput) (*sdkmcp.CallToolResult, db.WikiPage, error) {
		page, err := store.UpdateWikiPage(ctx, db.UpdateWikiPageParams{
			PageID:       input.PageID,
			Title:        input.Title,
			BodyMarkdown: input.BodyMarkdown,
		})
		return nil, page, err
	})

	return server
}

func createCardNeedsUpdate(input createCardInput) bool {
	return strings.TrimSpace(input.Description) != "" ||
		strings.TrimSpace(input.Priority) != "" ||
		strings.TrimSpace(input.Due) != ""
}

func searchCards(ctx context.Context, store Store, input searchCardsInput) (searchCardsOutput, error) {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(input.Query)))
	if len(terms) == 0 {
		return searchCardsOutput{}, db.ErrValidation
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	var boards []db.Board
	boardID := strings.TrimSpace(input.BoardID)
	if boardID != "" {
		board, err := store.GetBoard(ctx, boardID)
		if err != nil {
			return searchCardsOutput{}, err
		}
		boards = append(boards, board)
	} else {
		summaries, err := store.ListBoards(ctx)
		if err != nil {
			return searchCardsOutput{}, err
		}
		for _, summary := range summaries {
			board, err := store.GetBoard(ctx, summary.ID)
			if err != nil {
				return searchCardsOutput{}, err
			}
			boards = append(boards, board)
		}
	}

	output := searchCardsOutput{Matches: make([]searchCardMatch, 0)}
	for _, board := range boards {
		for _, column := range board.Columns {
			for _, card := range column.Cards {
				if !cardMatchesTerms(board, column, card, terms) {
					continue
				}
				output.Matches = append(output.Matches, searchCardMatch{
					Board: searchBoardRef{
						ID:   board.ID,
						Name: board.Name,
						Slug: board.Slug,
					},
					Column: searchColumnRef{
						ID:       column.ID,
						Title:    column.Title,
						Position: column.Position,
					},
					Card: card,
				})
				if len(output.Matches) >= limit {
					return output, nil
				}
			}
		}
	}

	return output, nil
}

func cardMatchesTerms(board db.Board, column db.BoardColumn, card db.BoardCard, terms []string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		board.Name,
		board.Slug,
		column.Title,
		card.Title,
		card.Description,
		card.Owner,
		card.Priority,
		card.Due,
	}, " "))

	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
