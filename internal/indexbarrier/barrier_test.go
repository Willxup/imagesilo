package indexbarrier

import (
	"runtime"
	"testing"
	"time"
)

func TestChangesShareBarrierWhileRebuildIsExclusive(t *testing.T) {
	barrier := New()
	releaseFirst := barrier.BeginChange()
	secondAcquired := make(chan func(), 1)
	go func() { secondAcquired <- barrier.BeginChange() }()
	var releaseSecond func()
	select {
	case releaseSecond = <-secondAcquired:
	case <-time.After(time.Second):
		t.Fatal("a second ordinary index change was serialized")
	}

	rebuildAcquired := make(chan func(), 1)
	go func() { rebuildAcquired <- barrier.BeginRebuild() }()
	select {
	case release := <-rebuildAcquired:
		release()
		t.Fatal("rebuild acquired its exclusive lock while changes were active")
	case <-time.After(25 * time.Millisecond):
	}
	releaseFirst()
	releaseSecond()
	select {
	case release := <-rebuildAcquired:
		release()
	case <-time.After(time.Second):
		t.Fatal("rebuild did not proceed after ordinary changes completed")
	}
}

func TestAliasChangesLockOnlyTheirPathAndReleaseEntries(t *testing.T) {
	barrier := New()
	releaseFirst := barrier.BeginAliasChange("/same.jpg")
	samePath := make(chan func(), 1)
	go func() { samePath <- barrier.BeginAliasChange("/same.jpg") }()
	waitForAliasReferences(t, barrier, "/same.jpg", 2)
	otherPath := make(chan func(), 1)
	go func() { otherPath <- barrier.BeginAliasChange("/other.jpg") }()
	select {
	case release := <-otherPath:
		release()
	case <-time.After(time.Second):
		t.Fatal("an unrelated alias path was serialized")
	}
	select {
	case release := <-samePath:
		release()
		t.Fatal("the same alias path was acquired concurrently")
	default:
	}
	releaseFirst()
	select {
	case release := <-samePath:
		release()
	case <-time.After(time.Second):
		t.Fatal("the same alias path was not released")
	}
	barrier.aliases.mu.Lock()
	remaining := len(barrier.aliases.entries)
	barrier.aliases.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("idle alias lock entries = %d, want 0", remaining)
	}
}

func TestWaitingAliasChangeAllowsConcurrentRebuild(t *testing.T) {
	barrier := New()
	releaseFirst := barrier.BeginAliasChange("/same.jpg")
	waitingAlias := make(chan func(), 1)
	go func() { waitingAlias <- barrier.BeginAliasChange("/same.jpg") }()
	waitForAliasReferences(t, barrier, "/same.jpg", 2)
	rebuild := make(chan func(), 1)
	go func() { rebuild <- barrier.BeginRebuild() }()
	select {
	case release := <-rebuild:
		release()
		t.Fatal("rebuild passed an active alias mutation")
	case <-time.After(25 * time.Millisecond):
	}
	releaseFirst()
	var releaseRebuild func()
	select {
	case releaseRebuild = <-rebuild:
	case <-time.After(time.Second):
		t.Fatal("waiting alias change blocked the pending rebuild")
	}
	select {
	case release := <-waitingAlias:
		release()
		t.Fatal("alias mutation passed an active rebuild")
	default:
	}
	releaseRebuild()
	select {
	case release := <-waitingAlias:
		release()
	case <-time.After(time.Second):
		t.Fatal("alias mutation did not resume after rebuild")
	}
}

func waitForAliasReferences(t *testing.T, barrier *Barrier, path string, want int) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		barrier.aliases.mu.Lock()
		entry := barrier.aliases.entries[path]
		ready := entry != nil && entry.refs == want
		barrier.aliases.mu.Unlock()
		if ready {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("alias lock %q did not acquire %d references", path, want)
		default:
			runtime.Gosched()
		}
	}
}
