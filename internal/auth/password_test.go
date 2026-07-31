package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	parameters := PasswordParameters{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	hash, err := hashPassword("correct horse battery staple", parameters)
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	valid, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !valid {
		t.Fatal("VerifyPassword() rejected the correct password")
	}
	valid, err = VerifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("VerifyPassword(wrong) error = %v", err)
	}
	if valid {
		t.Fatal("VerifyPassword() accepted the wrong password")
	}
}

func TestHashPasswordRejectsOversizedInputBeforeKDF(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("x", MaximumPasswordBytes+1)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("HashPassword() error = %v, want ErrPasswordTooLong", err)
	}
}
