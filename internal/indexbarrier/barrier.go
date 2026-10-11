package indexbarrier

import "sync"

// Barrier coordinates durable mutations with full in-memory index rebuilds.
// Delivery reads intentionally do not use this barrier.
type Barrier struct {
	mu      sync.RWMutex
	aliases KeyedLocker
}

func New() *Barrier {
	return &Barrier{}
}

func (b *Barrier) BeginChange() func() {
	b.mu.RLock()
	return b.mu.RUnlock
}

// BeginAliasChange locks the path before taking the shared rebuild barrier.
// Callers use this instead of BeginChange and release both locks in reverse order.
func (b *Barrier) BeginAliasChange(path string) func() {
	unlockPath := b.aliases.Lock(path)
	b.mu.RLock()
	return func() {
		b.mu.RUnlock()
		unlockPath()
	}
}

func (b *Barrier) BeginRebuild() func() {
	b.mu.Lock()
	return b.mu.Unlock
}
