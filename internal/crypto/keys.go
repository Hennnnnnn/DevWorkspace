// Package crypto provides the client-side E2E primitives for devsync.
//
// Two keypairs per device:
//   - Ed25519 signing key: authenticates every API request (SSH-style).
//   - X25519 box key: receives sealed vault keys (NaCl box).
//
// Vault keys are symmetric (secretbox) and never leave the client in plaintext.
// Private keys at rest are encrypted with a passphrase-derived Argon2id key.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// KeyPair is a device's full key material: an Ed25519 signing pair and an
// X25519 encryption (box) pair.
type KeyPair struct {
	SignPub  ed25519.PublicKey
	SignPriv ed25519.PrivateKey
	BoxPub   [32]byte
	BoxPriv  [32]byte
}

// GenerateKeyPair creates a fresh device key pair.
func GenerateKeyPair() (*KeyPair, error) {
	signPub, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("gen ed25519: %w", err)
	}
	boxPub, boxPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("gen box key: %w", err)
	}
	kp := &KeyPair{SignPub: signPub, SignPriv: signPriv}
	copy(kp.BoxPub[:], boxPub[:])
	copy(kp.BoxPriv[:], boxPriv[:])
	return kp, nil
}

// KeyPairFromPrivates reconstructs a KeyPair from raw private key bytes,
// deriving the public halves. Used by the agent session cache.
func KeyPairFromPrivates(signPriv, boxPriv []byte) (*KeyPair, error) {
	if len(signPriv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("bad signing key length")
	}
	if len(boxPriv) != 32 {
		return nil, fmt.Errorf("bad box key length")
	}
	kp := &KeyPair{
		SignPriv: ed25519.PrivateKey(signPriv),
		SignPub:  ed25519.PrivateKey(signPriv).Public().(ed25519.PublicKey),
	}
	copy(kp.BoxPriv[:], boxPriv)
	curve25519Public(&kp.BoxPub, &kp.BoxPriv)
	return kp, nil
}

// Fingerprint is a short human-verifiable identifier for a signing public key.
// Format: SHA256:base64 (SSH-like), used for out-of-band approval.
func Fingerprint(signPub ed25519.PublicKey) string {
	sum := sha256.Sum256(signPub)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}
