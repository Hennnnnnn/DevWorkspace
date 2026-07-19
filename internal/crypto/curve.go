package crypto

import "golang.org/x/crypto/curve25519"

// curve25519Public derives the X25519 public key from a private scalar.
func curve25519Public(pub, priv *[32]byte) {
	pk, _ := curve25519.X25519(priv[:], curve25519.Basepoint)
	copy(pub[:], pk)
}
