// Package agent caches an unlocked device key for a configurable timeout so the
// user types their passphrase once (ssh-agent style).
//
// ponytail: cross-platform simple design — the unlocked key bundle is written to
// a 0600 session file under ~/.devsync/agent.session with an expiry timestamp.
// Ceiling: key sits in a temp file, not a locked-memory daemon. Upgrade path:
// replace with a background process holding keys in mlock'd memory + unix socket
// / named pipe. Fine for a self-hosted single-user CLI today.
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/devsync/devsync/internal/client/config"
	"github.com/devsync/devsync/internal/client/keystore"
	"github.com/devsync/devsync/internal/crypto"
)

const sessionFile = "agent.session"

type session struct {
	ExpiresAt int64  `json:"expires_at"`
	SignPriv  []byte `json:"sign_priv"`
	BoxPriv   []byte `json:"box_priv"`
}

func sessionPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionFile), nil
}

// Unlock decrypts the device key and caches it for `ttl`.
func Unlock(passphrase string, ttl time.Duration) error {
	kp, err := keystore.Unlock(passphrase)
	if err != nil {
		return err
	}
	s := session{
		ExpiresAt: time.Now().Add(ttl).Unix(),
		SignPriv:  kp.SignPriv,
		BoxPriv:   kp.BoxPriv[:],
	}
	data, _ := json.Marshal(s)
	p, err := sessionPath()
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Lock removes the cached session.
func Lock() error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Cached returns the unlocked keypair if a live session exists, else nil.
func Cached() (*crypto.KeyPair, bool) {
	p, err := sessionPath()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var s session
	if json.Unmarshal(data, &s) != nil {
		return nil, false
	}
	if time.Now().Unix() > s.ExpiresAt {
		_ = os.Remove(p)
		return nil, false
	}
	kp, err := crypto.KeyPairFromPrivates(s.SignPriv, s.BoxPriv)
	if err != nil {
		return nil, false
	}
	return kp, true
}

// Get returns the unlocked keypair from the agent, or an instructive error.
func Get() (*crypto.KeyPair, error) {
	if kp, ok := Cached(); ok {
		return kp, nil
	}
	return nil, fmt.Errorf("locked — run `devsync unlock` first")
}
