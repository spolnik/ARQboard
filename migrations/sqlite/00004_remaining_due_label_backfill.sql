-- +goose Up
UPDATE cards
SET due_at = '2026-05-01'
WHERE due_at IS NULL
  AND due_label = 'Now';

UPDATE cards
SET due_at = '2026-05-29'
WHERE due_at IS NULL
  AND due_label <> ''
  AND due_label NOT GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]';

UPDATE cards
SET due_label = ''
WHERE due_at IS NOT NULL
  AND due_label <> '';

-- +goose Down
SELECT 1;
