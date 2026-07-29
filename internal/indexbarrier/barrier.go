package indexbarrier

import "sync"

// Barrier coordinates durable mutations with full in-memory index rebuilds.
// Delivery reads intentionally do not use this barrier.
type Barrier struct {
	mu sync.RWMutex
}

func New() *Barrier {
	return &Barrier{}
}

func (b *Barrier) BeginChange() func() {
	b.mu.RLock()
	return b.mu.RUnlock
}

func (b *Barrier) BeginRebuild() func() {
	b.mu.Lock()
	return b.mu.Unlock
}
