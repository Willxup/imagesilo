package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const sessionLifetime = 24 * time.Hour

const dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$CaZiBzg+q0Gx30vFL1d3Xg$rt3GDgS5aKjp0hB3w/VcOEuZBdYepyP0FgzG7dhZqTU"

type Service struct {
	repository *Repository
	index      *SessionIndex
	dummyHash  string
}

func NewService(repository *Repository, index *SessionIndex) (*Service, error) {
	return &Service{repository: repository, index: index, dummyHash: dummyPasswordHash}, nil
}

func (s *Service) LoadSessions(ctx context.Context, now time.Time) error {
	sessions, err := s.repository.ListActiveSessions(ctx, now)
	if err != nil {
		return err
	}
	s.index.Replace(sessions)
	return nil
}

func (s *Service) Login(ctx context.Context, email, password string, now time.Time) (SessionIdentity, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	admin, err := s.repository.FindAdminByEmail(ctx, email)
	encoded := s.dummyHash
	if err == nil {
		encoded = admin.PasswordHash
	} else if err != sql.ErrNoRows {
		return SessionIdentity{}, "", fmt.Errorf("find administrator: %w", err)
	}

	valid, verifyErr := VerifyPassword(password, encoded)
	if verifyErr != nil {
		return SessionIdentity{}, "", verifyErr
	}
	if err == sql.ErrNoRows || !valid {
		return SessionIdentity{}, "", ErrInvalidCredentials
	}

	token, hash, err := newSessionToken()
	if err != nil {
		return SessionIdentity{}, "", err
	}
	sessionID, err := uuid.NewV7()
	if err != nil {
		return SessionIdentity{}, "", fmt.Errorf("generate session id: %w", err)
	}
	identity := SessionIdentity{
		SessionID: sessionID.String(),
		AdminID:   admin.ID,
		Email:     admin.Email,
		ExpiresAt: now.Add(sessionLifetime).UTC(),
	}
	session := Session{
		ID:        identity.SessionID,
		AdminID:   identity.AdminID,
		TokenHash: hash,
		ExpiresAt: identity.ExpiresAt,
		CreatedAt: now.UTC(),
	}
	if err := s.repository.CreateSession(ctx, session); err != nil {
		return SessionIdentity{}, "", err
	}
	s.index.Add(hash, identity)
	return identity, token, nil
}

func (s *Service) Authenticate(token string, now time.Time) (SessionIdentity, error) {
	if token == "" {
		return SessionIdentity{}, ErrInvalidSession
	}
	hash := sha256.Sum256([]byte(token))
	identity, ok := s.index.Get(hash, now)
	if !ok {
		return SessionIdentity{}, ErrInvalidSession
	}
	return identity, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(token))
	if err := s.repository.DeleteSessionByHash(ctx, hash); err != nil {
		return err
	}
	s.index.Remove(hash)
	return nil
}

func newSessionToken() (string, [32]byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate session token: %w", err)
	}
	token := "iss_" + base64.RawURLEncoding.EncodeToString(raw)
	return token, sha256.Sum256([]byte(token)), nil
}
