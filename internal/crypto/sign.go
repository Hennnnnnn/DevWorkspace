package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
)

// Sign signs a message with the device signing key, returning base64.
func Sign(priv ed25519.PrivateKey, msg []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg))
}

// Verify checks a base64 signature against the message and public key.
func Verify(pub ed25519.PublicKey, msg []byte, sigB64 string) bool {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, msg, sig)
}
