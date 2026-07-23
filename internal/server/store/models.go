package store

import "time"

type User struct {
	ID       string
	Username string
	Status   string
}

type Device struct {
	ID          string
	UserID      string
	Name        string
	SignPubKey  []byte
	BoxPubKey   []byte
	Fingerprint string
	Status      string
}

type Team struct {
	ID        string
	Name      string
	CreatedBy string
}

type Vault struct {
	ID     string
	TeamID string
	Name   string
}

type FileMeta struct {
	ID            string
	VaultID       string
	Path          string
	LatestVersion int
	Deleted       bool
}

type FileVersion struct {
	Version      int
	KeyVersion   int
	Ciphertext   []byte
	SizeBytes    int
	AuthorDevice string
	Deleted      bool
	CreatedAt    time.Time
}

type KeyShare struct {
	VaultID      string
	DeviceID     string
	KeyVersion   int
	EncryptedKey []byte
}

type InviteToken struct {
	Token     string
	TeamID    string
	Username  string
	CreatedBy string
	ExpiresAt time.Time
	UsedBy    *string
	UsedAt    *time.Time
}

type AuditRow struct {
	Username  string
	Device    string
	Action    string
	Target    string
	CreatedAt time.Time
}
