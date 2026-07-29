package httpapi

import (
	"testing"
	"time"
)

func TestLoginLimiterBlocksAndResetsKeysIndependently(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limiter := newLoginLimiter(2, time.Minute)
	for attempt := 1; attempt <= 2; attempt++ {
		if allowed, _ := limiter.Allow("account:admin@example.com", now); !allowed {
			t.Fatalf("attempt %d was unexpectedly blocked", attempt)
		}
	}
	if allowed, retry := limiter.Allow("account:admin@example.com", now.Add(10*time.Second)); allowed || retry != 50*time.Second {
		t.Fatalf("blocked attempt = (%t, %s), want (false, 50s)", allowed, retry)
	}
	if allowed, _ := limiter.Allow("address:127.0.0.1", now.Add(10*time.Second)); !allowed {
		t.Fatal("one key unexpectedly blocked another key")
	}
	limiter.Reset("account:admin@example.com")
	if allowed, _ := limiter.Allow("account:admin@example.com", now.Add(10*time.Second)); !allowed {
		t.Fatal("Reset() did not allow a new attempt")
	}
	if allowed, _ := limiter.Allow("window", now); !allowed {
		t.Fatal("first window attempt was blocked")
	}
	if allowed, _ := limiter.Allow("window", now.Add(time.Minute)); !allowed {
		t.Fatal("expired limiter window did not reset")
	}
}

func TestLoginLimiterBoundsUnexpiredKeyMemory(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limiter := newLoginLimiter(1, time.Minute)
	for index := 0; index < maxLoginLimiterEntries; index++ {
		key := time.Unix(int64(index), 0).String()
		if allowed, _ := limiter.Allow(key, now); !allowed {
			t.Fatalf("key %d was blocked before the limiter reached its bound", index)
		}
	}
	if allowed, retry := limiter.Allow("one-key-too-many", now); allowed || retry != time.Minute {
		t.Fatalf("entry bound result = (%t, %s), want (false, 1m)", allowed, retry)
	}
}
