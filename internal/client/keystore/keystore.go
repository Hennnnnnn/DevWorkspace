// Package keystore persists and loads the device's passphrase-encrypted keys
// under ~/.devsync/. Private keys never leave this file in plaintext.
package keystore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
)

const keyFile = "device.key"

func keyPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, keyFile), nil
}

// Exists reports whether a device key file is already present.
func Exists() bool {
	p, err := keyPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Save writes the encrypted key file with 0600 perms.
func Save(ek *crypto.EncryptedKey) error {
	p, err := keyPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(ek, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// LoadEncrypted reads the on-disk encrypted key (still locked).
func LoadEncrypted() (*crypto.EncryptedKey, error) {
	p, err := keyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("no device key — run `devsync init` first")
	}
	if err != nil {
		return nil, err
	}
	var ek crypto.EncryptedKey
	if err := json.Unmarshal(data, &ek); err != nil {
		return nil, err
	}
	return &ek, nil
}

// Unlock decrypts the key file with the passphrase.
func Unlock(passphrase string) (*crypto.KeyPair, error) {
	ek, err := LoadEncrypted()
	if err != nil {
		return nil, err
	}
	return crypto.DecryptPrivateKey(ek, passphrase)
}
