package delivery

import "testing"

func TestGateRejectsExcessDeliveryAndReleasesCapacity(t *testing.T) {
	gate := NewGate(1)
	release, ok := gate.TryAcquire()
	if !ok {
		t.Fatal("first TryAcquire() rejected")
	}
	if _, ok := gate.TryAcquire(); ok {
		t.Fatal("second TryAcquire() exceeded configured capacity")
	}
	release()
	releaseAgain, ok := gate.TryAcquire()
	if !ok {
		t.Fatal("TryAcquire() did not recover released capacity")
	}
	releaseAgain()
}

func TestGateZeroAllowsUnlimitedDelivery(t *testing.T) {
	gate := NewGate(0)
	releases := make([]func(), 128)
	for index := range releases {
		release, ok := gate.TryAcquire()
		if !ok {
			t.Fatalf("TryAcquire() rejected unlimited request %d", index)
		}
		releases[index] = release
	}
	for _, release := range releases {
		release()
	}
}
