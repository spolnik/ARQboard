-- +goose Up
UPDATE users
SET display_name = CASE
    WHEN instr(email, '@') > 1 THEN substr(email, 1, instr(email, '@') - 1)
    ELSE email
END
WHERE trim(display_name) = ''
   OR lower(trim(display_name)) = lower(email);

-- +goose Down
-- Display name backfills are intentionally not reverted.
