package apitoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func generateToken() (string, string, [32]byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", [32]byte{}, fmt.Errorf("generate API token: %w", err)
	}
	token := "ist_" + base64.RawURLEncoding.EncodeToString(raw)
	prefix := token
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return token, prefix, sha256.Sum256([]byte(token)), nil
}
