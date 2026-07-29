package delivery

import "testing"

func TestIndexReplaceAndRead(t *testing.T) {
	index := NewIndex()
	index.Add("first", Target{StorageKey: "first"})
	if _, ok := index.Get("first"); !ok {
		t.Fatal("Get(first) missed an inserted target")
	}
	index.Replace(map[string]Target{"second": {StorageKey: "second"}})
	if _, ok := index.Get("first"); ok {
		t.Fatal("Replace() retained stale target")
	}
	if target, ok := index.Get("second"); !ok || target.StorageKey != "second" {
		t.Fatal("Replace() did not expose the complete new map")
	}
}
