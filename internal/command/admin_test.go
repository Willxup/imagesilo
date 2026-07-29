package command

import (
	"strings"
	"testing"
)

func TestReadPasswordFromStdin(t *testing.T) {
	password, err := readPasswordFromStdin(strings.NewReader("a-secure-ci-password\n"))
	if err != nil {
		t.Fatalf("readPasswordFromStdin() error = %v", err)
	}
	if password != "a-secure-ci-password" {
		t.Fatalf("password = %q", password)
	}
}

func TestReadPasswordFromStdinRejectsMultipleLines(t *testing.T) {
	if _, err := readPasswordFromStdin(strings.NewReader("first-password\nsecond-password\n")); err == nil {
		t.Fatal("readPasswordFromStdin() accepted multiple lines")
	}
}
