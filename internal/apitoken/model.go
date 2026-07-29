package apitoken

import "time"

type Scope string

const (
	ScopeImagesUpload      Scope = "images:upload"
	ScopeImagesReadPrivate Scope = "images:read_private"
	ScopeImagesDelete      Scope = "images:delete"
	ScopeAliasesWrite      Scope = "aliases:write"
)

var validScopes = map[Scope]struct{}{
	ScopeImagesUpload:      {},
	ScopeImagesReadPrivate: {},
	ScopeImagesDelete:      {},
	ScopeAliasesWrite:      {},
}

type Token struct {
	ID          string
	Name        string
	TokenPrefix string
	Scopes      []Scope
	ExpiresAt   *time.Time
	Status      string
	CreatedAt   time.Time
}

type Identity struct {
	TokenID   string
	Prefix    string
	Scopes    map[Scope]struct{}
	ExpiresAt *time.Time
}

func (i Identity) HasScope(scope Scope) bool {
	_, ok := i.Scopes[scope]
	return ok
}
