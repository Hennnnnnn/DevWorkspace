package protocol

import (
	"strings"
	"testing"
)

func TestSigningStringDeterministic(t *testing.T) {
	a := SigningString("get", "/vaults", "hash", 100)
	b := SigningString("GET", "/vaults", "hash", 100)
	if string(a) != string(b) {
		t.Fatal("method case not normalized")
	}
	parts := strings.Split(string(a), "\n")
	if len(parts) != 4 || parts[0] != "GET" || parts[3] != "100" {
		t.Fatalf("unexpected signing string: %q", a)
	}
}

func TestBodyHashDistinct(t *testing.T) {
	if BodyHash([]byte("a")) == BodyHash([]byte("b")) {
		t.Fatal("different bodies share hash")
	}
	if BodyHash(nil) != BodyHash([]byte{}) {
		t.Fatal("nil and empty body hash differ")
	}
}
