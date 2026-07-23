-- +goose Up
-- +goose StatementBegin

-- Global user activation removed (room-based ownership: every authenticated
-- user is usable immediately, no admin approval). Register now writes
-- status='active' explicitly, so the column default is no longer exercised.
-- Promote any legacy pending users left from the old global-admin model.
UPDATE users SET status = 'active' WHERE status = 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No-op: we cannot tell which users were genuinely pending vs. promoted, so
-- the down migration does not revert the promotion.
-- +goose StatementEnd