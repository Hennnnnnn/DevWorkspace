package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) CreateVault(ctx context.Context, teamID, name string, shares []KeyShare) (*Vault, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	v := Vault{ID: newID(), TeamID: teamID, Name: name}
	if _, err := tx.ExecContext(ctx, s.rebind(
		`INSERT INTO vaults (id, team_id, name) VALUES (?,?,?)`), v.ID, teamID, name); err != nil {
		return nil, err
	}
	for _, sh := range shares {
		if _, err := tx.ExecContext(ctx, s.rebind(
			`INSERT INTO vault_key_shares (vault_id, device_id, key_version, encrypted_key)
			 VALUES (?,?,?,?)`), v.ID, sh.DeviceID, sh.KeyVersion, sh.EncryptedKey); err != nil {
			return nil, err
		}
	}
	return &v, tx.Commit()
}

// GetVault resolves a vault by team name + vault name.
func (s *Store) GetVault(ctx context.Context, teamName, vaultName string) (*Vault, error) {
	var v Vault
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT v.id, v.team_id, v.name FROM vaults v
		 JOIN teams t ON t.id = v.team_id
		 WHERE t.name=? AND v.name=?`), teamName, vaultName).
		Scan(&v.ID, &v.TeamID, &v.Name)
	if errors.Is(err, sql.ErrNoRows) {
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
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT id, team_id, name FROM vaults WHERE name=? LIMIT 1`), name).
		Scan(&v.ID, &v.TeamID, &v.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) HasGrant(ctx context.Context, vaultID, userID string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT 1 FROM vault_grants WHERE vault_id=? AND user_id=?`), vaultID, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Grant inserts a grant and the sealed key shares for the grantee's devices.
func (s *Store) Grant(ctx context.Context, vaultID, userID string, shares []KeyShare) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, s.rebind(
		`INSERT INTO vault_grants (vault_id, user_id) VALUES (?,?)
		 ON CONFLICT DO NOTHING`), vaultID, userID); err != nil {
		return err
	}
	for _, sh := range shares {
		if _, err := tx.ExecContext(ctx, s.rebind(
			`INSERT INTO vault_key_shares (vault_id, device_id, key_version, encrypted_key)
			 VALUES (?,?,?,?) ON CONFLICT DO NOTHING`),
			vaultID, sh.DeviceID, sh.KeyVersion, sh.EncryptedKey); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AddKeyShares inserts sealed shares (used on approve + rotate).
func (s *Store) AddKeyShares(ctx context.Context, shares []KeyShare) error {
	for _, sh := range shares {
		if _, err := s.db.ExecContext(ctx, s.rebind(
			`INSERT INTO vault_key_shares (vault_id, device_id, key_version, encrypted_key)
			 VALUES (?,?,?,?) ON CONFLICT DO NOTHING`),
			sh.VaultID, sh.DeviceID, sh.KeyVersion, sh.EncryptedKey); err != nil {
			return err
		}
	}
	return nil
}

// RevokeGrant removes a user's grant and their key shares for the vault.
func (s *Store) RevokeGrant(ctx context.Context, vaultID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, s.rebind(
		`DELETE FROM vault_grants WHERE vault_id=? AND user_id=?`), vaultID, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, s.rebind(
		`DELETE FROM vault_key_shares WHERE vault_id=? AND device_id IN
		 (SELECT id FROM devices WHERE user_id=?)`), vaultID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// GetKeySharesForDevice returns the sealed vault keys for one device in a vault.
func (s *Store) GetKeySharesForDevice(ctx context.Context, vaultID, deviceID string) ([]KeyShare, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT vault_id, device_id, key_version, encrypted_key
		 FROM vault_key_shares WHERE vault_id=? AND device_id=?
		 ORDER BY key_version`), vaultID, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanShares(rows)
}

// ListVaultsForUser returns vaults the user has a grant on.
func (s *Store) ListVaultsForUser(ctx context.Context, userID string) ([]Vault, []string, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT v.id, v.team_id, v.name, t.name FROM vaults v
		 JOIN vault_grants g ON g.vault_id = v.id
		 JOIN teams t ON t.id = v.team_id
		 WHERE g.user_id=? ORDER BY t.name, v.name`), userID)
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

func scanShares(rows *sql.Rows) ([]KeyShare, error) {
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
