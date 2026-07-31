package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

type PasswordParameters struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

const MaximumPasswordBytes = 1024

var defaultPasswordParameters = PasswordParameters{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

var passwordKDFMu sync.Mutex

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", ErrPasswordTooShort
	}
	if len(password) > MaximumPasswordBytes {
		return "", ErrPasswordTooLong
	}
	return hashPassword(password, defaultPasswordParameters)
}

func hashPassword(password string, parameters PasswordParameters) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}
	salt := make([]byte, parameters.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := derivePasswordKey(password, salt, parameters)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		parameters.Memory,
		parameters.Iterations,
		parameters.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	if len(password) > MaximumPasswordBytes {
		return false, ErrPasswordTooLong
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid password hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, fmt.Errorf("unsupported argon2 version")
	}
	var parameters PasswordParameters
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &parameters.Memory, &parameters.Iterations, &parameters.Parallelism); err != nil {
		return false, fmt.Errorf("invalid password parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode password salt: %w", err)
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode password key: %w", err)
	}
	parameters.KeyLength = uint32(len(expected))
	actual := derivePasswordKey(password, salt, parameters)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func derivePasswordKey(password string, salt []byte, parameters PasswordParameters) []byte {
	passwordKDFMu.Lock()
	defer passwordKDFMu.Unlock()
	return argon2.IDKey([]byte(password), salt, parameters.Iterations, parameters.Memory, parameters.Parallelism, parameters.KeyLength)
}
