package auth

import "time"

type Admin struct {
	ID           string
	DisplayName  string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        string
	AdminID   string
	TokenHash [32]byte
	CSRFHash  [32]byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

type SessionIdentity struct {
	SessionID   string
	AdminID     string
	DisplayName string
	Email       string
	CSRFHash    [32]byte
	ExpiresAt   time.Time
}
