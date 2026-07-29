package indexbarrier

import (
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
