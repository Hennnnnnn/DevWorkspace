package actions

import (
	"fmt"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/api"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
)

// InitResult is the outcome of generating a new device key.
type InitResult struct {
	Mnemonic    string // 24-word recovery phrase — shown exactly once
	Fingerprint string
}

// InitDevice generates a device keypair protected by passphrase and saves it
// to the keystore. Fails if a key already exists (no silent overwrite).
func InitDevice(passphrase string) (*InitResult, error) {
	if keystore.Exists() {
		return nil, fmt.Errorf("device key already exists")
	}
	if len(passphrase) < 8 {
		return nil, fmt.Errorf("passphrase must be at least 8 characters")
	}
	seed, err := crypto.GenerateRecoverySeed()
	if err != nil {
		return nil, err
	}
	kp, err := crypto.DeriveKeyPairFromSeed(seed)
	if err != nil {
		return nil, err
	}
	ek, err := crypto.EncryptPrivateKey(kp, passphrase)
	if err != nil {
		return nil, err
	}
	if err := keystore.Save(ek); err != nil {
		return nil, err
	}
	mnemonic, err := crypto.SeedToMnemonic(seed)
	if err != nil {
		return nil, err
	}
	return &InitResult{Mnemonic: mnemonic, Fingerprint: crypto.Fingerprint(kp.SignPub)}, nil
}

// BootstrapAdmin promotes the registered user to admin. Only succeeds when no
// admin exists yet on the server (first user).
func BootstrapAdmin(passphrase string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Username == "" || cfg.ServerURL == "" {
		return fmt.Errorf("register first — no username or server_url in config")
	}
	kp, err := keystore.Unlock(passphrase)
	if err != nil {
		return err
	}
	req := map[string]string{
		"username":    cfg.Username,
		"fingerprint": crypto.Fingerprint(kp.SignPub),
	}
	return api.PostUnsigned(cfg.ServerURL, "/admin/bootstrap", req, nil)
}
