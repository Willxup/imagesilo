package httpapi

import (
	"sync"
	"time"
)

const maxLoginLimiterEntries = 4096

type loginLimitEntry struct {
	attempts int
	resetAt  time.Time
}

type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]loginLimitEntry
	limit   int
	window  time.Duration
}

func newLoginLimiter(limit int, window time.Duration) *loginLimiter {
	return &loginLimiter{entries: make(map[string]loginLimitEntry), limit: limit, window: window}
}

func (l *loginLimiter) Allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) >= maxLoginLimiterEntries {
		for existingKey, existing := range l.entries {
			if !existing.resetAt.After(now) {
				delete(l.entries, existingKey)
			}
		}
		if _, exists := l.entries[key]; !exists && len(l.entries) >= maxLoginLimiterEntries {
			return false, l.window
		}
	}
	entry, ok := l.entries[key]
	if !ok || !entry.resetAt.After(now) {
		entry = loginLimitEntry{resetAt: now.Add(l.window)}
	}
	if entry.attempts >= l.limit {
		return false, entry.resetAt.Sub(now)
	}
	entry.attempts++
	l.entries[key] = entry
	return true, 0
}

func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}
