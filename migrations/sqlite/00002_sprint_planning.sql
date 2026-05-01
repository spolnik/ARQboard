-- +goose Up
CREATE TABLE sprints (
    id uuid PRIMARY KEY NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    goal text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed')),
    starts_on text,
    ends_on text,
    started_at text,
    completed_at text,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (workspace_id, name)
);

CREATE INDEX sprints_workspace_status_idx ON sprints(workspace_id, status);
CREATE UNIQUE INDEX sprints_one_active_per_workspace_idx ON sprints(workspace_id) WHERE status = 'active';

ALTER TABLE cards ADD COLUMN sprint_id uuid REFERENCES sprints(id) ON DELETE SET NULL;
CREATE INDEX cards_sprint_id_idx ON cards(sprint_id);

-- +goose Down
DROP INDEX IF EXISTS cards_sprint_id_idx;
ALTER TABLE cards DROP COLUMN sprint_id;
DROP INDEX IF EXISTS sprints_one_active_per_workspace_idx;
DROP INDEX IF EXISTS sprints_workspace_status_idx;
DROP TABLE IF EXISTS sprints;
