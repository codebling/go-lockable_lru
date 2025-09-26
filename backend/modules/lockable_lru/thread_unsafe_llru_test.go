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
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}
}

func TestAddUnlockedToEmpty(t *testing.T) {
	llru, err := NewUnsafe[string, string](10)
	if err != nil {
		t.Fatalf("could not create llru: %v", err)
	}
	ok, evicted := llru.AddOrUpdateUnlocked("x", "x")
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}
}

