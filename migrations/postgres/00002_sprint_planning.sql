-- +goose Up
CREATE TABLE sprints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    goal text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed')),
    starts_on date,
    ends_on date,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE INDEX sprints_workspace_status_idx ON sprints(workspace_id, status);
CREATE UNIQUE INDEX sprints_one_active_per_workspace_idx ON sprints(workspace_id) WHERE status = 'active';
CREATE TRIGGER sprints_set_updated_at BEFORE UPDATE ON sprints FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE cards ADD COLUMN sprint_id uuid REFERENCES sprints(id) ON DELETE SET NULL;
CREATE INDEX cards_sprint_id_idx ON cards(sprint_id);

-- +goose Down
DROP INDEX IF EXISTS cards_sprint_id_idx;
ALTER TABLE cards DROP COLUMN IF EXISTS sprint_id;
DROP TRIGGER IF EXISTS sprints_set_updated_at ON sprints;
DROP INDEX IF EXISTS sprints_one_active_per_workspace_idx;
DROP INDEX IF EXISTS sprints_workspace_status_idx;
DROP TABLE IF EXISTS sprints;
