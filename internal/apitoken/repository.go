package apitoken

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, token Token, hash [32]byte) error {
	scopes, err := json.Marshal(token.Scopes)
	if err != nil {
		return fmt.Errorf("encode API token scopes: %w", err)
	}
	var expiresAt any
	if token.ExpiresAt != nil {
		expiresAt = token.ExpiresAt.Unix()
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO api_tokens(id, name, token_prefix, token_hash, scopes, expires_at, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		token.ID, token.Name, token.TokenPrefix, hash[:], string(scopes), expiresAt, token.Status, token.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create API token: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context) ([]Token, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, token_prefix, scopes, expires_at, status, created_at
		FROM api_tokens ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list API tokens: %w", err)
	}
	defer rows.Close()
	var result []Token
	for rows.Next() {
		token, _, err := scanToken(rows, false)
		if err != nil {
			return nil, err
		}
		result = append(result, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate API tokens: %w", err)
	}
	return result, nil
}

func (r *Repository) ListActive(ctx context.Context, now time.Time) (map[[32]byte]Identity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, token_prefix, token_hash, scopes, expires_at, status, created_at
		FROM api_tokens
		WHERE status = 'active' AND (expires_at IS NULL OR expires_at > ?)`, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("list active API tokens: %w", err)
	}
	defer rows.Close()
	result := make(map[[32]byte]Identity)
	for rows.Next() {
		token, rawHash, err := scanToken(rows, true)
		if err != nil {
			return nil, err
		}
		if len(rawHash) != 32 {
			return nil, fmt.Errorf("API token %s has invalid hash length", token.ID)
		}
		var hash [32]byte
		copy(hash[:], rawHash)
		result[hash] = identityFromToken(token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active API tokens: %w", err)
	}
	return result, nil
}

func (r *Repository) Revoke(ctx context.Context, id string) ([32]byte, error) {
	var rawHash []byte
	err := r.db.QueryRowContext(ctx, `
		UPDATE api_tokens SET status = 'revoked'
		WHERE id = ? AND status = 'active'
		RETURNING token_hash`, id).Scan(&rawHash)
	if err == sql.ErrNoRows {
		return [32]byte{}, ErrTokenNotFound
	}
	if err != nil {
		return [32]byte{}, fmt.Errorf("revoke API token: %w", err)
	}
	if len(rawHash) != 32 {
		return [32]byte{}, fmt.Errorf("API token %s has invalid hash length", id)
	}
	var hash [32]byte
	copy(hash[:], rawHash)
	return hash, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanToken(row rowScanner, includeHash bool) (Token, []byte, error) {
	var token Token
	var scopesJSON string
	var expiresAt sql.NullInt64
	var createdAt int64
	var rawHash []byte
	var err error
	if includeHash {
		err = row.Scan(&token.ID, &token.Name, &token.TokenPrefix, &rawHash, &scopesJSON, &expiresAt, &token.Status, &createdAt)
	} else {
		err = row.Scan(&token.ID, &token.Name, &token.TokenPrefix, &scopesJSON, &expiresAt, &token.Status, &createdAt)
	}
	if err != nil {
		return Token{}, nil, fmt.Errorf("scan API token: %w", err)
	}
	if err := json.Unmarshal([]byte(scopesJSON), &token.Scopes); err != nil {
		return Token{}, nil, fmt.Errorf("decode API token %s scopes: %w", token.ID, err)
	}
	if expiresAt.Valid {
		value := time.Unix(expiresAt.Int64, 0).UTC()
		token.ExpiresAt = &value
	}
	token.CreatedAt = time.Unix(createdAt, 0).UTC()
	return token, rawHash, nil
}
