-- +goose Up
-- +goose StatementBegin

-- Users. Global identity. status: pending until active (via invite token or bootstrap first-user).
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'active', 'disabled')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Devices. One keypair per device. public_key stored, server verifies signatures.
-- status: pending until approved (first device by admin w/ fingerprint, later by trusted device).
CREATE TABLE devices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    public_key  BYTEA NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'active', 'revoked')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);
CREATE INDEX idx_devices_user ON devices(user_id);

-- Teams.
CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Team membership. role: admin or member within this team. Team admin manages team vaults, invites, etc.
CREATE TABLE team_members (
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'active')),
    role        TEXT NOT NULL DEFAULT 'member'
                    CHECK (role IN ('admin', 'member')),
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

-- Vaults. Belong to a team. Symmetric vault key lives client-side only.
CREATE TABLE vaults (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (team_id, name)
);

-- Vault access grant, per-user. The vault key encrypted to each of the user's
-- active devices is stored in vault_key_shares below.
CREATE TABLE vault_grants (
    vault_id    UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (vault_id, user_id)
);

-- Vault key encrypted to a specific device public key (E2E: server never sees plaintext key).
-- key_version bumps on rotation (revoke -> rotate).
CREATE TABLE vault_key_shares (
    vault_id      UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    device_id     UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    key_version   INT  NOT NULL DEFAULT 1,
    encrypted_key BYTEA NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (vault_id, device_id, key_version)
);

-- Files: logical file within a vault. Latest version pointer + soft delete.
CREATE TABLE files (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vault_id       UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    path           TEXT NOT NULL,
    latest_version INT  NOT NULL DEFAULT 0,
    deleted        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (vault_id, path)
);

-- File versions: encrypted blob, versioned for history + rollback (optimistic lock via version).
CREATE TABLE file_versions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id      UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    version      INT  NOT NULL,
    key_version  INT  NOT NULL,
    ciphertext   BYTEA NOT NULL,
    size_bytes   INT  NOT NULL,
    author_device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
    deleted      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_id, version)
);

-- Audit log: metadata only (no plaintext, stays E2E).
CREATE TABLE audit_log (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    device_id  UUID REFERENCES devices(id) ON DELETE SET NULL,
    vault_id   UUID REFERENCES vaults(id) ON DELETE SET NULL,
    action     TEXT NOT NULL,
    target     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
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
