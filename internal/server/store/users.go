package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// CreateUserWithDevice inserts a new user and their first device in one tx.
// Used by register (status pending) and create-admin (active + admin).
func (s *Store) CreateUserWithDevice(ctx context.Context, u User, d Device) (userID, deviceID string, err error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO users (username, status, is_admin) VALUES ($1,$2,$3) RETURNING id`,
		u.Username, u.Status, u.IsAdmin).Scan(&userID)
	if err != nil {
		return "", "", fmt.Errorf("insert user: %w", err)
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO devices (user_id, name, public_key, box_public_key, fingerprint, status)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		userID, d.Name, d.SignPubKey, d.BoxPubKey, d.Fingerprint, d.Status).Scan(&deviceID)
	if err != nil {
		return "", "", fmt.Errorf("insert device: %w", err)
	}
	return userID, deviceID, tx.Commit(ctx)
}

// AddDevice inserts an additional device for an existing user.
func (s *Store) AddDevice(ctx context.Context, d Device) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO devices (user_id, name, public_key, box_public_key, fingerprint, status)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		d.UserID, d.Name, d.SignPubKey, d.BoxPubKey, d.Fingerprint, d.Status).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert device: %w", err)
	}
	return id, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`SELECT id, username, status, is_admin FROM users WHERE username=$1`, username).
		Scan(&u.ID, &u.Username, &u.Status, &u.IsAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) SetUserStatus(ctx context.Context, userID, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET status=$2 WHERE id=$1`, userID, status)
	return err
}

// GetDeviceByFingerprint returns the device and its owning user.
func (s *Store) GetDeviceByFingerprint(ctx context.Context, fp string) (*Device, *User, error) {
	var d Device
	var u User
	err := s.Pool.QueryRow(ctx,
		`SELECT d.id, d.user_id, d.name, d.public_key, d.box_public_key, d.fingerprint, d.status,
		        u.id, u.username, u.status, u.is_admin
		 FROM devices d JOIN users u ON u.id = d.user_id
		 WHERE d.fingerprint=$1`, fp).
		Scan(&d.ID, &d.UserID, &d.Name, &d.SignPubKey, &d.BoxPubKey, &d.Fingerprint, &d.Status,
			&u.ID, &u.Username, &u.Status, &u.IsAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &d, &u, nil
}

// ListDevices returns all devices for a user.
func (s *Store) ListDevices(ctx context.Context, userID string) ([]Device, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, user_id, name, public_key, box_public_key, fingerprint, status
		 FROM devices WHERE user_id=$1 ORDER BY created_at`, userID)
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
	q := `UPDATE devices SET status=$2 WHERE id=$1`
	if status == "revoked" {
		q = `UPDATE devices SET status=$2, revoked_at=now() WHERE id=$1`
	}
	_, err := s.Pool.Exec(ctx, q, deviceID, status)
	return err
}

// CountUsers returns total user count (bootstrap admin race check).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

// CountAdmins returns the number of admin users.
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin=TRUE`).Scan(&n)
	return n, err
}

// SetUserAdmin sets a user's admin flag.
func (s *Store) SetUserAdmin(ctx context.Context, userID string, isAdmin bool) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET is_admin=$2 WHERE id=$1`, userID, isAdmin)
	return err
}
