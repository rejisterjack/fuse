package auth

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const Prefix = "fuse_"

// GenerateKey returns a new plaintext Fuse proxy key.
func GenerateKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return Prefix + hex.EncodeToString(b), nil
}

// IsFuseKey reports whether the bearer token looks like a Fuse key.
func IsFuseKey(token string) bool {
	return strings.HasPrefix(token, Prefix)
}

// BearerToken extracts a bearer token from Authorization header value.
func BearerToken(authHeader string) string {
	const p = "Bearer "
	if strings.HasPrefix(authHeader, p) {
		return strings.TrimSpace(authHeader[len(p):])
	}
	if strings.HasPrefix(authHeader, "bearer ") {
		return strings.TrimSpace(authHeader[len("bearer "):])
	}
	return ""
}
