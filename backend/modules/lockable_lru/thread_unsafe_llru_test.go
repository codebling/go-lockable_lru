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

func TestEvictsOldestEntry(t *testing.T) {
	llru := buildPartiallyLocked(t, 2, 2)

	//room for 2 unlocked entries. When we add 3 in a row, the first should get evicted
	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")
	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")

	if !ok || evicted == nil || evicted.Key != "new key1" || evicted.Value != "1" {
		t.Errorf("expected `true` and `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
}

// If the key exists and is unlocked, its value is updated, making it the most recently used item, and `true, nil` is returned.
func TestAddOrUpdateUnlockedCase1(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")

	ok, evicted := llru.AddOrUpdateUnlocked("new key1", "1") //should become most recently used, even if it has the same value
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}

	//check that if we add another, "new key1" is not the entry that gets evicted (it was most recently used)
	ok, evicted = llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted == nil || evicted.Key == "new key1" || evicted.Value == "1" {
		t.Errorf("expected `true` and NOT `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
}

// If the key exists and is locked, its value is updated and it is unlocked, making it the most recently used item, and `true, nil` is returned.
func TestAddOrUpdateUnlockedCase2(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")

	ok, evicted := llru.AddOrUpdateUnlocked("new key1", "1")
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}

	//check that if we add another, "new key1" is not the entry that gets evicted (it was most recently used)
	ok, evicted = llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted == nil || evicted.Key == "new key1" || evicted.Value == "1" {
		t.Errorf("expected `true` and NOT `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
	//check that if we add another, "new key1" is the entry that gets evicted (it is oldest and it is unlocked)
	ok, evicted = llru.AddOrUpdateUnlocked("new key4", "4")
	if !ok || evicted == nil || evicted.Key != "new key1" || evicted.Value != "1" {
		t.Errorf("expected `true` and `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
}