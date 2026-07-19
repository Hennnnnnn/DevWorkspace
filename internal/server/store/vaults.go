package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateVault(ctx context.Context, teamID, name string, shares []KeyShare) (*Vault, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var v Vault
	v.TeamID, v.Name = teamID, name
	err = tx.QueryRow(ctx,
		`INSERT INTO vaults (team_id, name) VALUES ($1,$2) RETURNING id`, teamID, name).Scan(&v.ID)
	if err != nil {
		return nil, err
	}
	for _, sh := range shares {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vault_key_shares (vault_id, device_id, key_version, encrypted_key)
			 VALUES ($1,$2,$3,$4)`, v.ID, sh.DeviceID, sh.KeyVersion, sh.EncryptedKey); err != nil {
			return nil, err
		}
	}
	return &v, tx.Commit(ctx)
}

// GetVault resolves a vault by team name + vault name.
func (s *Store) GetVault(ctx context.Context, teamName, vaultName string) (*Vault, error) {
	var v Vault
	err := s.Pool.QueryRow(ctx,
		`SELECT v.id, v.team_id, v.name FROM vaults v
		 JOIN teams t ON t.id = v.team_id
		 WHERE t.name=$1 AND v.name=$2`, teamName, vaultName).
		Scan(&v.ID, &v.TeamID, &v.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// GetVaultByName resolves a vault by name alone (unique per team; picks first).
// Used by commands that pass --vault without team context.
func (s *Store) GetVaultByName(ctx context.Context, name string) (*Vault, error) {
	var v Vault
	err := s.Pool.QueryRow(ctx,
		`SELECT id, team_id, name FROM vaults WHERE name=$1 LIMIT 1`, name).
		Scan(&v.ID, &v.TeamID, &v.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) HasGrant(ctx context.Context, vaultID, userID string) (bool, error) {
	var one int
	err := s.Pool.QueryRow(ctx,
		`SELECT 1 FROM vault_grants WHERE vault_id=$1 AND user_id=$2`, vaultID, userID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Grant inserts a grant and the sealed key shares for the grantee's devices.
func (s *Store) Grant(ctx context.Context, vaultID, userID string, shares []KeyShare) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO vault_grants (vault_id, user_id) VALUES ($1,$2)
		 ON CONFLICT DO NOTHING`, vaultID, userID); err != nil {
		return err
	}
	for _, sh := range shares {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vault_key_shares (vault_id, device_id, key_version, encrypted_key)
			 VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			vaultID, sh.DeviceID, sh.KeyVersion, sh.EncryptedKey); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AddKeyShares inserts sealed shares (used on approve + rotate).
func (s *Store) AddKeyShares(ctx context.Context, shares []KeyShare) error {
	for _, sh := range shares {
		if _, err := s.Pool.Exec(ctx,
			`INSERT INTO vault_key_shares (vault_id, device_id, key_version, encrypted_key)
			 VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			sh.VaultID, sh.DeviceID, sh.KeyVersion, sh.EncryptedKey); err != nil {
			return err
		}
	}
	return nil
}

// RevokeGrant removes a user's grant and their key shares for the vault.
func (s *Store) RevokeGrant(ctx context.Context, vaultID, userID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`DELETE FROM vault_grants WHERE vault_id=$1 AND user_id=$2`, vaultID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM vault_key_shares WHERE vault_id=$1 AND device_id IN
		 (SELECT id FROM devices WHERE user_id=$2)`, vaultID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetKeySharesForDevice returns the sealed vault keys for one device in a vault.
func (s *Store) GetKeySharesForDevice(ctx context.Context, vaultID, deviceID string) ([]KeyShare, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT vault_id, device_id, key_version, encrypted_key
		 FROM vault_key_shares WHERE vault_id=$1 AND device_id=$2
		 ORDER BY key_version`, vaultID, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanShares(rows)
}

// ListVaultsForUser returns vaults the user has a grant on.
func (s *Store) ListVaultsForUser(ctx context.Context, userID string) ([]Vault, []string, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT v.id, v.team_id, v.name, t.name FROM vaults v
		 JOIN vault_grants g ON g.vault_id = v.id
		 JOIN teams t ON t.id = v.team_id
		 WHERE g.user_id=$1 ORDER BY t.name, v.name`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var vaults []Vault
	var teams []string
	for rows.Next() {
		var v Vault
		var team string
		if err := rows.Scan(&v.ID, &v.TeamID, &v.Name, &team); err != nil {
			return nil, nil, err
		}
		vaults = append(vaults, v)
		teams = append(teams, team)
	}
	return vaults, teams, rows.Err()
}

func scanShares(rows pgx.Rows) ([]KeyShare, error) {
	var out []KeyShare
	for rows.Next() {
		var sh KeyShare
		if err := rows.Scan(&sh.VaultID, &sh.DeviceID, &sh.KeyVersion, &sh.EncryptedKey); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}
