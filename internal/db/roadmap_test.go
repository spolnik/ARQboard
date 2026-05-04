package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestRoadmapCreatesEpicsAssignsCardsAndTracksDependencies(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	planned := findColumn(t, board, "Planned")
	done := findColumn(t, board, "Done")

	blocked, err := store.CreateCard(ctx, CreateCardParams{ColumnID: planned.ID, Title: "Build dependency graph"})
	if err != nil {
		t.Fatalf("CreateCard blocked returned error: %v", err)
	}
	blocker, err := store.CreateCard(ctx, CreateCardParams{ColumnID: planned.ID, Title: "Ship schema migration"})
	if err != nil {
		t.Fatalf("CreateCard blocker returned error: %v", err)
	}
	doneCard, err := store.CreateCard(ctx, CreateCardParams{ColumnID: planned.ID, Title: "Draft roadmap copy"})
	if err != nil {
		t.Fatalf("CreateCard done returned error: %v", err)
	}
	if _, err := store.MoveCard(ctx, MoveCardParams{CardID: doneCard.ID, ColumnID: done.ID, Position: 0}); err != nil {
		t.Fatalf("MoveCard done returned error: %v", err)
	}

	epic, err := store.CreateEpic(ctx, CreateEpicParams{
		TeamID:      board.TeamID,
		Title:       "Roadmap, epics, and dependencies",
		Description: "Plan initiatives above individual cards.",
		Status:      "active",
		StartsOn:    "2026-05-04",
		TargetOn:    "2026-05-12",
	})
	if err != nil {
		t.Fatalf("CreateEpic returned error: %v", err)
	}
	if epic.ID == "" || epic.Slug != "roadmap-epics-and-dependencies" {
		t.Fatalf("epic = %#v, want uuid id and slug", epic)
	}

	assigned, err := store.AssignCardToEpic(ctx, AssignCardToEpicParams{CardID: blocked.ID, EpicID: epic.ID})
	if err != nil {
		t.Fatalf("AssignCardToEpic blocked returned error: %v", err)
	}
	if assigned.EpicID != epic.ID {
		t.Fatalf("assigned.EpicID = %q, want %q", assigned.EpicID, epic.ID)
	}
	if _, err := store.AssignCardToEpic(ctx, AssignCardToEpicParams{CardID: doneCard.ID, EpicID: epic.ID}); err != nil {
		t.Fatalf("AssignCardToEpic done returned error: %v", err)
	}

	dependency, err := store.CreateCardDependency(ctx, CreateCardDependencyParams{
		BlockedCardID: blocked.ID,
		BlockerCardID: blocker.ID,
	})
	if err != nil {
		t.Fatalf("CreateCardDependency returned error: %v", err)
	}
	if dependency.ID == "" || dependency.BlockedCardID != blocked.ID || dependency.BlockerCardID != blocker.ID {
		t.Fatalf("dependency = %#v, want persisted blocked-by relation", dependency)
	}

	dashboard, err := store.GetRoadmapDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetRoadmapDashboard returned error: %v", err)
	}
	if dashboard.TeamID != board.TeamID || dashboard.TeamName == "" {
		t.Fatalf("dashboard scope = %#v, want selected team", dashboard)
	}
	roadmapEpic := findRoadmapEpic(t, dashboard, epic.ID)
	if roadmapEpic.TotalCards != 2 || roadmapEpic.CompletedCards != 1 || roadmapEpic.BlockedCards != 1 || roadmapEpic.Risk != "blocked" {
		t.Fatalf("roadmap epic counts = %#v, want 2 total, 1 complete, 1 blocked", roadmapEpic)
	}
	if !roadmapHasCard(roadmapEpic.Cards, blocked.ID) || !roadmapHasCard(roadmapEpic.Cards, doneCard.ID) {
		t.Fatalf("roadmap epic cards = %#v, want assigned cards", roadmapEpic.Cards)
	}
	if !roadmapHasCard(dashboard.UnassignedCards, blocker.ID) {
		t.Fatalf("unassigned cards = %#v, want blocker card", dashboard.UnassignedCards)
	}
	blockedCard := findRoadmapCard(t, roadmapEpic.Cards, blocked.ID)
	if len(blockedCard.BlockedBy) != 1 || blockedCard.BlockedBy[0].BlockerTitle != blocker.Title {
		t.Fatalf("blocked card dependencies = %#v, want blocker title", blockedCard.BlockedBy)
	}

	updated, err := store.UpdateEpic(ctx, UpdateEpicParams{
		EpicID:      epic.ID,
		Title:       "Roadmap launch plan",
		Description: "Updated plan.",
		Status:      "done",
		StartsOn:    "2026-05-05",
		TargetOn:    "2026-05-20",
	})
	if err != nil {
		t.Fatalf("UpdateEpic returned error: %v", err)
	}
	if updated.Title != "Roadmap launch plan" || updated.Status != "done" || updated.TargetOn != "2026-05-20" {
		t.Fatalf("updated epic = %#v, want changed fields", updated)
	}

	if err := store.DeleteCardDependency(ctx, dependency.ID); err != nil {
		t.Fatalf("DeleteCardDependency returned error: %v", err)
	}
	dashboard, err = store.GetRoadmapDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetRoadmapDashboard after delete returned error: %v", err)
	}
	if got := findRoadmapEpic(t, dashboard, epic.ID).BlockedCards; got != 0 {
		t.Fatalf("blocked count after delete = %d, want 0", got)
	}

	unassigned, err := store.AssignCardToEpic(ctx, AssignCardToEpicParams{CardID: blocked.ID})
	if err != nil {
		t.Fatalf("AssignCardToEpic unassign returned error: %v", err)
	}
	if unassigned.EpicID != "" {
		t.Fatalf("unassigned.EpicID = %q, want empty", unassigned.EpicID)
	}
}

func TestRoadmapValidationRejectsInvalidDependencies(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	planned := findColumn(t, board, "Planned")
	card, err := store.CreateCard(ctx, CreateCardParams{ColumnID: planned.ID, Title: "Cannot block itself"})
	if err != nil {
		t.Fatalf("CreateCard returned error: %v", err)
	}

	if _, err := store.CreateEpic(ctx, CreateEpicParams{TeamID: board.TeamID}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateEpic missing title error = %v, want ErrValidation", err)
	}
	if _, err := store.CreateCardDependency(ctx, CreateCardDependencyParams{BlockedCardID: card.ID, BlockerCardID: card.ID}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateCardDependency self error = %v, want ErrValidation", err)
	}
	if _, err := store.AssignCardToEpic(ctx, AssignCardToEpicParams{CardID: card.ID, EpicID: "missing-epic"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AssignCardToEpic missing epic error = %v, want ErrNotFound", err)
	}
}

func findRoadmapEpic(t *testing.T, dashboard RoadmapDashboard, epicID string) RoadmapEpic {
	t.Helper()
	for _, epic := range dashboard.Epics {
		if epic.Epic.ID == epicID {
			return epic
		}
	}
	t.Fatalf("epic %q not found in %#v", epicID, dashboard.Epics)
	return RoadmapEpic{}
}

func findRoadmapCard(t *testing.T, cards []RoadmapCard, cardID string) RoadmapCard {
	t.Helper()
	for _, card := range cards {
		if card.Card.ID == cardID {
			return card
		}
	}
	t.Fatalf("card %q not found in %#v", cardID, cards)
	return RoadmapCard{}
}

func roadmapHasCard(cards []RoadmapCard, cardID string) bool {
	for _, card := range cards {
		if card.Card.ID == cardID {
			return true
		}
	}
	return false
}
