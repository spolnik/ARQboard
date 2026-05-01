-- +goose Up
ALTER TABLE sprints ADD COLUMN board_id uuid REFERENCES boards(id) ON DELETE CASCADE;

UPDATE sprints
SET board_id = COALESCE(
    (SELECT cards.board_id FROM cards WHERE cards.sprint_id = sprints.id ORDER BY cards.created_at, cards.id LIMIT 1),
    (SELECT boards.id FROM boards WHERE boards.workspace_id = sprints.workspace_id ORDER BY boards.created_at, boards.id LIMIT 1)
);

DELETE FROM sprints WHERE board_id IS NULL;

ALTER TABLE sprints ALTER COLUMN board_id SET NOT NULL;
ALTER TABLE sprints DROP CONSTRAINT IF EXISTS sprints_workspace_id_name_key;
ALTER TABLE sprints ADD CONSTRAINT sprints_board_id_name_key UNIQUE (board_id, name);

DROP INDEX IF EXISTS sprints_one_active_per_workspace_idx;
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
    gen_random_uuid(),
    boards.workspace_id,
    boards.id,
    'Current sprint',
    'Migrated active sprint for existing board cards.',
    'active',
    now()
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

-- +goose Down
SELECT 1;
