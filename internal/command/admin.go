package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/config"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/google/uuid"
	"golang.org/x/term"
)

func admin(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: imagesilo admin create --email ADDRESS")
	}

	flags := flag.NewFlagSet("admin create", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	email := flags.String("email", "", "administrator email address")
	passwordStdin := flags.Bool("password-stdin", false, "read one password from standard input without confirmation")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	normalizedEmail, err := auth.NormalizeEmail(*email)
	if err != nil {
		return err
	}

	password, err := readNewPassword(*passwordStdin)
	if err != nil {
		return err
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.PrepareDataDirectories(); err != nil {
		return err
	}
	db, err := database.Open(cfg.DatabasePath())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		return err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate administrator id: %w", err)
	}
	now := time.Now().UTC()
	if err := auth.NewRepository(db).CreateAdmin(context.Background(), auth.Admin{
		ID:           id.String(),
		DisplayName:  "ImageSilo",
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "administrator created for %s\n", normalizedEmail)
	return nil
}

func readNewPassword(fromStdin bool) (string, error) {
	if fromStdin {
		return readPasswordFromStdin(os.Stdin)
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("administrator password must be entered from a terminal")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read administrator password: %w", err)
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read administrator password confirmation: %w", err)
	}
	if string(first) != string(second) {
		return "", fmt.Errorf("password confirmation does not match")
	}
	return validatePassword(first)
}

func readPasswordFromStdin(reader io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, 4097))
	if err != nil {
		return "", fmt.Errorf("read administrator password from stdin: %w", err)
	}
	if len(raw) > 4096 {
		return "", fmt.Errorf("administrator password from stdin is too long")
	}
	raw = bytesTrimSingleLineEnding(raw)
	for _, value := range raw {
		if value == '\r' || value == '\n' {
			return "", fmt.Errorf("administrator password from stdin must contain exactly one line")
		}
	}
	return validatePassword(raw)
}

func bytesTrimSingleLineEnding(value []byte) []byte {
	if len(value) > 0 && value[len(value)-1] == '\n' {
		value = value[:len(value)-1]
	}
	if len(value) > 0 && value[len(value)-1] == '\r' {
		value = value[:len(value)-1]
	}
	return value
}

func validatePassword(value []byte) (string, error) {
	if len(value) < 12 {
		return "", fmt.Errorf("administrator password must contain at least 12 bytes")
	}
	return string(value), nil
}
