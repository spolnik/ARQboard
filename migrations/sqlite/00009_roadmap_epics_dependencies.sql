-- +goose Up
CREATE TABLE epics (
    id uuid PRIMARY KEY NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    title text NOT NULL,
    slug text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'done')),
    starts_on text,
    target_on text,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (team_id, slug)
);

ALTER TABLE cards ADD COLUMN epic_id uuid REFERENCES epics(id) ON DELETE SET NULL;

CREATE TABLE card_dependencies (
    id uuid PRIMARY KEY NOT NULL,
    blocked_card_id uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    blocker_card_id uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    relation_type text NOT NULL DEFAULT 'blocks' CHECK (relation_type IN ('blocks')),
    created_at text NOT NULL DEFAULT (datetime('now')),
    CHECK (blocked_card_id <> blocker_card_id),
    UNIQUE (blocked_card_id, blocker_card_id, relation_type)
);

CREATE INDEX epics_team_status_idx ON epics(team_id, status);
CREATE INDEX epics_team_target_idx ON epics(team_id, target_on);
CREATE INDEX cards_epic_id_idx ON cards(epic_id);
CREATE INDEX card_dependencies_blocked_idx ON card_dependencies(blocked_card_id);
CREATE INDEX card_dependencies_blocker_idx ON card_dependencies(blocker_card_id);

-- +goose Down
SELECT 1;
