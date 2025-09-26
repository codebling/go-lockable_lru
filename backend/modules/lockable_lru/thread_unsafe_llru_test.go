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

func TestAddLockedToFullyLocked(t *testing.T) {
	llru, err := NewUnsafe[string, string](5)
	if err != nil {
		t.Fatalf("could not create llru: %v", err)
	}

	for i := range 5 {
		ok, evicted := llru.AddOrUpdateLocked(string(rune(i)), "x")
		if !ok || evicted != nil {
			t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
		}
	}

	//cache now full, another add with new key should fail
	ok, evicted := llru.AddOrUpdateLocked("new key", "x")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}
}

