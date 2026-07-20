-- +goose Up
-- +goose StatementBegin
-- Squashed final schema for SQLite (equivalent of postgres 0001+0002+0003).
-- IDs are TEXT, generated Go-side (no gen_random_uuid). box_public_key and
-- global-unique vault name are baked in.

CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    username    TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'active', 'disabled')),
    is_admin    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE devices (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    public_key     BLOB NOT NULL,
    box_public_key BLOB NOT NULL,
    fingerprint    TEXT NOT NULL UNIQUE,
    status         TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'active', 'revoked')),
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at     TIMESTAMP
);
CREATE INDEX idx_devices_user ON devices(user_id);

CREATE TABLE teams (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE team_members (
    team_id     TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'active')),
    joined_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (team_id, user_id)
);

-- Vault name is globally unique (postgres migration 0003).
CREATE TABLE vaults (
    id          TEXT PRIMARY KEY,
    team_id     TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name        TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE vault_grants (
    vault_id    TEXT NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (vault_id, user_id)
);

CREATE TABLE vault_key_shares (
    vault_id      TEXT NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    device_id     TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    key_version   INTEGER NOT NULL DEFAULT 1,
    encrypted_key BLOB NOT NULL,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (vault_id, device_id, key_version)
);

CREATE TABLE files (
    id             TEXT PRIMARY KEY,
    vault_id       TEXT NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    path           TEXT NOT NULL,
    latest_version INTEGER NOT NULL DEFAULT 0,
    deleted        INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (vault_id, path)
);

CREATE TABLE file_versions (
    id           INTEGER PRIMARY KEY,
    file_id      TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    version      INTEGER NOT NULL,
    key_version  INTEGER NOT NULL,
    ciphertext   BLOB NOT NULL,
    size_bytes   INTEGER NOT NULL,
    author_device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    deleted      INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (file_id, version)
);

CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY,
    user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    device_id  TEXT REFERENCES devices(id) ON DELETE SET NULL,
    vault_id   TEXT REFERENCES vaults(id) ON DELETE SET NULL,
    action     TEXT NOT NULL,
    target     TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_vault ON audit_log(vault_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS file_versions;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS vault_key_shares;
DROP TABLE IF EXISTS vault_grants;
DROP TABLE IF EXISTS vaults;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
