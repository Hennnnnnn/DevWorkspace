package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateTeam(ctx context.Context, name string) (*Team, error) {
	var t Team
	t.Name = name
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO teams (name) VALUES ($1) RETURNING id`, name).Scan(&t.ID)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) GetTeamByName(ctx context.Context, name string) (*Team, error) {
	var t Team
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name FROM teams WHERE name=$1`, name).Scan(&t.ID, &t.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// AddTeamMember inserts (or no-ops) a membership row.
func (s *Store) AddTeamMember(ctx context.Context, teamID, userID, status string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, status) VALUES ($1,$2,$3)
		 ON CONFLICT (team_id, user_id) DO UPDATE SET status=EXCLUDED.status`,
		teamID, userID, status)
	return err
}

// ListTeamsForUser returns teams the user is an active member of.
func (s *Store) ListTeamsForUser(ctx context.Context, userID string) ([]Team, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT t.id, t.name FROM teams t
		 JOIN team_members m ON m.team_id = t.id
		 WHERE m.user_id=$1 AND m.status='active' ORDER BY t.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListMembers returns members of a team; pendingOnly filters to pending status.
func (s *Store) ListMembers(ctx context.Context, teamID string, pendingOnly bool) ([]Member, error) {
	q := `SELECT u.username, m.status, d.fingerprint, d.id, d.box_public_key
	      FROM team_members m
	      JOIN users u ON u.id = m.user_id
	      LEFT JOIN devices d ON d.user_id = u.id
	      WHERE m.team_id=$1`
	if pendingOnly {
		q += ` AND m.status='pending'`
	}
	q += ` ORDER BY u.username`
	rows, err := s.Pool.Query(ctx, q, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		var fp, did *string
		var box []byte
		if err := rows.Scan(&m.Username, &m.Status, &fp, &did, &box); err != nil {
			return nil, err
		}
		if fp != nil {
			m.Fingerprint = *fp
		}
		if did != nil {
			m.DeviceID = *did
		}
		m.BoxPubKey = box
		out = append(out, m)
	}
	return out, rows.Err()
}

// Member mirrors protocol.Member but lives here to avoid importing protocol in store.
type Member struct {
	Username    string
	Status      string
	Fingerprint string
	DeviceID    string
	BoxPubKey   []byte
}
