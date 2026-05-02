-- +goose Up
UPDATE users
SET display_name = split_part(email, '@', 1)
WHERE btrim(display_name) = ''
   OR lower(btrim(display_name)) = lower(email);

-- +goose Down
-- Display name backfills are intentionally not reverted.
