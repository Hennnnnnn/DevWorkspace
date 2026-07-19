// Package protocol defines the wire contract shared by client and server:
// the canonical request-signing string, auth header names, and JSON DTOs.
package protocol

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// Auth header names. Every authenticated request carries these.
const (
	HeaderUser      = "X-Devsync-User"
	HeaderDevice    = "X-Devsync-Device" // device fingerprint
	HeaderTimestamp = "X-Devsync-Timestamp"
	HeaderSignature = "X-Devsync-Signature"
)

// MaxSkew is the anti-replay window: requests older/newer than this are rejected.
const MaxSkewSeconds = 300

// BodyHash returns the base64 SHA-256 of the request body ("" hash for empty).
func BodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// SigningString builds the canonical string that gets signed/verified.
// Fields are newline-joined and order-fixed so both sides agree byte-for-byte.
func SigningString(method, path string, bodyHash string, unixTs int64) []byte {
	return []byte(strings.Join([]string{
		strings.ToUpper(method),
		path,
		bodyHash,
		fmt.Sprintf("%d", unixTs),
	}, "\n"))
}
