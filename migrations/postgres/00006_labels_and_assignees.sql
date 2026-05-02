-- +goose Up
CREATE TABLE labels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    color text NOT NULL DEFAULT '#64748b',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX labels_workspace_name_unique ON labels (workspace_id, lower(name));
CREATE INDEX labels_workspace_id_idx ON labels(workspace_id);

CREATE TABLE card_labels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    label_id uuid NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (card_id, label_id)
);

CREATE INDEX card_labels_card_id_idx ON card_labels(card_id);
CREATE INDEX card_labels_label_id_idx ON card_labels(label_id);

CREATE TRIGGER labels_set_updated_at BEFORE UPDATE ON labels FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS labels_set_updated_at ON labels;
DROP TABLE IF EXISTS card_labels;
DROP TABLE IF EXISTS labels;
