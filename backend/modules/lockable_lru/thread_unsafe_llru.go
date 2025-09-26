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
		llru.unlocked.Resize(llru.size - llru.locked.Count())
		
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
		evicted = resize(llru.unlocked, llru.size - llru.locked.Count()) //recalculate size of unlocked in case we added a new value
	}

	ok = hasRoom
	return ok, evicted
}

// Locks an unlocked value in the cache. 
// If the key exists and is unlocked, it is locked, making it the most recently used item, and `true` is returned
// If the key exists and is locked, `true` is returned
// Returns `false, nil` if the key did not exist, otherwise returns true
func (llru *ThreadunsafeLLRU[K, V]) Lock(key K) (ok bool) {
	value, exists := llru.unlocked.Get(key)
	if !exists {
		_, exists = llru.locked.Get(key)
		return exists
	}
	llru.unlocked.Remove(key)
	llru.locked.Set(key, value)
	return true
}
