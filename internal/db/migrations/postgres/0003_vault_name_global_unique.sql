-- +goose Up
-- +goose StatementBegin
ALTER TABLE vaults DROP CONSTRAINT vaults_team_id_name_key;
ALTER TABLE vaults ADD UNIQUE (name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE vaults DROP CONSTRAINT vaults_name_key;
ALTER TABLE vaults ADD UNIQUE (team_id, name);
-- +goose StatementEnd
