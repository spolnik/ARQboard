-- +goose Up
CREATE TABLE teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    slug text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, slug)
);

CREATE TABLE team_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (team_id, user_id)
);

CREATE TRIGGER teams_set_updated_at BEFORE UPDATE ON teams FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO teams (id, workspace_id, name, slug)
SELECT gen_random_uuid(), workspaces.id, workspaces.name, workspaces.slug
FROM workspaces;

INSERT INTO team_members (id, team_id, user_id, role)
SELECT gen_random_uuid(), teams.id, workspace_members.user_id, workspace_members.role
FROM teams
JOIN workspace_members ON workspace_members.workspace_id = teams.workspace_id
ON CONFLICT (team_id, user_id) DO NOTHING;

ALTER TABLE boards ADD COLUMN team_id uuid REFERENCES teams(id) ON DELETE CASCADE;

UPDATE boards
SET team_id = teams.id
FROM teams
WHERE teams.workspace_id = boards.workspace_id
  AND teams.slug = (
      SELECT workspaces.slug
      FROM workspaces
      WHERE workspaces.id = boards.workspace_id
  );

ALTER TABLE boards ALTER COLUMN team_id SET NOT NULL;
CREATE INDEX boards_team_id_idx ON boards(team_id);

ALTER TABLE sprints ADD COLUMN team_id uuid REFERENCES teams(id) ON DELETE CASCADE;

UPDATE sprints
SET team_id = boards.team_id
FROM boards
WHERE boards.id = sprints.board_id;

ALTER TABLE sprints ALTER COLUMN team_id SET NOT NULL;

WITH ranked AS (
    SELECT id, row_number() OVER (PARTITION BY team_id, name ORDER BY created_at, id) AS duplicate_index
    FROM sprints
)
UPDATE sprints
SET name = sprints.name || ' ' || ranked.duplicate_index
FROM ranked
WHERE ranked.id = sprints.id
  AND ranked.duplicate_index > 1;

WITH ranked AS (
    SELECT id, row_number() OVER (PARTITION BY team_id ORDER BY started_at DESC NULLS LAST, created_at DESC, id) AS active_index
    FROM sprints
    WHERE status = 'active'
)
UPDATE sprints
SET status = 'planned',
    started_at = NULL
FROM ranked
WHERE ranked.id = sprints.id
  AND ranked.active_index > 1;

DROP INDEX IF EXISTS sprints_one_active_per_board_idx;
CREATE INDEX sprints_team_status_idx ON sprints(team_id, status);
CREATE UNIQUE INDEX sprints_team_name_unique ON sprints(team_id, name);
CREATE UNIQUE INDEX sprints_one_active_per_team_idx ON sprints(team_id) WHERE status = 'active';

-- +goose Down
SELECT 1;
