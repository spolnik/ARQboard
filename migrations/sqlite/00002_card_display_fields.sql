-- +goose Up
ALTER TABLE cards ADD COLUMN owner_initials text NOT NULL DEFAULT '';
ALTER TABLE cards ADD COLUMN due_label text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE cards DROP COLUMN due_label;
ALTER TABLE cards DROP COLUMN owner_initials;
