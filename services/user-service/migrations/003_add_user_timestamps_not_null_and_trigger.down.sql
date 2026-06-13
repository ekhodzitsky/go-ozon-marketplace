DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;

ALTER TABLE users
    ALTER COLUMN created_at DROP NOT NULL,
    ALTER COLUMN updated_at DROP NOT NULL;

ALTER TABLE users DROP COLUMN IF EXISTS updated_at;
