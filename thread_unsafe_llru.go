package lockable_lru

/*
 * A thread-safe LRU implementation with items that can be "locked"
 *
 * Based on hashicorp/golang-lru. Uses orcaman/concurrent-map for locked items. Both of these underlying
 * structures are thread-safe, so if you really want ThreadunsafeLLRU to be faster, this needs to be refactored
 * to use non-locking underlying implementations. 
 * 
 * The only reason there is a thread-unsafe version of this LLRU is to separate concerns and keep the code clearn.
 * Thread safety is handled entirely in thread_safe_llru.go.
 *
 * A locked item cannot be evicted until it is unlocked. When it is unlocked, it is moved to the most recent.
 *
 */
import (
	lru "github.com/hashicorp/golang-lru/v2"
	gmap "github.com/wk8/go-ordered-map/v2"
)

type ThreadunsafeLLRU[K comparable, V any] struct {
	unlocked         *lru.Cache[K, V]							//unlocked k-v store whose values can be evicted when a new value is added
	locked						*gmap.OrderedMap[K,V]   //locked k-v store, whose values can never be evicted
	size int			                                //total size, combined locked and unlocked
}

type Entry[K comparable, V any] struct {
	Key K
	Value V
}

// New creates an LRU of the given size.
func NewUnsafe[K comparable, V any](size int) (*ThreadunsafeLLRU[K, V], error) {
	return NewUnsafeWithEvict[K, V](size, nil)
}

// NewWithEvict constructs a fixed size cache with the given eviction
// callback.
func NewUnsafeWithEvict[K comparable, V any](size int, onEvicted func(key K, value V)) (*ThreadunsafeLLRU[K, V], error) {
	lru, err := lru.NewWithEvict(size, onEvicted)
	if err != nil {	
		return nil, err
	}

	m := gmap.New[K,V]()
	llru := ThreadunsafeLLRU[K, V]{
		unlocked: lru,
		locked: m,
		size: size,
	}

	return &llru, nil
}

//modifies the passed LRU to add or update the key/value pair. If a value was evicted, returns it.
func addOrUpdate[K comparable, V any](lru *lru.Cache[K, V], key K, value V) (*Entry[K, V]) {
	oldestKey, oldestValue, _ := lru.GetOldest() //we can ignore the last parameter, which is false if the lru is empty
	wasEvicted := lru.Add(key, value)

	if wasEvicted {
		return &Entry[K, V]{Key: oldestKey, Value: oldestValue}
	} else {
		return nil
	}
}

//modifies the passed LRU to change its size. If one item was evicted, it is returned. If more than one is evicted, the oldest is returned
func resize[K comparable, V any](lru *lru.Cache[K, V], size int) (*Entry[K, V]) {
	oldestKey, oldestValue, _ := lru.GetOldest() //we can ignore the last parameter, which is false if the lru is empty
	numberEvicted := lru.Resize(size)

	if numberEvicted > 0 {
		return &Entry[K, V]{Key: oldestKey, Value: oldestValue}
	} else {
		return nil
	}
}

//return array of values from oldest to newest
func collectValues[K comparable, V any](gmap *gmap.OrderedMap[K,V]) []V {
	values := make([]V, gmap.Len())
	i := 0
	for pair := gmap.Oldest(); pair != nil; pair = pair.Next() {
		values[i] = pair.Value
		i++
	}
	return values
}

// Add adds an unlocked value to the cache. 
// If the key exists and is unlocked, its value is updated, making it the most recently used item, and `true, nil` is returned.
// If the key exists and is locked, its value is updated and it is unlocked, making it the most recently used item, and `true, nil` is returned.
// If the key does not exist and there is room, it is added, making it the most recently used item. If an entry was evicted, `true, entry` is returned, otherwise `true, nil` is returned.
// If the key does not exist and there is no room, `false, nil` is returned.
func (llru *ThreadunsafeLLRU[K, V]) AddOrUpdateUnlocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	llru.locked.Delete(key) //safe to do here, we'll never remove a value and then not have room

	hasRoom := llru.locked.Len() < llru.size
	if hasRoom {
		//in case we did remove from the locked values, resize the locked so we don't unnecessarily evict
		llru.unlocked.Resize(llru.size - llru.locked.Len())
		
		evicted = addOrUpdate(llru.unlocked, key, value)
	}

	ok = hasRoom
	return ok, evicted
}


// Add adds a locked value to the cache. 
// If the key exists and is locked, its value is updated, and `true, nil` is returned.
// If the key exists and is unlocked, its value is updated and it is locked, and `true, nil` is returned.
// If the key does not exist and there is room, it is added, making it the most recently used item. If an entry was evicted, `true, entry` is returned, otherwise `true, nil` is returned.
// If the key does not exist and there is no room, `false, nil` is returned.
func (llru *ThreadunsafeLLRU[K, V]) AddOrUpdateLocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	//instead of checking if the value already exists, which complicates the capacity check, just remove
	llru.locked.Delete(key)

	hasRoom := llru.locked.Len() < llru.size
	if hasRoom {
		llru.unlocked.Remove(key)
		llru.locked.Set(key, value)
		evicted = resize(llru.unlocked, llru.size - llru.locked.Len()) //recalculate size of unlocked in case we added a new value
	}

	ok = hasRoom
	return ok, evicted
}

// Locks an unlocked value in the cache. 
// If the key exists and is unlocked, it is locked, and `true` is returned
// If the key exists and is locked, `true` is returned
// If the key does not exist, returns `false`
func (llru *ThreadunsafeLLRU[K, V]) Lock(key K) (ok bool) {
	value, exists := llru.unlocked.Get(key)
	if !exists {
		_, exists = llru.locked.Get(key)
		return exists
	}
	llru.unlocked.Remove(key)
	llru.locked.Set(key, value)

	//resize unlocked
	resize(llru.unlocked, llru.size - llru.locked.Len())

	return true
}

// Unlocks a locked value in the cache. 
// If the key exists and is locked, it is unlocked, making it the most recently used item, and `true` is returned
// If the key exists and is unlocked, it becomes the most recently used item, and `true` is returned
// If the key does not exist, returns `false`
func (llru *ThreadunsafeLLRU[K, V]) Unlock(key K) (ok bool) {
	value, exists := llru.locked.Get(key)
	if !exists {
		_, exists = llru.unlocked.Get(key)
		return exists
	}
	llru.locked.Delete(key)

	//grow unlocked to prevent unnecessary eviction prior to adding the new value
	resize(llru.unlocked, llru.size - llru.locked.Len())

	llru.unlocked.Add(key, value)

	return true
}

// If the key exists and is locked, the value is returned
// If the key exists and is unlocked, it becomes the most recently used item, and the value is returned
// If the key does not exist, `nil` is returned
func (llru *ThreadunsafeLLRU[K, V]) Get(key K) (value *V) {
	val, exists := llru.locked.Get(key)
	if exists {
		return &val
	} else {
		val, exists = llru.unlocked.Get(key)
		if exists {
			return &val
		} else {
			return nil
		}
	}
}

// If the key exists, true is returned. The recentness of the item is unchanged
// If the key does not exist, false is returned. 
func (llru *ThreadunsafeLLRU[K, V]) Contains(key K) bool {
	inUnlocked := llru.unlocked.Contains(key)
	if inUnlocked {
		return inUnlocked
	}
	_, inLocked := llru.locked.Get(key)

	return inLocked
}

// Returns the number of entries
func (llru *ThreadunsafeLLRU[K, V]) Len() int {
	return llru.locked.Len() + llru.unlocked.Len()
}

// Returns an array of every value, starting with unlocked from oldest to newest, then locked
func (llru *ThreadunsafeLLRU[K, V]) Values() []V {
	unlockedValues := llru.unlocked.Values()
	lockedValues := collectValues(llru.locked)

	return append(unlockedValues, lockedValues...)
}

func (llru *ThreadunsafeLLRU[K, V]) RemoveOldest() *Entry[K, V] {
	oldestKey, oldestValue, ok := llru.unlocked.RemoveOldest()

	if ok {
		return &Entry[K, V]{
			Key: oldestKey,
			Value: oldestValue,
		}
	}
	return nil
}

//If `newKey` does not exist, and there is at least one unlocked entry, replaces the key in the oldest entry with `newKey` and returns the oldest entry's value, the old key, and `true`
//If `newKey` does not exist, and there are no unlocked entries, returns `nil, nil, false`
//If `newKey` exists, returns `nil, nil, false`
func (llru *ThreadunsafeLLRU[K, V]) ReplaceOldestKey(newKey K) (value *V, oldKey *K, ok bool) {
	contains := llru.Contains(newKey)
	
	if !contains { //error if key exists
		oldestKey, oldestValue, ok := llru.unlocked.RemoveOldest()

		if ok {
			ok, _ = llru.AddOrUpdateUnlocked(newKey, oldestValue)
			return &oldestValue, &oldestKey, ok
		}
	}

	return nil, nil, false
}

//If there is at least one unlocked entry, replaces the value in the oldest entry with `newValue` and returns the oldest entry's old value, the key, and `true`
//If there are no unlocked entries, returns `nil, nil, false`
func (llru *ThreadunsafeLLRU[K, V]) ReplaceOldestValue(newValue V) (oldValue *V, key *K, ok bool) {
	oldestKey, oldestValue, ok := llru.unlocked.RemoveOldest()

	if ok {
		ok, _ = llru.AddOrUpdateUnlocked(oldestKey, newValue)
		return &oldestValue, &oldestKey, ok
	}

	return nil, nil, false
}
