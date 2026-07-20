package store

import "context"

// Log records a metadata-only audit event (no plaintext, stays E2E).
func (s *Store) Log(ctx context.Context, userID, deviceID, vaultID, action, target string) error {
	var uid, did, vid *string
	if userID != "" {
		uid = &userID
	}
	if deviceID != "" {
		did = &deviceID
	}
	if vaultID != "" {
		vid = &vaultID
	}
	_, err := s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO audit_log (user_id, device_id, vault_id, action, target)
		 VALUES (?,?,?,?,?)`), uid, did, vid, action, target)
	return err
}

// Audit returns the audit trail for a vault, newest first.
func (s *Store) Audit(ctx context.Context, vaultID string) ([]AuditRow, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT COALESCE(u.username,''), COALESCE(d.fingerprint,''), a.action,
		        COALESCE(a.target,''), a.created_at
		 FROM audit_log a
		 LEFT JOIN users u ON u.id = a.user_id
		 LEFT JOIN devices d ON d.id = a.device_id
		 WHERE a.vault_id=? ORDER BY a.created_at DESC LIMIT 500`), vaultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.Username, &a.Device, &a.Action, &a.Target, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
