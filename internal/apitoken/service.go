package apitoken

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repository *Repository
	index      *Index
}

func NewService(repository *Repository, index *Index) *Service {
	return &Service{repository: repository, index: index}
}

func (s *Service) Load(ctx context.Context, now time.Time) error {
	tokens, err := s.repository.ListActive(ctx, now)
	if err != nil {
		return err
	}
	s.index.Replace(tokens)
	return nil
}

func (s *Service) Create(ctx context.Context, name string, scopes []Scope, expiresAt *time.Time, now time.Time) (Token, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return Token{}, "", ErrInvalidName
	}
	normalizedScopes, err := normalizeScopes(scopes)
	if err != nil {
		return Token{}, "", err
	}
	if expiresAt != nil {
		value := expiresAt.UTC()
		if !value.After(now) {
			return Token{}, "", ErrInvalidExpiration
		}
		expiresAt = &value
	}
	plaintext, prefix, hash, err := generateToken()
	if err != nil {
		return Token{}, "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Token{}, "", fmt.Errorf("generate API token id: %w", err)
	}
	token := Token{
		ID: id.String(), Name: name, TokenPrefix: prefix, Scopes: normalizedScopes,
		ExpiresAt: expiresAt, Status: "active", CreatedAt: now.UTC(),
	}
	if err := s.repository.Create(ctx, token, hash); err != nil {
		return Token{}, "", err
	}
	s.index.Add(hash, identityFromToken(token))
	return token, plaintext, nil
}

func (s *Service) List(ctx context.Context) ([]Token, error) {
	return s.repository.List(ctx)
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	hash, err := s.repository.Revoke(ctx, id)
	if err != nil {
		return err
	}
	s.index.Remove(hash)
	return nil
}

func (s *Service) Authenticate(plaintext string, now time.Time) (Identity, error) {
	if !strings.HasPrefix(plaintext, "ist_") {
		return Identity{}, ErrInvalidToken
	}
	hash := sha256.Sum256([]byte(plaintext))
	identity, ok := s.index.Get(hash, now)
	if !ok {
		return Identity{}, ErrInvalidToken
	}
	return identity, nil
}

func (s *Service) CleanupExpired(now time.Time) int {
	return s.index.PurgeExpired(now)
}

func (s *Service) TokenCount() int {
	return s.index.Len()
}

func normalizeScopes(scopes []Scope) ([]Scope, error) {
	if len(scopes) == 0 {
		return nil, ErrInvalidScope
	}
	set := make(map[Scope]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, ok := validScopes[scope]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidScope, scope)
		}
		set[scope] = struct{}{}
	}
	result := make([]Scope, 0, len(set))
	for scope := range set {
		result = append(result, scope)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func identityFromToken(token Token) Identity {
	scopes := make(map[Scope]struct{}, len(token.Scopes))
	for _, scope := range token.Scopes {
		scopes[scope] = struct{}{}
	}
	return Identity{TokenID: token.ID, Prefix: token.TokenPrefix, Scopes: scopes, ExpiresAt: token.ExpiresAt}
}
