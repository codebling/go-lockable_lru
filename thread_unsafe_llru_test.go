package lockable_lru

import (
	"slices"
	"strconv"
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
		ok, evicted := llru.AddOrUpdateLocked(string(rune(i)), "x" + strconv.Itoa(i))
		if !ok || evicted != nil {
			t.Errorf("expected `true, nil` but got %v, %v", ok, evicted)
		}
	}

	for i := range unlockedSize {
		ok, evicted := llru.AddOrUpdateUnlocked(string(rune(i+lockedSize)), "x" + strconv.Itoa(i+lockedSize))
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

func TestLen(t *testing.T) {
	lockedLen := 3
	unlockedLen := 3
	llru := buildPartiallyLocked(t, lockedLen, unlockedLen)

	len := llru.Len()
	expectedLen := lockedLen + unlockedLen
	if len != expectedLen {
		t.Errorf("expected `%v` but got %v", expectedLen, len)
	}
}

func TestValuesLenEqualsLen(t *testing.T) {
	lockedLen := 3
	unlockedLen := 3
	length := lockedLen + unlockedLen
	llru := buildPartiallyLocked(t, lockedLen, unlockedLen)

	valuesLen := len(llru.Values())

	if valuesLen != length {
		t.Errorf("expected `%v` but got %v", length, valuesLen)
	}
}


func TestValuesContainsAllAdded(t *testing.T) {
		llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")
	_, _ = llru.AddOrUpdateLocked("new key3", "3")
	_, _ = llru.AddOrUpdateLocked("new key4", "4")

	values := llru.Values()

	contains1 := slices.Contains(values, "1")
	contains2 := slices.Contains(values, "2")
	contains3 := slices.Contains(values, "3")
	contains4 := slices.Contains(values, "4")

	if !contains1 || !contains2 || !contains3 || !contains4 {
		t.Errorf("expected to contain values but did not")
	}
}

func TestValuesAreOrdered(t *testing.T) {
	llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")
	_, _ = llru.AddOrUpdateLocked("new key3", "3")
	_, _ = llru.AddOrUpdateLocked("new key4", "4")

	values := llru.Values()

	isInCorrectPosition1 := values[0] == "1"
	isInCorrectPosition2 := values[1] == "2"
	isInCorrectPosition3 := values[2] == "3"
	isInCorrectPosition4 := values[3] == "4"

	if !isInCorrectPosition1 || !isInCorrectPosition2 || !isInCorrectPosition3 || !isInCorrectPosition4 {
		t.Errorf("expected values to be in correct order")
	}
}

func TestRemoveOldest(t *testing.T) {
	llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateUnlocked("new key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("new key2", "2")
	_, _ = llru.AddOrUpdateLocked("new key3", "3")
	_, _ = llru.AddOrUpdateLocked("new key4", "4")

	oldest := llru.RemoveOldest()

	if oldest == nil || oldest.Key != "new key1" || oldest.Value != "1" {
		t.Errorf("expected `Entry{Key: \"new key1\", Value: \"1\"}` but got %v", oldest)
	}
}


//If `newKey` does not exist, and there is at least one unlocked entry, replaces the key in the oldest entry with `newKey` and returns the oldest entry's value, the old key, and `true`
func TestReplaceOldestKeyCase1(t *testing.T) {
	llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateUnlocked("old key1", "1")
	_, _ = llru.AddOrUpdateUnlocked("old key2", "2")

	value, oldKey, ok := llru.ReplaceOldestKey("new key1")

	if !ok || *oldKey != "old key1" || *value != "1" {
		t.Errorf("expected `\"1\", \"old key1\", true` but got %v, %v, %v", value, oldKey, ok)
	}
}

//If `newKey` does not exist, and there are no unlocked entries, returns `nil, nil, false`
func TestReplaceOldestKeyCase2(t *testing.T) {
	llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateLocked("old key1", "1")

	value, oldKey, ok := llru.ReplaceOldestKey("new key1")

	if ok || oldKey != nil || value != nil {
		t.Errorf("expected `nil, nil, false` but got %v, %v, %v", value, oldKey, ok)
	}
}

//If `newKey` exists, returns `nil, nil, false`
func TestReplaceOldestKeyCase3(t *testing.T) {
	llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateLocked("new key1", "1")

	value, oldKey, ok := llru.ReplaceOldestKey("new key1")

	if ok || oldKey != nil || value != nil {
		t.Errorf("expected `nil, nil, false` but got %v, %v, %v", value, oldKey, ok)
	}
}

//If there is at least one unlocked entry, replaces the value in the oldest entry with `newValue` and returns the oldest entry's old value, the key, and `true`
func TestReplaceOldestValueCase1(t *testing.T) {
	llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateUnlocked("key1", "-1")
	_, _ = llru.AddOrUpdateUnlocked("key2", "2")

	oldValue, key, ok := llru.ReplaceOldestValue("1")

	if !ok || *oldValue != "-1" || *key != "key1" {
		t.Errorf("expected `\"-1\", \"key1\", true` but got %v, %v, %v", oldValue, key, ok)
	}
}

//If there are no unlocked entries, returns `nil, nil, false`
func TestReplaceOldestValueCase2(t *testing.T) {
	llru := buildNewEmpty(t, 4)

	_, _ = llru.AddOrUpdateLocked("old key1", "1")

	oldValue, key, ok := llru.ReplaceOldestValue("new value")
	
	if ok || oldValue != nil || key != nil {
		t.Errorf("expected `nil, nil, false` but got %v, %v, %v", oldValue, key, ok)
	}
}
