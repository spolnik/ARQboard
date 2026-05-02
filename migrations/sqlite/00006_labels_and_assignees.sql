-- +goose Up
CREATE TABLE labels (
    id uuid PRIMARY KEY NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    color text NOT NULL DEFAULT '#64748b',
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX labels_workspace_name_unique ON labels (workspace_id, lower(name));
CREATE INDEX labels_workspace_id_idx ON labels(workspace_id);

CREATE TABLE card_labels (
    id uuid PRIMARY KEY NOT NULL,
    card_id uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    label_id uuid NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    created_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (card_id, label_id)
);

CREATE INDEX card_labels_card_id_idx ON card_labels(card_id);
CREATE INDEX card_labels_label_id_idx ON card_labels(label_id);

-- +goose Down
DROP TABLE IF EXISTS card_labels;
DROP TABLE IF EXISTS labels;
