package delivery

import "testing"

func TestIndexReplaceAndRead(t *testing.T) {
	index := NewIndex()
	index.Add("first", Target{StorageKey: "first"})
	index.AddAlias("/legacy/first.jpg", "first")
	if _, ok := index.Get("first"); !ok {
		t.Fatal("Get(first) missed an inserted target")
	}
	if target, ok := index.GetAlias("/legacy/first.jpg"); !ok || target.StorageKey != "first" {
		t.Fatal("GetAlias() did not resolve through the target map")
	}
	index.ReplaceAll(
		map[string]Target{"second": {StorageKey: "second"}},
		map[string]string{"/legacy/second.jpg": "second"},
	)
	if _, ok := index.Get("first"); ok {
		t.Fatal("Replace() retained stale target")
	}
	if _, ok := index.GetAlias("/legacy/first.jpg"); ok {
		t.Fatal("ReplaceAll() retained a stale alias")
	}
	if target, ok := index.Get("second"); !ok || target.StorageKey != "second" {
		t.Fatal("Replace() did not expose the complete new map")
	}
	if id, ok := index.ResolveAlias("/legacy/second.jpg"); !ok || id != "second" {
		t.Fatalf("ResolveAlias() = %q, %t", id, ok)
	}
	index.RemoveAlias("/legacy/second.jpg")
	if index.AliasLen() != 0 {
		t.Fatalf("AliasLen() = %d, want 0", index.AliasLen())
	}
}
