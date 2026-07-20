package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// CreateUserWithDevice inserts a new user and their first device in one tx.
// Used by register (status pending) and create-admin (active + admin).
func (s *Store) CreateUserWithDevice(ctx context.Context, u User, d Device) (userID, deviceID string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	userID = newID()
	if _, err = tx.ExecContext(ctx, s.rebind(
		`INSERT INTO users (id, username, status, is_admin) VALUES (?,?,?,?)`),
		userID, u.Username, u.Status, u.IsAdmin); err != nil {
		return "", "", fmt.Errorf("insert user: %w", err)
	}
	deviceID = newID()
	if _, err = tx.ExecContext(ctx, s.rebind(
		`INSERT INTO devices (id, user_id, name, public_key, box_public_key, fingerprint, status)
		 VALUES (?,?,?,?,?,?,?)`),
		deviceID, userID, d.Name, d.SignPubKey, d.BoxPubKey, d.Fingerprint, d.Status); err != nil {
		return "", "", fmt.Errorf("insert device: %w", err)
	}
	return userID, deviceID, tx.Commit()
}

// AddDevice inserts an additional device for an existing user.
func (s *Store) AddDevice(ctx context.Context, d Device) (string, error) {
	id := newID()
	_, err := s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO devices (id, user_id, name, public_key, box_public_key, fingerprint, status)
		 VALUES (?,?,?,?,?,?,?)`),
		id, d.UserID, d.Name, d.SignPubKey, d.BoxPubKey, d.Fingerprint, d.Status)
	if err != nil {
		return "", fmt.Errorf("insert device: %w", err)
	}
	return id, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT id, username, status, is_admin FROM users WHERE username=?`), username).
		Scan(&u.ID, &u.Username, &u.Status, &u.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) SetUserStatus(ctx context.Context, userID, status string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE users SET status=? WHERE id=?`), status, userID)
	return err
}

// GetDeviceByFingerprint returns the device and its owning user.
func (s *Store) GetDeviceByFingerprint(ctx context.Context, fp string) (*Device, *User, error) {
	var d Device
	var u User
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT d.id, d.user_id, d.name, d.public_key, d.box_public_key, d.fingerprint, d.status,
		        u.id, u.username, u.status, u.is_admin
		 FROM devices d JOIN users u ON u.id = d.user_id
		 WHERE d.fingerprint=?`), fp).
		Scan(&d.ID, &d.UserID, &d.Name, &d.SignPubKey, &d.BoxPubKey, &d.Fingerprint, &d.Status,
			&u.ID, &u.Username, &u.Status, &u.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &d, &u, nil
}

// ListDevices returns all devices for a user.
func (s *Store) ListDevices(ctx context.Context, userID string) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT id, user_id, name, public_key, box_public_key, fingerprint, status
		 FROM devices WHERE user_id=? ORDER BY created_at`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.SignPubKey, &d.BoxPubKey, &d.Fingerprint, &d.Status); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListActiveDevices returns active devices for a user (targets for key sealing).
func (s *Store) ListActiveDevices(ctx context.Context, userID string) ([]Device, error) {
	all, err := s.ListDevices(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []Device
	for _, d := range all {
		if d.Status == "active" {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *Store) SetDeviceStatus(ctx context.Context, deviceID, status string) error {
	q := `UPDATE devices SET status=? WHERE id=?`
	if status == "revoked" {
		q = `UPDATE devices SET status=?, revoked_at=CURRENT_TIMESTAMP WHERE id=?`
	}
	_, err := s.db.ExecContext(ctx, s.rebind(q), status, deviceID)
	return err
}

// CountUsers returns total user count (bootstrap admin race check).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

// CountAdmins returns the number of admin users.
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE is_admin=TRUE`).Scan(&n)
	return n, err
}

// SetUserAdmin sets a user's admin flag.
func (s *Store) SetUserAdmin(ctx context.Context, userID string, isAdmin bool) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE users SET is_admin=? WHERE id=?`), isAdmin, userID)
	return err
}
