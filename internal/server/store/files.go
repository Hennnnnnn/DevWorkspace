package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrVersionConflict signals an optimistic-lock failure (stale push).
var ErrVersionConflict = errors.New("version conflict: pull first")

// PushFile appends a new version with optimistic locking. baseVersion must equal
// the file's current latest_version (0 for a brand-new file). Returns new version.
func (s *Store) PushFile(ctx context.Context, vaultID, path string, keyVersion, baseVersion int, ciphertext []byte, sizeBytes int, authorDeviceID string, deleted bool) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var fileID string
	var latest int
	err = tx.QueryRowContext(ctx, s.rebind(
		`SELECT id, latest_version FROM files WHERE vault_id=? AND path=?`+s.forUpdate()),
		vaultID, path).Scan(&fileID, &latest)
	if errors.Is(err, sql.ErrNoRows) {
		if baseVersion != 0 {
			return 0, ErrVersionConflict
		}
		fileID = newID()
		if _, err = tx.ExecContext(ctx, s.rebind(
			`INSERT INTO files (id, vault_id, path, latest_version) VALUES (?,?,?,0)`),
			fileID, vaultID, path); err != nil {
			return 0, fmt.Errorf("insert file: %w", err)
		}
		latest = 0
	} else if err != nil {
		return 0, err
	}

	if latest != baseVersion {
		return 0, ErrVersionConflict
	}
	newVersion := latest + 1

	if _, err := tx.ExecContext(ctx, s.rebind(
		`INSERT INTO file_versions (file_id, version, key_version, ciphertext, size_bytes, author_device_id, deleted)
		 VALUES (?,?,?,?,?,?,?)`),
		fileID, newVersion, keyVersion, ciphertext, sizeBytes, authorDeviceID, deleted); err != nil {
		return 0, fmt.Errorf("insert version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, s.rebind(
		`UPDATE files SET latest_version=?, deleted=? WHERE id=?`),
		newVersion, deleted, fileID); err != nil {
		return 0, err
	}
	return newVersion, tx.Commit()
}

// GetFileVersion returns a specific version (or latest if version==0).
func (s *Store) GetFileVersion(ctx context.Context, vaultID, path string, version int) (*FileVersion, error) {
	var fileID string
	var latest int
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT id, latest_version FROM files WHERE vault_id=? AND path=?`), vaultID, path).
		Scan(&fileID, &latest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if version == 0 {
		version = latest
	}
	var fv FileVersion
	var author *string
	err = s.db.QueryRowContext(ctx, s.rebind(
		`SELECT version, key_version, ciphertext, size_bytes, author_device_id, deleted, created_at
		 FROM file_versions WHERE file_id=? AND version=?`), fileID, version).
		Scan(&fv.Version, &fv.KeyVersion, &fv.Ciphertext, &fv.SizeBytes, &author, &fv.Deleted, &fv.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if author != nil {
		fv.AuthorDevice = *author
	}
	return &fv, nil
}

// ListFiles returns file metadata for a vault (includes soft-deleted).
func (s *Store) ListFiles(ctx context.Context, vaultID string) ([]FileMeta, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT id, vault_id, path, latest_version, deleted FROM files
		 WHERE vault_id=? ORDER BY path`), vaultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FileMeta
	for rows.Next() {
		var f FileMeta
		if err := rows.Scan(&f.ID, &f.VaultID, &f.Path, &f.LatestVersion, &f.Deleted); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// History returns all versions of a file, newest first.
func (s *Store) History(ctx context.Context, vaultID, path string) ([]FileVersion, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT fv.version, fv.key_version, fv.size_bytes,
		        COALESCE(fv.author_device_id,''), fv.deleted, fv.created_at
		 FROM file_versions fv
		 JOIN files f ON f.id = fv.file_id
		 WHERE f.vault_id=? AND f.path=?
		 ORDER BY fv.version DESC`), vaultID, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FileVersion
	for rows.Next() {
		var fv FileVersion
		if err := rows.Scan(&fv.Version, &fv.KeyVersion, &fv.SizeBytes, &fv.AuthorDevice, &fv.Deleted, &fv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fv)
	}
	return out, rows.Err()
}
