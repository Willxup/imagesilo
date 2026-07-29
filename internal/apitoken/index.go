package apitoken

import (
	"sync"
	"time"
)

type Index struct {
	mu     sync.RWMutex
	tokens map[[32]byte]Identity
}

func NewIndex() *Index {
	return &Index{tokens: make(map[[32]byte]Identity)}
}

func (i *Index) Replace(tokens map[[32]byte]Identity) {
	i.mu.Lock()
	i.tokens = tokens
	i.mu.Unlock()
}

func (i *Index) Add(hash [32]byte, identity Identity) {
	i.mu.Lock()
	i.tokens[hash] = identity
	i.mu.Unlock()
}

func (i *Index) Remove(hash [32]byte) {
	i.mu.Lock()
	delete(i.tokens, hash)
	i.mu.Unlock()
}

func (i *Index) Get(hash [32]byte, now time.Time) (Identity, bool) {
	i.mu.RLock()
	identity, ok := i.tokens[hash]
	i.mu.RUnlock()
	if !ok || (identity.ExpiresAt != nil && !identity.ExpiresAt.After(now)) {
		return Identity{}, false
	}
	return identity, true
}

func (i *Index) PurgeExpired(now time.Time) int {
	i.mu.Lock()
	removed := 0
	for hash, identity := range i.tokens {
		if identity.ExpiresAt != nil && !identity.ExpiresAt.After(now) {
			delete(i.tokens, hash)
			removed++
		}
	}
	i.mu.Unlock()
	return removed
}

func (i *Index) Len() int {
	i.mu.RLock()
	length := len(i.tokens)
	i.mu.RUnlock()
	return length
}
