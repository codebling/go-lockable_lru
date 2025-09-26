package lockable_lru

/*
 * A thread-safe LRU implementation with items that can be "locked"
 *
 * Based on hashicorp/golang-lru
 *
 * A locked item cannot be evicted until it is unlocked. When it is unlocked, it is moved to the most recent.
 *
 */
import (
	lru "github.com/hashicorp/golang-lru/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
)

type ThreadunsafeLLRU[K cmap.Stringer, V any] struct {
	unlocked         *lru.Cache[K, V]							//unlocked k-v store whose values can be evicted when a new value is added
	locked						 cmap.ConcurrentMap[K, V]   //locked k-v store, whose values can never be evicted
	size int			                                //total size, combined locked and unlocked
}

type Entry[K cmap.Stringer, V any] struct {
	Key K
	Value V
}

// New creates an LRU of the given size.
func NewUnsafe[K cmap.Stringer, V any](size int) (*ThreadunsafeLLRU[K, V], error) {
	return NewUnsafeWithEvict[K, V](size, nil)
}

// NewWithEvict constructs a fixed size cache with the given eviction
// callback.
func NewUnsafeWithEvict[K cmap.Stringer, V any](size int, onEvicted func(key K, value V)) (*ThreadunsafeLLRU[K, V], error) {
	lru, err := lru.NewWithEvict(size, onEvicted)
	if err != nil {	
		return nil, err
	}

	m := cmap.NewStringer[K, V]()
	llru := ThreadunsafeLLRU[K, V]{
		unlocked: lru,
		locked: m,
		size: size,
	}

	return &llru, nil
}

// Add adds an unlocked value to the cache. If a value was evicted, returns it.
func (llru *ThreadunsafeLLRU[K, V]) addOrUpdateUnlockedWithoutLockingNorCheckingCapacity(key K, value V) (*Entry[K, V]) {
	oldestKey, oldestValue, _ := llru.unlocked.GetOldest() //we can ignore the last parameter, which is false if the lru is empty
	wasEvicted := llru.unlocked.Add(key, value)

	if wasEvicted {
		return &Entry[K, V]{Key: oldestKey, Value: oldestValue}
	} else {
		return nil
	}
}

// Add adds an unlocked value to the cache. 
// If the value exists, it is updated. If it existed and was locked, it is unlocked.
// Returns `false, nil` if there was no room, otherwise returns true and the evicted entry, if any
func (llru *ThreadunsafeLLRU[K, V]) AddOrUpdateUnlocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	llru.locked.Remove(key) //safe to do here, we'll never remove a value and then not have room

	hasRoom := llru.locked.Count() < llru.size
	if hasRoom {
		//in case we did remove from the locked values, resize the locked so we don't unnecessarily evict
		llru.unlocked.Resize(llru.size - llru.locked.Count())
		
		evicted = llru.addOrUpdateUnlockedWithoutLockingNorCheckingCapacity()
	}

	ok = hasRoom
	return ok, evicted
}


// Add adds a locked value to the cache. 
// If the value exists, it is updated. If it existed and was unlocked, it is locked.
// Returns `false, nil` if there was no room, otherwise returns true and the evicted entry, if any
func (llru *ThreadunsafeLLRU[K, V]) AddOrUpdateLocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	hasRoom := llru.locked.Count() < llru.size
	if hasRoom {
		llru.unlocked.Remove(key)

		
		evicted = llru.addOrUpdateUnlockedWithoutLockingNorCheckingCapacity(key, value)
	}

	ok = hasRoom
	return ok, evicted
}
