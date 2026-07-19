package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/nacl/secretbox"
)

// argon2 params — interactive-grade, tuned for a CLI unlock.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32
)

// EncryptedKey is the on-disk, passphrase-protected device key material.
type EncryptedKey struct {
	Version    int    `json:"v"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ct"` // secretbox of the plaintext key bundle
}

// keyBundle is the plaintext serialized inside EncryptedKey.Ciphertext.
type keyBundle struct {
	SignPriv []byte `json:"sign_priv"`
	BoxPriv  []byte `json:"box_priv"`
}

func deriveKey(passphrase string, salt []byte) [32]byte {
	dk := argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	var k [32]byte
	copy(k[:], dk)
	return k
}

// EncryptPrivateKey seals the device's private keys under a passphrase.
func EncryptPrivateKey(kp *KeyPair, passphrase string) (*EncryptedKey, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("gen salt: %w", err)
	}
	bundle, err := json.Marshal(keyBundle{
		SignPriv: kp.SignPriv,
		BoxPriv:  kp.BoxPriv[:],
	})
	if err != nil {
		return nil, err
	}
	key := deriveKey(passphrase, salt)
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("gen nonce: %w", err)
	}
	ct := secretbox.Seal(nil, bundle, &nonce, &key)
	return &EncryptedKey{Version: 1, Salt: salt, Nonce: nonce[:], Ciphertext: ct}, nil
}

// DecryptPrivateKey reconstructs the KeyPair from an EncryptedKey + passphrase.
// The signing public key is derived from the private key.
func DecryptPrivateKey(ek *EncryptedKey, passphrase string) (*KeyPair, error) {
	if len(ek.Nonce) != 24 {
		return nil, fmt.Errorf("bad nonce length")
	}
	key := deriveKey(passphrase, ek.Salt)
	var nonce [24]byte
	copy(nonce[:], ek.Nonce)
	plain, ok := secretbox.Open(nil, ek.Ciphertext, &nonce, &key)
	if !ok {
		return nil, fmt.Errorf("wrong passphrase or corrupt key file")
	}
	var b keyBundle
	if err := json.Unmarshal(plain, &b); err != nil {
		return nil, err
	}
	if len(b.SignPriv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("bad signing key length")
	}
	kp := &KeyPair{
		SignPriv: ed25519.PrivateKey(b.SignPriv),
		SignPub:  ed25519.PrivateKey(b.SignPriv).Public().(ed25519.PublicKey),
	}
	copy(kp.BoxPriv[:], b.BoxPriv)
	curve25519Public(&kp.BoxPub, &kp.BoxPriv)
	return kp, nil
}
