package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func mustKP(t *testing.T) *KeyPair {
	t.Helper()
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return kp
}

func TestSignVerify(t *testing.T) {
	kp := mustKP(t)
	msg := []byte("GET\n/vaults\nabc\n123")
	sig := Sign(kp.SignPriv, msg)
	if !Verify(kp.SignPub, msg, sig) {
		t.Fatal("valid signature rejected")
	}
	if Verify(kp.SignPub, []byte("tampered"), sig) {
		t.Fatal("tampered message accepted")
	}
	other := mustKP(t)
	if Verify(other.SignPub, msg, sig) {
		t.Fatal("signature verified under wrong key")
	}
}

func TestSealOpenVaultKey(t *testing.T) {
	recipient := mustKP(t)
	vk, err := NewVaultKey()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := SealVaultKey(vk, recipient.BoxPub)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenVaultKey(sealed, recipient.BoxPub, recipient.BoxPriv)
	if err != nil {
		t.Fatal(err)
	}
	if got != vk {
		t.Fatal("vault key mismatch after seal/open")
	}
	// Wrong recipient cannot open.
	wrong := mustKP(t)
	if _, err := OpenVaultKey(sealed, wrong.BoxPub, wrong.BoxPriv); err == nil {
		t.Fatal("wrong recipient opened sealed key")
	}
}

func TestEncryptDecryptBlob(t *testing.T) {
	vk, _ := NewVaultKey()
	plain := []byte("DATABASE_URL=postgres://secret")
	ct, err := EncryptBlob(vk, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, plain) {
		t.Fatal("plaintext leaked into ciphertext")
	}
	got, err := DecryptBlob(vk, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("blob mismatch")
	}
	// Tampered ciphertext fails auth.
	ct[len(ct)-1] ^= 0xff
	if _, err := DecryptBlob(vk, ct); err == nil {
		t.Fatal("tampered blob decrypted")
	}
}

func TestPrivateKeyRoundTrip(t *testing.T) {
	kp := mustKP(t)
	ek, err := EncryptPrivateKey(kp, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptPrivateKey(ek, "wrong pass"); err == nil {
		t.Fatal("wrong passphrase accepted")
	}
	got, err := DecryptPrivateKey(ek, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !got.SignPriv.Equal(kp.SignPriv) {
		t.Fatal("signing key mismatch after decrypt")
	}
	if got.BoxPriv != kp.BoxPriv || got.BoxPub != kp.BoxPub {
		t.Fatal("box key mismatch after decrypt")
	}
	// Derived signing pub must round-trip and verify.
	sig := Sign(got.SignPriv, []byte("x"))
	if !Verify(got.SignPub, []byte("x"), sig) {
		t.Fatal("derived signing pub failed verify")
	}
}

func TestMnemonicRoundTrip(t *testing.T) {
	seed, err := GenerateRecoverySeed()
	if err != nil {
		t.Fatal(err)
	}
	mnemonic, err := SeedToMnemonic(seed)
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(mnemonic)
	if len(words) != 24 {
		t.Fatalf("expected 24 words, got %d", len(words))
	}
	got, err := MnemonicToSeed(mnemonic)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(seed, got) {
		t.Fatal("seed mismatch after mnemonic round-trip")
	}
}

func TestMnemonicInvalidWord(t *testing.T) {
	mnemonic := strings.Repeat("abandon ", 23) + "notaword"
	if _, err := MnemonicToSeed(mnemonic); err == nil {
		t.Fatal("expected error for invalid word")
	}
}

func TestMnemonicWrongChecksum(t *testing.T) {
	seed, _ := GenerateRecoverySeed()
	mnemonic, _ := SeedToMnemonic(seed)
	words := strings.Fields(mnemonic)
	words[23] = "zoo"
	if _, err := MnemonicToSeed(strings.Join(words, " ")); err == nil {
		t.Fatal("expected checksum error for tampered mnemonic")
	}
}

func TestMnemonicWrongLength(t *testing.T) {
	if _, err := MnemonicToSeed("abandon abandon abandon"); err == nil {
		t.Fatal("expected error for wrong word count")
	}
}

func TestDeriveKeyPairFromSeedDeterministic(t *testing.T) {
	seed, _ := GenerateRecoverySeed()
	kp1, err := DeriveKeyPairFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	kp2, err := DeriveKeyPairFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	if !kp1.SignPriv.Equal(kp2.SignPriv) || kp1.BoxPriv != kp2.BoxPriv {
		t.Fatal("same seed produced different keys")
	}
	if Fingerprint(kp1.SignPub) != Fingerprint(kp2.SignPub) {
		t.Fatal("same seed produced different fingerprints")
	}
}

func TestDeriveKeyPairFromSeedDistinct(t *testing.T) {
	s1, _ := GenerateRecoverySeed()
	s2, _ := GenerateRecoverySeed()
	kp1, _ := DeriveKeyPairFromSeed(s1)
	kp2, _ := DeriveKeyPairFromSeed(s2)
	if Fingerprint(kp1.SignPub) == Fingerprint(kp2.SignPub) {
		t.Fatal("different seeds produced same fingerprint")
	}
}

func TestDeriveKeyPairFromSeedBadLength(t *testing.T) {
	if _, err := DeriveKeyPairFromSeed([]byte("too short")); err == nil {
		t.Fatal("expected error for short seed")
	}
}

func TestFullRecoveryFlow(t *testing.T) {
	seed, _ := GenerateRecoverySeed()
	kp, _ := DeriveKeyPairFromSeed(seed)

	ek, err := EncryptPrivateKey(kp, "recovery pass")
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecryptPrivateKey(ek, "recovery pass")
	if err != nil {
		t.Fatal(err)
	}
	if !got.SignPriv.Equal(kp.SignPriv) || got.BoxPriv != kp.BoxPriv {
		t.Fatal("recovery key pair did not survive encrypt/decrypt round-trip")
	}
}

func TestFingerprintStableAndDistinct(t *testing.T) {
	kp := mustKP(t)
	fp := Fingerprint(kp.SignPub)
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("unexpected fingerprint format: %s", fp)
	}
	if fp != Fingerprint(kp.SignPub) {
		t.Fatal("fingerprint not stable")
	}
	if fp == Fingerprint(mustKP(t).SignPub) {
		t.Fatal("distinct keys share fingerprint")
	}
}
