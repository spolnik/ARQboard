-- +goose Up
CREATE TEMP TABLE board_team_reassignments (
    board_id uuid PRIMARY KEY NOT NULL,
    old_team_id uuid NOT NULL,
    new_team_id uuid NOT NULL
);

INSERT INTO board_team_reassignments (board_id, old_team_id, new_team_id)
SELECT
    id,
    team_id,
    lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6)))
FROM (
    SELECT id, team_id, row_number() OVER (PARTITION BY team_id ORDER BY created_at, id) AS board_rank
    FROM boards
    WHERE team_id IS NOT NULL
)
WHERE board_rank > 1;

INSERT INTO teams (id, workspace_id, name, slug)
SELECT
    board_team_reassignments.new_team_id,
    boards.workspace_id,
    boards.name,
    boards.slug || '-team-' || substr(board_team_reassignments.new_team_id, 1, 8)
FROM board_team_reassignments
JOIN boards ON boards.id = board_team_reassignments.board_id;

INSERT OR IGNORE INTO team_members (id, team_id, user_id, role)
SELECT
    lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))),
    board_team_reassignments.new_team_id,
    team_members.user_id,
    team_members.role
FROM board_team_reassignments
JOIN team_members ON team_members.team_id = board_team_reassignments.old_team_id;

DELETE FROM card_dependencies
WHERE blocked_card_id IN (
    SELECT cards.id
    FROM cards
    JOIN board_team_reassignments ON board_team_reassignments.board_id = cards.board_id
)
OR blocker_card_id IN (
    SELECT cards.id
    FROM cards
    JOIN board_team_reassignments ON board_team_reassignments.board_id = cards.board_id
);

UPDATE cards
SET epic_id = NULL
WHERE board_id IN (SELECT board_id FROM board_team_reassignments);

UPDATE sprints
SET team_id = (
    SELECT new_team_id
    FROM board_team_reassignments
    WHERE board_team_reassignments.board_id = sprints.board_id
)
WHERE board_id IN (SELECT board_id FROM board_team_reassignments);

UPDATE boards
SET team_id = (
    SELECT new_team_id
    FROM board_team_reassignments
    WHERE board_team_reassignments.board_id = boards.id
)
WHERE id IN (SELECT board_id FROM board_team_reassignments);

CREATE TEMP TABLE team_board_backfills (
    team_id uuid PRIMARY KEY NOT NULL,
    board_id uuid NOT NULL
);

INSERT INTO team_board_backfills (team_id, board_id)
SELECT
    teams.id,
    lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6)))
FROM teams
WHERE NOT EXISTS (
    SELECT 1
    FROM boards
    WHERE boards.team_id = teams.id
);

INSERT INTO boards (id, workspace_id, team_id, name, slug, description)
SELECT
    team_board_backfills.board_id,
    teams.workspace_id,
    teams.id,
    teams.name,
    teams.slug || '-board-' || substr(team_board_backfills.board_id, 1, 8),
    'Team-owned board.'
FROM team_board_backfills
JOIN teams ON teams.id = team_board_backfills.team_id;

INSERT INTO columns (id, board_id, name, system_key, position)
SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))), board_id, 'Planned', 'planned', 0
FROM team_board_backfills
UNION ALL
SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))), board_id, 'In progress', 'in_progress', 1
FROM team_board_backfills
UNION ALL
SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))), board_id, 'Ready for review', 'ready_for_review', 2
FROM team_board_backfills
UNION ALL
SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))), board_id, 'Done', 'done', 3
FROM team_board_backfills;

CREATE UNIQUE INDEX boards_team_id_unique ON boards(team_id);

DROP TABLE team_board_backfills;
DROP TABLE board_team_reassignments;

-- +goose Down
SELECT 1;
