package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) CreateTeam(ctx context.Context, name string) (*Team, error) {
	t := Team{ID: newID(), Name: name}
	_, err := s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO teams (id, name) VALUES (?,?)`), t.ID, name)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

const creatorSubquery = `COALESCE((SELECT u.username FROM audit_log a
		 JOIN users u ON u.id = a.user_id
		 WHERE a.target = t.name AND a.action = 'create_team'
		 ORDER BY a.created_at DESC LIMIT 1), '')`

func (s *Store) GetTeamByName(ctx context.Context, name string) (*Team, error) {
	var t Team
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT id, name, `+creatorSubquery+` FROM teams t WHERE name=?`), name).Scan(&t.ID, &t.Name, &t.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteTeam removes a team by name. CASCADE handles vaults, files, members.
func (s *Store) DeleteTeam(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM teams WHERE name=?`), name)
	return err
}

// AddTeamMember inserts (or no-ops) a membership row.
func (s *Store) AddTeamMember(ctx context.Context, teamID, userID, status, role string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO team_members (team_id, user_id, status, role) VALUES (?,?,?,?)
		 ON CONFLICT (team_id, user_id) DO UPDATE SET status=EXCLUDED.status, role=EXCLUDED.role`),
		teamID, userID, status, role)
	return err
}

// ListTeamsWithRoleForUser returns teams the user belongs to with their role.
func (s *Store) ListTeamsWithRoleForUser(ctx context.Context, userID string) ([]Team, []string, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT t.id, t.name, `+creatorSubquery+`, m.role FROM teams t
		 JOIN team_members m ON m.team_id = t.id
		 WHERE m.user_id=? AND m.status='active'
		 ORDER BY t.name`), userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var teams []Team
	var roles []string
	for rows.Next() {
		var t Team
		var role string
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &role); err != nil {
			return nil, nil, err
		}
		teams = append(teams, t)
		roles = append(roles, role)
	}
	return teams, roles, rows.Err()
}

// IsTeamAdmin checks if a user is an active admin of the given team.
func (s *Store) IsTeamAdmin(ctx context.Context, teamID, userID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT 1 FROM team_members WHERE team_id=? AND user_id=? AND role='admin' AND status='active'`),
		teamID, userID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// SetTeamMemberRole updates a team member's role.
func (s *Store) SetTeamMemberRole(ctx context.Context, teamID, userID, role string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(
		`UPDATE team_members SET role=? WHERE team_id=? AND user_id=?`), role, teamID, userID)
	return err
}

func (s *Store) ListAllTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT id, name, `+creatorSubquery+` FROM teams t ORDER BY name`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListTeamsForUser returns teams the user is a member of, filtered by status
// (empty=all, "active", "pending").
func (s *Store) ListTeamsForUser(ctx context.Context, userID, status string) ([]Team, error) {
	q := `SELECT t.id, t.name, ` + creatorSubquery + ` FROM teams t
		 JOIN team_members m ON m.team_id = t.id
		 WHERE m.user_id=?`
	if status != "" {
		q += ` AND m.status=?`
	}
	q += ` ORDER BY t.name`
	args := []interface{}{userID}
	if status != "" {
		args = append(args, status)
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListMembers returns members of a team; pendingOnly filters to pending status.
func (s *Store) ListMembers(ctx context.Context, teamID string, pendingOnly bool) ([]Member, error) {
	q := `SELECT u.username, m.status, m.role, d.fingerprint, d.id, d.box_public_key
	      FROM team_members m
	      JOIN users u ON u.id = m.user_id
	      LEFT JOIN devices d ON d.user_id = u.id
	      WHERE m.team_id=?`
	if pendingOnly {
		q += ` AND m.status='pending'`
	}
	q += ` ORDER BY u.username`
	rows, err := s.db.QueryContext(ctx, s.rebind(q), teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		var fp, did *string
		var box []byte
		if err := rows.Scan(&m.Username, &m.Status, &m.Role, &fp, &did, &box); err != nil {
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
	Role        string
	Fingerprint string
	DeviceID    string
	BoxPubKey   []byte
}
