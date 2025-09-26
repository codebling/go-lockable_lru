package lockable_lru

import (
	"testing"
)

func buildNewEmpty(t *testing.T, size int) *ThreadunsafeLLRU[string, string] {
	llru, err := NewUnsafe[string, string](size)
	if err != nil {
		t.Fatalf("could not create llru: %v", err)
	}
	return llru
}

func buildFullyLocked(t *testing.T, size int) *ThreadunsafeLLRU[string, string] {
	llru := buildNewEmpty(t, size)

	for i := range size {
		ok, evicted := llru.AddOrUpdateLocked(string(rune(i)), "x")
		if !ok || evicted != nil {
			t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
		}
	}

	return llru
}

func buildPartiallyLocked(t *testing.T, lockedSize int, unlockedSize int) *ThreadunsafeLLRU[string, string] {
	llru := buildNewEmpty(t, lockedSize+unlockedSize)

	for i := range lockedSize {
		ok, evicted := llru.AddOrUpdateLocked(string(rune(i)), "x")
		if !ok || evicted != nil {
			t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
		}
	}

	for i := range unlockedSize {
		ok, evicted := llru.AddOrUpdateUnlocked(string(rune(i+lockedSize)), "x")
		if !ok || evicted != nil {
			t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
		}
	}

	return llru
}

func TestAddLockedToEmpty(t *testing.T) {
	llru := buildNewEmpty(t, 10)

	ok, evicted := llru.AddOrUpdateLocked("x", "x")
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}
}

func TestAddUnlockedToEmpty(t *testing.T) {
	llru := buildNewEmpty(t, 10)

	ok, evicted := llru.AddOrUpdateUnlocked("x", "x")
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}
}

func TestAddLockedToFullyLocked(t *testing.T) {
	llru := buildFullyLocked(t, 5)

	//cache now full, another add with new key should fail
	ok, evicted := llru.AddOrUpdateLocked("new key", "x")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}
}

func TestAddUnlockedToFullyLocked(t *testing.T) {
	llru := buildFullyLocked(t, 5)

	//cache now full, another add with new key should fail
	ok, evicted := llru.AddOrUpdateUnlocked("new key", "x")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}
}

func TestAddLockedToFullButEvictable(t *testing.T) {
	llru := buildPartiallyLocked(t, 2, 2)

	ok, evicted := llru.AddOrUpdateLocked("new key", "x")
	if !ok || evicted == nil {
		t.Errorf("expected `true` and one item evicted but got %v, %v", ok, evicted)
	}
}

func TestAddUnlockedToFullButEvictable(t *testing.T) {
	llru := buildPartiallyLocked(t, 2, 2)

	ok, evicted := llru.AddOrUpdateUnlocked("new key", "x")
	if !ok || evicted == nil {
		t.Errorf("expected `true` and one item evicted but got %v, %v", ok, evicted)
	}
}

