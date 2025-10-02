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

// If the key does not exist and there is room, it is added, making it the most recently used item. If an entry was evicted, `true, entry` is returned, otherwise `true, nil` is returned.
// If an entry was evicted, `true, entry` is returned
func TestAddOrUpdateUnlockedCase3Part1(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")

	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted == nil || evicted.Key != "new key2" || evicted.Value != "2" {
		t.Errorf("expected `true` and `Entry{Key: \"new key2\", Value: \"2\"}` evicted but got %v, %v", ok, evicted)
	}

	//check that if we add another, "new key3" is the entry that gets evicted (it is not the oldest)
	ok, evicted = llru.AddOrUpdateUnlocked("new key4", "4")
	if !ok || evicted == nil || evicted.Key != "new key3" || evicted.Value != "3" {
		t.Errorf("expected `true` and `Entry{Key: \"new key3\", Value: \"3\"}` evicted but got %v, %v", ok, evicted)
	}
}


// If the key does not exist and there is room, it is added, making it the most recently used item. If an entry was evicted, `true, entry` is returned, otherwise `true, nil` is returned.
// If an entry was not evicted, `true, nil` is returned
func TestAddOrUpdateUnlockedCase3Part2(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")

	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}

	//check that if we add another, "new key3" is the entry that gets evicted (it is not the oldest)
	ok, evicted = llru.AddOrUpdateUnlocked("new key4", "4")
	if !ok || evicted == nil || evicted.Key != "new key3" || evicted.Value != "3" {
		t.Errorf("expected `true` and `Entry{Key: \"new key3\", Value: \"3\"}` evicted but got %v, %v", ok, evicted)
	}
}

// If the key does not exist and there is no room, `false, nil` is returned.
func TestAddOrUpdateUnlockedCase4(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")
	_, _ = llru.AddOrUpdateLocked("new key2", "2")

	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}

}

// If the key exists and is locked, its value is updated, and `true, nil` is returned.
func TestAddOrUpdateLockedCase1(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	//add the test key
	_, _ = llru.AddOrUpdateLocked("new key1", "1")

	ok, evicted := llru.AddOrUpdateLocked("new key", "x")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}

}

// If the key exists and is unlocked, its value is updated and it is locked, and `true, nil` is returned.
func TestAddOrUpdateLockedCase2(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	_, _ = llru.AddOrUpdateUnlocked("new key", "x")
	ok, evicted := llru.AddOrUpdateLocked("new key", "x")
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}

	//key should now be locked, try adding another to confirm lock
	ok, evicted = llru.AddOrUpdateLocked("new key1", "1")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}
}

// If the key does not exist and there is room, it is added, making it the most recently used item. If an entry was evicted, `true, entry` is returned, otherwise `true, nil` is returned.
// If an entry was evicted, `true, entry` is returned
func TestAddOrUpdateLockedCase3Part1(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")

	ok, evicted := llru.AddOrUpdateLocked("new key2", "2")
	if !ok || evicted == nil || evicted.Key != "new key1" || evicted.Value != "1" {
		t.Errorf("expected `true` and `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}

}

// If the key does not exist and there is room, it is added, making it the most recently used item. If an entry was evicted, `true, entry` is returned, otherwise `true, nil` is returned.
// If an entry was not evicted, `true, nil` is returned
func TestAddOrUpdateLockedCase3Part2(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	ok, evicted := llru.AddOrUpdateLocked("new key2", "2")
	if !ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}
}

// If the key does not exist and there is no room, `false, nil` is returned.
func TestAddOrUpdateLockedCase4(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")

	ok, evicted := llru.AddOrUpdateLocked("new key2", "2")
	if ok || evicted != nil {
		t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
	}
}

// If the key exists and is unlocked, it is locked, and `true` is returned
func TestLockCase1(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	_, _ = llru.AddOrUpdateUnlocked("new key", "x")
	ok := llru.Lock("new key")
	if !ok {
		t.Errorf("expected `true` but got %v", ok)
	}

	//key should now be locked, try adding another to confirm lock
	ok, evicted := llru.AddOrUpdateLocked("new key1", "1")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}
}

// If the key exists and is locked, `true` is returned
func TestLockCase2(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	_, _ = llru.AddOrUpdateLocked("new key", "x")
	ok := llru.Lock("new key")
	if !ok {
		t.Errorf("expected `true` but got %v", ok)
	}

	//key should now be locked, try adding another to confirm lock
	ok, evicted := llru.AddOrUpdateLocked("new key1", "1")
	if ok || evicted != nil {
		t.Errorf("expected `false, nil` but got %v, %v", ok, evicted)
	}
}

// If the key does not exist, returns `false`
func TestLockCase3(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	ok := llru.Lock("new key")
	if ok {
		t.Errorf("expected `false` but got %v", ok)
	}
}

// If the key exists and is locked, it is unlocked, making it the most recently used item, and `true` is returned
func TestUnlockCase1(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")

	ok := llru.Unlock("new key1")
	if !ok {
		t.Errorf("expected `true` but got %v", ok)
	}

	//check that if we add another, "new key1" is not the entry that gets evicted (it was most recently used)
	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted == nil || evicted.Key == "new key1" || evicted.Value == "1" {
		t.Errorf("expected `true` and NOT `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
	//check that if we add another, "new key1" is the entry that gets evicted (it is oldest and it is unlocked)
	ok, evicted = llru.AddOrUpdateUnlocked("new key4", "4")
	if !ok || evicted == nil || evicted.Key != "new key1" || evicted.Value != "1" {
		t.Errorf("expected `true` and `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
}

// If the key exists and is unlocked, it becomes the most recently used item, and `true` is returned
func TestUnlockCase2(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")

	ok := llru.Unlock("new key1")
	if !ok {
		t.Errorf("expected `true` but got %v", ok)
	}

	//check that if we add another, "new key1" is not the entry that gets evicted (it was most recently used)
	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted == nil || evicted.Key == "new key1" || evicted.Value == "1" {
		t.Errorf("expected `true` and NOT `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
	//check that if we add another, "new key1" is the entry that gets evicted (it is oldest and it is unlocked)
	ok, evicted = llru.AddOrUpdateUnlocked("new key4", "4")
	if !ok || evicted == nil || evicted.Key != "new key1" || evicted.Value != "1" {
		t.Errorf("expected `true` and `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
}

// If the key does not exist, returns `false`
func TestUnlockCase3(t *testing.T) {
	llru := buildNewEmpty(t, 1)

	ok := llru.Unlock("new key")
	if ok {
		t.Errorf("expected `false` but got %v", ok)
	}
}


// If the key exists and is locked, the value is returned
func TestGetCase1(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")

	value := llru.Get("new key1")
	if *value != "1" {
		t.Errorf("expected `1` but got %v", *value)
	}
}

// If the key exists and is unlocked, it becomes the most recently used item, and the value is returned
func TestGetCase2(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")

	value := llru.Get("new key1")
	if *value != "1" {
		t.Errorf("expected `1` but got %v", *value)
	}

	//test that `new key1`, which was the oldest, became most recent and didn't get evicted
	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted == nil || evicted.Key == "new key1" || evicted.Value == "1" {
		t.Errorf("expected `true` and NOT `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}

}

// If the key does not exist, `nil` is returned
func TestGetCase3(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	value := llru.Get("x")
	if value != nil {
		t.Errorf("expected `nil` but got %v", value)
	}
}

// If the key exists, true is returned. The recentness of the item is unchanged
func TestContainsReturnsTrueWhenExistsInLocked(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")

	exists := llru.Contains("new key1")
	if exists != true {
		t.Errorf("expected `true` but got %v", exists)
	}
}

// If the key exists, true is returned. The recentness of the item is unchanged
func TestContainsReturnsTrueWhenExistsInUnlocked(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")

	exists := llru.Contains("new key1")
	if exists != true {
		t.Errorf("expected `true` but got %v", exists)
	}
}

// If the key exists, true is returned. The recentness of the item is unchanged
func TestContainsDoesNotUpdateRecentness(t *testing.T) {
	llru := buildNewEmpty(t, 2)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")

	_ = llru.Contains("new key1")

	//"new key1" should still be the oldest. Since we're full, it should be the next evicted.
	ok, evicted := llru.AddOrUpdateUnlocked("new key3", "3")
	if !ok || evicted == nil || evicted.Key != "new key1" || evicted.Value != "1" {
		t.Errorf("expected `true` and `Entry{Key: \"new key1\", Value: \"1\"}` evicted but got %v, %v", ok, evicted)
	}
}

// If the key does not exist, false is returned. 
func TestContainsReturnsFalseWhenKeyDoesNotExist(t *testing.T) {
	llru := buildPartiallyLocked(t, 3, 3)

	exists := llru.Contains("xyzabc")
	if exists != false {
		t.Errorf("expected `false` but got %v", exists)
	}

}