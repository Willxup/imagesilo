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

func (s *Service) Login(ctx context.Context, email, password string, now time.Time) (SessionIdentity, string, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	admin, err := s.repository.FindAdminByEmail(ctx, email)
	encoded := s.dummyHash
	if err == nil {
		encoded = admin.PasswordHash
	} else if err != sql.ErrNoRows {
		return SessionIdentity{}, "", "", fmt.Errorf("find administrator: %w", err)
	}

	valid, verifyErr := VerifyPassword(password, encoded)
	if verifyErr != nil {
		return SessionIdentity{}, "", "", verifyErr
	}
	if err == sql.ErrNoRows || !valid {
		return SessionIdentity{}, "", "", ErrInvalidCredentials
	}

	token, hash, err := newSessionToken()
	if err != nil {
		return SessionIdentity{}, "", "", err
	}
	csrfToken, csrfHash, err := newCSRFToken()
	if err != nil {
		return SessionIdentity{}, "", "", err
	}
	sessionID, err := uuid.NewV7()
	if err != nil {
		return SessionIdentity{}, "", "", fmt.Errorf("generate session id: %w", err)
	}
	identity := SessionIdentity{
		SessionID: sessionID.String(),
		AdminID:   admin.ID,
		Email:     admin.Email,
		CSRFHash:  csrfHash,
		ExpiresAt: now.Add(sessionLifetime).UTC(),
	}
	session := Session{
		ID:        identity.SessionID,
		AdminID:   identity.AdminID,
		TokenHash: hash,
		CSRFHash:  csrfHash,
		ExpiresAt: identity.ExpiresAt,
		CreatedAt: now.UTC(),
	}
	if err := s.repository.CreateSession(ctx, session); err != nil {
		return SessionIdentity{}, "", "", err
	}
	s.index.Add(hash, identity)
	return identity, token, csrfToken, nil
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

func (s *Service) ValidateCSRF(identity SessionIdentity, token string) error {
	if token == "" {
		return ErrInvalidCSRF
	}
	hash := sha256.Sum256([]byte(token))
	if hash != identity.CSRFHash {
		return ErrInvalidCSRF
	}
	return nil
}

func (s *Service) ChangePassword(
	ctx context.Context,
	identity SessionIdentity,
	sessionToken string,
	currentPassword string,
	newPassword string,
	now time.Time,
) error {
	admin, err := s.repository.FindAdminByID(ctx, identity.AdminID)
	if err != nil {
		return fmt.Errorf("find administrator for password change: %w", err)
	}
	valid, err := VerifyPassword(currentPassword, admin.PasswordHash)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidCredentials
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	keepSessionHash := sha256.Sum256([]byte(sessionToken))
	if err := s.repository.ChangePasswordAndRevokeOtherSessions(ctx, identity.AdminID, newHash, keepSessionHash, now); err != nil {
		return err
	}
	s.index.RemoveAllExcept(keepSessionHash)
	return nil
}

func (s *Service) CleanupExpired(ctx context.Context, now time.Time) (int64, error) {
	deleted, err := s.repository.DeleteExpiredSessions(ctx, now)
	if err != nil {
		return 0, err
	}
	s.index.PurgeExpired(now)
	return deleted, nil
}

func (s *Service) SessionCount() int {
	return s.index.Len()
}

func newSessionToken() (string, [32]byte, error) {
	return newRandomToken("iss_")
}

func newCSRFToken() (string, [32]byte, error) {
	return newRandomToken("isc_")
}

func newRandomToken(prefix string) (string, [32]byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate session token: %w", err)
	}
	token := prefix + base64.RawURLEncoding.EncodeToString(raw)
	return token, sha256.Sum256([]byte(token)), nil
}
