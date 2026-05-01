-- +goose Up
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS sprints_one_active_per_workspace_idx;
DROP INDEX IF EXISTS sprints_workspace_status_idx;

CREATE TABLE sprints_new (
    id uuid PRIMARY KEY NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    board_id uuid NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name text NOT NULL,
    goal text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed')),
    starts_on text,
    ends_on text,
    started_at text,
    completed_at text,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (board_id, name)
);

INSERT INTO sprints_new (
    id,
    workspace_id,
    board_id,
    name,
    goal,
    status,
    starts_on,
    ends_on,
    started_at,
    completed_at,
    created_at,
    updated_at
)
SELECT
    sprints.id,
    sprints.workspace_id,
    COALESCE(
        (SELECT cards.board_id FROM cards WHERE cards.sprint_id = sprints.id ORDER BY cards.created_at, cards.id LIMIT 1),
        (SELECT boards.id FROM boards WHERE boards.workspace_id = sprints.workspace_id ORDER BY boards.created_at, boards.id LIMIT 1)
    ),
    sprints.name,
    sprints.goal,
    sprints.status,
    sprints.starts_on,
    sprints.ends_on,
    sprints.started_at,
    sprints.completed_at,
    sprints.created_at,
    sprints.updated_at
FROM sprints
WHERE COALESCE(
    (SELECT cards.board_id FROM cards WHERE cards.sprint_id = sprints.id ORDER BY cards.created_at, cards.id LIMIT 1),
    (SELECT boards.id FROM boards WHERE boards.workspace_id = sprints.workspace_id ORDER BY boards.created_at, boards.id LIMIT 1)
) IS NOT NULL;

DROP TABLE sprints;
ALTER TABLE sprints_new RENAME TO sprints;

CREATE INDEX sprints_workspace_status_idx ON sprints(workspace_id, status);
CREATE INDEX sprints_board_status_idx ON sprints(board_id, status);
CREATE UNIQUE INDEX sprints_one_active_per_board_idx ON sprints(board_id) WHERE status = 'active';

INSERT INTO sprints (
    id,
    workspace_id,
    board_id,
    name,
    goal,
    status,
    started_at
)
SELECT
    lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))),
    boards.workspace_id,
    boards.id,
    'Current sprint',
    'Migrated active sprint for existing board cards.',
    'active',
    datetime('now')
FROM boards
WHERE EXISTS (
    SELECT 1 FROM cards WHERE cards.board_id = boards.id AND cards.sprint_id IS NULL
)
AND NOT EXISTS (
    SELECT 1 FROM sprints WHERE sprints.board_id = boards.id AND sprints.status = 'active'
);

UPDATE cards
SET sprint_id = (
    SELECT sprints.id
    FROM sprints
    WHERE sprints.board_id = cards.board_id
      AND sprints.status = 'active'
    ORDER BY sprints.started_at DESC, sprints.created_at DESC, sprints.id
    LIMIT 1
)
WHERE sprint_id IS NULL
  AND EXISTS (
      SELECT 1
      FROM sprints
      WHERE sprints.board_id = cards.board_id
        AND sprints.status = 'active'
  );

PRAGMA foreign_keys = ON;

-- +goose Down
SELECT 1;
