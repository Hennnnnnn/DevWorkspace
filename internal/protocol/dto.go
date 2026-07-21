package protocol

// Request/response DTOs. Blobs and keys are base64-encoded ([]byte marshals
// to base64 in encoding/json automatically).

type ErrorResponse struct {
	Error string `json:"error"`
}

// --- register / device ---

type RegisterRequest struct {
	Username     string `json:"username"`
	DeviceName   string `json:"device_name"`
	SignPubKey   []byte `json:"sign_pub_key"`
	BoxPubKey    []byte `json:"box_pub_key"`
	Fingerprint  string `json:"fingerprint"`
	// For a second+ device: signature by an existing trusted device over the
	// new device's fingerprint (self-approve, WhatsApp-style linking).
	LinkSignature []byte `json:"link_signature,omitempty"`
	LinkDeviceFingerprint string `json:"link_device_fingerprint,omitempty"`
}

type RegisterResponse struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
	Status   string `json:"status"` // pending | active
}

type Device struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	BoxPubKey   []byte `json:"box_pub_key,omitempty"`
}

type DeviceList struct {
	Devices []Device `json:"devices"`
}

type WhoAmIResponse struct {
	Username string `json:"username"`
	Status   string `json:"status"`
	IsAdmin  bool   `json:"is_admin"`
	Device   Device `json:"device"`
}

// --- teams / members ---

type CreateTeamRequest struct {
	Name string `json:"name"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TeamList struct {
	Teams []Team `json:"teams"`
}

type Member struct {
	Username    string `json:"username"`
	Status      string `json:"status"`
	Fingerprint string `json:"fingerprint"`
	DeviceID    string `json:"device_id"`
	BoxPubKey   []byte `json:"box_pub_key,omitempty"`
}

type MemberList struct {
	Members []Member `json:"members"`
}

type ApproveRequest struct {
	Username    string `json:"username"`
	Fingerprint string `json:"fingerprint"`
	// Vault key shares the admin re-encrypts to the approved user's device.
	Shares []VaultKeyShare `json:"shares,omitempty"`
}

type InviteRequest struct {
	Username string `json:"username"`
	TeamName string `json:"team_name"`
}

type InviteTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type ClaimInviteRequest struct {
	Token string `json:"token"`
}

// --- vaults / grants ---

type CreateVaultRequest struct {
	Team string `json:"team"`
	Name string `json:"name"`
	// Vault key sealed to the creator's own device(s).
	Shares []VaultKeyShare `json:"shares"`
}

type Vault struct {
	ID   string `json:"id"`
	Team string `json:"team"`
	Name string `json:"name"`
}

type VaultList struct {
	Vaults []Vault `json:"vaults"`
}

// VaultKeyShare is a vault key sealed to one device.
type VaultKeyShare struct {
	VaultID      string `json:"vault_id,omitempty"`
	DeviceID     string `json:"device_id"`
	KeyVersion   int    `json:"key_version"`
	EncryptedKey []byte `json:"encrypted_key"`
}

type GrantRequest struct {
	Username string `json:"username"`
	Vault    string `json:"vault"`
	// Sealed vault key shares for each of the grantee's active devices.
	Shares []VaultKeyShare `json:"shares"`
}

type RevokeRequest struct {
	Username string `json:"username,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	Vault    string `json:"vault,omitempty"`
	// After a revoke the caller rotates: new key version sealed to survivors.
	NewKeyVersion int             `json:"new_key_version,omitempty"`
	Shares        []VaultKeyShare `json:"shares,omitempty"`
	// Re-encrypted files under the new key version.
	Files []FilePush `json:"files,omitempty"`
}

// KeySharesResponse returns the caller's sealed vault key for a vault.
type KeySharesResponse struct {
	Shares []VaultKeyShare `json:"shares"`
}

// --- files ---

type FilePush struct {
	Path        string `json:"path"`
	KeyVersion  int    `json:"key_version"`
	Ciphertext  []byte `json:"ciphertext"`
	// BaseVersion is the version the client edited from (optimistic lock).
	// Server rejects if the current latest_version != BaseVersion.
	BaseVersion int `json:"base_version"`
	// Deleted marks this version as a soft-delete tombstone.
	Deleted bool `json:"deleted,omitempty"`
}

type PushRequest struct {
	Vault string   `json:"vault"`
	File  FilePush `json:"file"`
}

type PushResponse struct {
	Version int `json:"version"`
}

type FileMeta struct {
	Path          string `json:"path"`
	LatestVersion int    `json:"latest_version"`
	Deleted       bool   `json:"deleted"`
}

type FileListResponse struct {
	Files []FileMeta `json:"files"`
}

type PullResponse struct {
	Path       string `json:"path"`
	Version    int    `json:"version"`
	KeyVersion int    `json:"key_version"`
	Ciphertext []byte `json:"ciphertext"`
	Deleted    bool   `json:"deleted"`
}

type HistoryEntry struct {
	Version    int    `json:"version"`
	KeyVersion int    `json:"key_version"`
	SizeBytes  int    `json:"size_bytes"`
	Deleted    bool   `json:"deleted"`
	AuthorDevice string `json:"author_device"`
	CreatedAt  string `json:"created_at"`
}

type HistoryResponse struct {
	Entries []HistoryEntry `json:"entries"`
}

// --- audit ---

type AuditEntry struct {
	Username  string `json:"username"`
	Device    string `json:"device"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	CreatedAt string `json:"created_at"`
}

type AuditResponse struct {
	Entries []AuditEntry `json:"entries"`
}
