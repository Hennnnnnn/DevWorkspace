package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"math/big"
	"strings"

	"golang.org/x/crypto/hkdf"
)

//go:embed bip39_words.txt
var bip39WordsRaw string

var bip39WordList []string
var bip39WordMap map[string]uint16

func init() {
	bip39WordList = strings.Split(strings.TrimSpace(bip39WordsRaw), "\n")
	bip39WordMap = make(map[string]uint16, len(bip39WordList))
	for i, w := range bip39WordList {
		bip39WordMap[w] = uint16(i)
	}
}

// GenerateRecoverySeed produces 32 random bytes for recovery key derivation.
func GenerateRecoverySeed() ([]byte, error) {
	seed := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, seed); err != nil {
		return nil, fmt.Errorf("gen recovery seed: %w", err)
	}
	return seed, nil
}

// SeedToMnemonic encodes a 32-byte recovery seed as a 24-word BIP39 mnemonic.
func SeedToMnemonic(seed []byte) (string, error) {
	if len(seed) != 32 {
		return "", fmt.Errorf("seed must be 32 bytes, got %d", len(seed))
	}
	checksum := sha256.Sum256(seed)

	data := make([]byte, 33)
	copy(data, seed)
	data[32] = checksum[0]

	var n big.Int
	n.SetBytes(data)

	words := make([]string, 24)
	mask := big.NewInt(0x7FF)
	for i := 23; i >= 0; i-- {
		var idx big.Int
		idx.And(&n, mask)
		words[i] = bip39WordList[idx.Int64()]
		n.Rsh(&n, 11)
	}
	return strings.Join(words, " "), nil
}

// MnemonicToSeed decodes a 24-word BIP39 mnemonic back to the 32-byte seed.
// Returns an error if the phrase is invalid or the checksum does not match.
func MnemonicToSeed(phrase string) ([]byte, error) {
	words := strings.Fields(strings.ToLower(phrase))
	if len(words) != 24 {
		return nil, fmt.Errorf("mnemonic must be 24 words, got %d", len(words))
	}

	var n big.Int
	for _, w := range words {
		idx, ok := bip39WordMap[w]
		if !ok {
			return nil, fmt.Errorf("unknown word: %s", w)
		}
		n.Lsh(&n, 11)
		n.Or(&n, big.NewInt(int64(idx)))
	}

	data := n.Bytes()
	padded := make([]byte, 33)
	copy(padded[33-len(data):], data)

	seed := padded[:32]
	checksum := padded[32]

	h := sha256.Sum256(seed)
	if h[0] != checksum {
		return nil, fmt.Errorf("invalid mnemonic (checksum mismatch)")
	}
	return seed, nil
}

// DeriveKeyPairFromSeed deterministically derives an Ed25519 + X25519 key pair
// from a 32-byte recovery seed using HKDF-SHA256.
func DeriveKeyPairFromSeed(seed []byte) (*KeyPair, error) {
	if len(seed) != 32 {
		return nil, fmt.Errorf("seed must be 32 bytes, got %d", len(seed))
	}

	edReader := hkdf.New(sha256.New, seed, []byte("devsync-ed25519"), []byte("ed25519-seed"))
	edSeed := make([]byte, ed25519.SeedSize)
	if _, err := io.ReadFull(edReader, edSeed); err != nil {
		return nil, fmt.Errorf("derive ed25519 seed: %w", err)
	}

	boxReader := hkdf.New(sha256.New, seed, []byte("devsync-x25519"), []byte("x25519-seed"))
	boxSeed := make([]byte, 32)
	if _, err := io.ReadFull(boxReader, boxSeed); err != nil {
		return nil, fmt.Errorf("derive x25519 seed: %w", err)
	}

	signPriv := ed25519.NewKeyFromSeed(edSeed)
	kp := &KeyPair{
		SignPriv: signPriv,
		SignPub:  signPriv.Public().(ed25519.PublicKey),
	}
	copy(kp.BoxPriv[:], boxSeed)
	curve25519Public(&kp.BoxPub, &kp.BoxPriv)
	return kp, nil
}
