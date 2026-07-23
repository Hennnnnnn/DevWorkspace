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
// Used by register (status active — room-based ownership, no global approval).
func (s *Store) CreateUserWithDevice(ctx context.Context, u User, d Device) (userID, deviceID string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	userID = newID()
	if _, err = tx.ExecContext(ctx, s.rebind(
		`INSERT INTO users (id, username, status) VALUES (?,?,?)`),
		userID, u.Username, u.Status); err != nil {
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
		`SELECT id, username, status FROM users WHERE username=?`), username).
		Scan(&u.ID, &u.Username, &u.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetDeviceByFingerprint returns the device and its owning user.
func (s *Store) GetDeviceByFingerprint(ctx context.Context, fp string) (*Device, *User, error) {
	var d Device
	var u User
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT d.id, d.user_id, d.name, d.public_key, d.box_public_key, d.fingerprint, d.status,
		        u.id, u.username, u.status
		 FROM devices d JOIN users u ON u.id = d.user_id
		 WHERE d.fingerprint=?`), fp).
		Scan(&d.ID, &d.UserID, &d.Name, &d.SignPubKey, &d.BoxPubKey, &d.Fingerprint, &d.Status,
			&u.ID, &u.Username, &u.Status)
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
