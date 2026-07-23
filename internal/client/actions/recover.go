package actions

import (
	"fmt"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
)

// Recover regenerates the device keypair from the 24-word recovery phrase and
// re-registers with the server. The server recognises the fingerprint and
// reactivates the existing device row (account-centric recovery: no manual
// activation, no link signature). Fails if a keystore already exists on this
// machine — use Login or remove the keystore first.
func Recover(username, mnemonic, passphrase string) (*RegisterResult, error) {
	if keystore.Exists() {
		return nil, fmt.Errorf("device key already exists — use Login, or remove the keystore to recover")
	}
	if len(passphrase) < 8 {
		return nil, fmt.Errorf("passphrase must be at least 8 characters")
	}
	seed, err := crypto.MnemonicToSeed(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("invalid recovery phrase: %w", err)
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
	// Re-register: server sees the original fingerprint and reactivates the
	// existing device row (status active, same DeviceID).
	return Register(username, "", passphrase)
}