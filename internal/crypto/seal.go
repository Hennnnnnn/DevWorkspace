package crypto

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// VaultKey is a 32-byte symmetric key that encrypts a vault's file blobs.
type VaultKey [32]byte

// NewVaultKey generates a random vault key.
func NewVaultKey() (VaultKey, error) {
	var k VaultKey
	if _, err := rand.Read(k[:]); err != nil {
		return k, fmt.Errorf("gen vault key: %w", err)
	}
	return k, nil
}

// SealVaultKey encrypts a vault key to a recipient device's box public key
// using an anonymous sealed box (sender key is ephemeral).
func SealVaultKey(k VaultKey, recipientBoxPub [32]byte) ([]byte, error) {
	sealed, err := box.SealAnonymous(nil, k[:], &recipientBoxPub, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("seal vault key: %w", err)
	}
	return sealed, nil
}

// OpenVaultKey decrypts a sealed vault key with the device's box keypair.
func OpenVaultKey(sealed []byte, boxPub, boxPriv [32]byte) (VaultKey, error) {
	var k VaultKey
	out, ok := box.OpenAnonymous(nil, sealed, &boxPub, &boxPriv)
	if !ok {
		return k, fmt.Errorf("open vault key: decrypt failed")
	}
	if len(out) != 32 {
		return k, fmt.Errorf("open vault key: bad length %d", len(out))
	}
	copy(k[:], out)
	return k, nil
}

// EncryptBlob encrypts plaintext with a vault key (secretbox), nonce prepended.
func EncryptBlob(k VaultKey, plaintext []byte) ([]byte, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("gen nonce: %w", err)
	}
	key := [32]byte(k)
	return secretbox.Seal(nonce[:], plaintext, &nonce, &key), nil
}

// DecryptBlob reverses EncryptBlob.
func DecryptBlob(k VaultKey, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 24 {
		return nil, fmt.Errorf("ciphertext too short")
	}
	var nonce [24]byte
	copy(nonce[:], ciphertext[:24])
	key := [32]byte(k)
	out, ok := secretbox.Open(nil, ciphertext[24:], &nonce, &key)
	if !ok {
		return nil, fmt.Errorf("decrypt blob: authentication failed")
	}
	return out, nil
}
