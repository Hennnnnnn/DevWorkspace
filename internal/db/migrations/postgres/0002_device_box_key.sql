-- +goose Up
-- +goose StatementBegin
-- Each device has two keys: ed25519 for signing (public_key), X25519 for
-- encryption (box_public_key). Vault keys are sealed to box_public_key.
ALTER TABLE devices ADD COLUMN box_public_key BYTEA NOT NULL DEFAULT ''::bytea;
ALTER TABLE devices ALTER COLUMN box_public_key DROP DEFAULT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE devices DROP COLUMN box_public_key;
-- +goose StatementEnd
