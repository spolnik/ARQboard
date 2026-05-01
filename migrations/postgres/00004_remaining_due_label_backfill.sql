-- +goose Up
UPDATE cards
SET due_at = '2026-05-01'::timestamptz
WHERE due_at IS NULL
  AND due_label = 'Now';

UPDATE cards
SET due_at = '2026-05-29'::timestamptz
WHERE due_at IS NULL
  AND due_label <> ''
  AND due_label !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$';

UPDATE cards
SET due_label = ''
WHERE due_at IS NOT NULL
  AND due_label <> '';

-- +goose Down
SELECT 1;
