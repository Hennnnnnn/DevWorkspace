package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateInviteToken(ctx context.Context, token, teamID, username, createdBy string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO invite_tokens (token, team_id, username, created_by, expires_at) VALUES (?,?,?,?,?)`),
		token, teamID, username, createdBy, expiresAt)
	return err
}

// ClaimInviteToken atomically: validates token, marks used, activates user+device,
// adds to team as active member. All in one transaction.
func (s *Store) ClaimInviteToken(ctx context.Context, token, username, userID, deviceID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var t InviteToken
	var usedBy, usedAt *sql.NullString
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, s.rebind(
		`SELECT token, team_id, username, expires_at, used_by, used_at
		 FROM invite_tokens WHERE token=?`), token).
		Scan(&t.Token, &t.TeamID, &t.Username, &expiresAt, &usedBy, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup token: %w", err)
	}
	if usedBy != nil && usedBy.Valid {
		return fmt.Errorf("token already used")
	}
	if time.Now().After(expiresAt) {
		return fmt.Errorf("token expired")
	}
	if t.Username != username {
		return fmt.Errorf("token not for this user")
	}

	_, err = tx.ExecContext(ctx, s.rebind(
		`UPDATE invite_tokens SET used_by=?, used_at=? WHERE token=?`),
		userID, time.Now(), token)
	if err != nil {
		return fmt.Errorf("mark used: %w", err)
	}

	_, err = tx.ExecContext(ctx, s.rebind(
		`UPDATE users SET status='active' WHERE id=? AND status='pending'`), userID)
	if err != nil {
		return fmt.Errorf("activate user: %w", err)
	}

	_, err = tx.ExecContext(ctx, s.rebind(
		`UPDATE devices SET status='active' WHERE id=? AND status='pending'`), deviceID)
	if err != nil {
		return fmt.Errorf("activate device: %w", err)
	}

	_, err = tx.ExecContext(ctx, s.rebind(
		`INSERT INTO team_members (team_id, user_id, status, role) VALUES (?,?,?,?)
		 ON CONFLICT (team_id, user_id) DO UPDATE SET status=EXCLUDED.status, role=EXCLUDED.role`),
		t.TeamID, userID, "active", "member")
	if err != nil {
		return fmt.Errorf("add team member: %w", err)
	}

	return tx.Commit()
}

func (s *Store) GetInviteToken(ctx context.Context, token string) (*InviteToken, error) {
	var t InviteToken
	var usedBy sql.NullString
	var usedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT token, team_id, username, created_by, expires_at, used_by, used_at
		 FROM invite_tokens WHERE token=?`), token).
		Scan(&t.Token, &t.TeamID, &t.Username, &t.CreatedBy, &t.ExpiresAt, &usedBy, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if usedBy.Valid {
		t.UsedBy = &usedBy.String
	}
	if usedAt.Valid {
		t.UsedAt = &usedAt.Time
	}
	return &t, nil
}
