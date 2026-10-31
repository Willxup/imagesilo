package indexbarrier

import "sync"

type keyedLockEntry struct {
	mu   sync.Mutex
	refs int
}

// KeyedLocker serializes mutations per key and removes idle lock entries.
type KeyedLocker struct {
	mu      sync.Mutex
	entries map[string]*keyedLockEntry
}

func (l *KeyedLocker) Lock(key string) func() {
	l.mu.Lock()
	if l.entries == nil {
		l.entries = make(map[string]*keyedLockEntry)
	}
	entry := l.entries[key]
	if entry == nil {
		entry = &keyedLockEntry{}
		l.entries[key] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.entries, key)
		}
		l.mu.Unlock()
	}
}
