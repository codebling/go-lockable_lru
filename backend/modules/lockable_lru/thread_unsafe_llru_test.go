package lockable_lru

import (
	"testing"
)

func TestAddLockedToEmpty(t *testing.T) {
	llru, err := NewUnsafe[string, string](10)
	if err != nil {
		t.Fatalf("could not create llru: %v", err)
	}
	ok, evicted := llru.AddOrUpdateLocked("x", "x")

	if !ok {
		t.Errorf("expected ok to be true, got false")
	}
	if evicted != nil {
		t.Errorf("expected evicted to be nil, got %v", evicted)
	}
}
