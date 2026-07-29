package auth

import (
	"sync"
	"time"
)

type SessionIndex struct {
	mu       sync.RWMutex
	sessions map[[32]byte]SessionIdentity
}

func NewSessionIndex() *SessionIndex {
	return &SessionIndex{sessions: make(map[[32]byte]SessionIdentity)}
}

func (i *SessionIndex) Replace(sessions map[[32]byte]SessionIdentity) {
	i.mu.Lock()
	i.sessions = sessions
	i.mu.Unlock()
}

func (i *SessionIndex) Add(hash [32]byte, identity SessionIdentity) {
	i.mu.Lock()
	i.sessions[hash] = identity
	i.mu.Unlock()
}

func (i *SessionIndex) Remove(hash [32]byte) {
	i.mu.Lock()
	delete(i.sessions, hash)
	i.mu.Unlock()
}

func (i *SessionIndex) Get(hash [32]byte, now time.Time) (SessionIdentity, bool) {
	i.mu.RLock()
	identity, ok := i.sessions[hash]
	i.mu.RUnlock()
	if !ok || !identity.ExpiresAt.After(now) {
		return SessionIdentity{}, false
	}
	return identity, true
}
