-- +goose Up
UPDATE cards
SET due_at = CASE due_label
    WHEN 'Apr 30' THEN '2026-04-30'::timestamptz
    WHEN 'Today' THEN '2026-05-01'::timestamptz
    WHEN 'May 2' THEN '2026-05-02'::timestamptz
    WHEN 'May 3' THEN '2026-05-03'::timestamptz
    WHEN 'Soon' THEN '2026-05-08'::timestamptz
    WHEN 'Next' THEN '2026-05-15'::timestamptz
    WHEN 'Later' THEN '2026-05-29'::timestamptz
    ELSE due_at
END
WHERE due_at IS NULL
  AND due_label IN ('Apr 30', 'Today', 'May 2', 'May 3', 'Soon', 'Next', 'Later');

UPDATE cards
SET due_label = ''
WHERE due_at IS NOT NULL
  AND due_label IN ('Apr 30', 'Today', 'May 2', 'May 3', 'Soon', 'Next', 'Later');

-- +goose Down
SELECT 1;
