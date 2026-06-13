ALTER TABLE events
    DROP COLUMN IF EXISTS amount,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS occurred_at;
