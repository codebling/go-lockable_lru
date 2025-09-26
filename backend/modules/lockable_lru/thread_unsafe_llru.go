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
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
)

type threadunsafeLLRU[K cmap.Stringer, V any] struct {
	unlocked         *lru.Cache[K, V]							//unlocked k-v store whose values can be evicted when a new value is added
	locked						 cmap.ConcurrentMap[K, V]   //locked k-v store, whose values can never be evicted
	size int			                                //total size, combined locked and unlocked
	lock        sync.RWMutex                      //even though the underlying structures are threadsafe, we need to lock if we have to do 2 or more operations - which means we have to lock for every operation, otherwise we could deadlock if one call has locked the outer lock but is waiting on the inner lock, and another call has not locked the outer but has locked the inner
}

type Entry[K cmap.Stringer, V any] struct {
	Key K
	Value V
}

// New creates an LRU of the given size.
func New[K cmap.Stringer, V any](size int) (*threadunsafeLLRU[K, V], error) {
	return NewWithEvict[K, V](size, nil)
}

// NewWithEvict constructs a fixed size cache with the given eviction
// callback.
func NewWithEvict[K cmap.Stringer, V any](size int, onEvicted func(key K, value V)) (*threadunsafeLLRU[K, V], error) {
	lru, err := lru.NewWithEvict(size, onEvicted)
	if err != nil {	
		return nil, err
	}

	m := cmap.NewStringer[K, V]()
	llru := threadunsafeLLRU[K, V]{
		unlocked: lru,
		locked: m,
		size: size,
	}

	return &llru, nil
}

// Add adds an unlocked value to the cache. If a value was evicted, returns it.
func (llru *threadunsafeLLRU[K, V]) addUnlockedWithoutLockingNorCheckingCapacity(key K, value V) (*Entry[K, V]) {
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
func (llru *threadunsafeLLRU[K, V]) AddOrUpdateUnlocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	llru.lock.Lock()
	defer llru.lock.Unlock()
	
	llru.locked.Remove(key) //safe to do here, we'll never remove a value and then not have room

	hasRoom := llru.locked.Count() < llru.size
	if hasRoom {
		//in case we did remove from the locked values, resize the locked so we don't unnecessarily evict
		llru.unlocked.Resize(llru.size - llru.locked.Count())
		
		evicted = llru.addUnlockedWithoutLockingNorCheckingCapacity()
	}

	ok = hasRoom
	return ok, evicted
}


// Add adds a locked value to the cache. 
// If the value exists, it is updated. If it existed and was unlocked, it is locked.
// Returns `false, nil` if there was no room, otherwise returns true and the evicted entry, if any
func (llru *threadunsafeLLRU[K, V]) AddOrUpdateLocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	llru.lock.Lock()
	defer llru.lock.Unlock()
	
	hasRoom := llru.locked.Count() < llru.size
	if hasRoom {
		llru.unlocked.Remove(key)

		
		evicted = llru.addUnlockedWithoutLockingNorCheckingCapacity()
	}

	ok = hasRoom
	return ok, evicted
}

// Contains checks if a key is in the cache, without updating the
// recent-ness or deleting it for being stale.
func (llru *threadunsafeLLRU[K, V]) Contains(key K) bool {
	c.lock.RLock()
	containKey := c.lru.Contains(key)
	c.lock.RUnlock()
	return containKey
}

// Peek returns the key value (or undefined if not found) without updating
// the "recently used"-ness of the key.
func (llru *threadunsafeLLRU[K, V]) Peek(key K) (value V, ok bool) {
	c.lock.RLock()
	value, ok = c.lru.Peek(key)
	c.lock.RUnlock()
	return value, ok
}

// ContainsOrAdd checks if a key is in the cache without updating the
// recent-ness or deleting it for being stale, and if not, adds the value.
// Returns whether found and whether an eviction occurred.
func (llru *threadunsafeLLRU[K, V]) ContainsOrAdd(key K, value V) (ok, evicted bool) {
	var k K
	var v V
	c.lock.Lock()
	if c.lru.Contains(key) {
		c.lock.Unlock()
		return true, false
	}
	evicted = c.lru.Add(key, value)
	if c.onEvictedCB != nil && evicted {
		k, v = c.evictedKeys[0], c.evictedVals[0]
		c.evictedKeys, c.evictedVals = c.evictedKeys[:0], c.evictedVals[:0]
	}
	c.lock.Unlock()
	if c.onEvictedCB != nil && evicted {
		c.onEvictedCB(k, v)
	}
	return false, evicted
}

// PeekOrAdd checks if a key is in the cache without updating the
// recent-ness or deleting it for being stale, and if not, adds the value.
// Returns whether found and whether an eviction occurred.
func (llru *threadunsafeLLRU[K, V]) PeekOrAdd(key K, value V) (previous V, ok, evicted bool) {
	var k K
	var v V
	c.lock.Lock()
	previous, ok = c.lru.Peek(key)
	if ok {
		c.lock.Unlock()
		return previous, true, false
	}
	evicted = c.lru.Add(key, value)
	if c.onEvictedCB != nil && evicted {
		k, v = c.evictedKeys[0], c.evictedVals[0]
		c.evictedKeys, c.evictedVals = c.evictedKeys[:0], c.evictedVals[:0]
	}
	c.lock.Unlock()
	if c.onEvictedCB != nil && evicted {
		c.onEvictedCB(k, v)
	}
	return
}

// Remove removes the provided key from the cache.
func (llru *threadunsafeLLRU[K, V]) Remove(key K) (present bool) {
	var k K
	var v V
	c.lock.Lock()
	present = c.lru.Remove(key)
	if c.onEvictedCB != nil && present {
		k, v = c.evictedKeys[0], c.evictedVals[0]
		c.evictedKeys, c.evictedVals = c.evictedKeys[:0], c.evictedVals[:0]
	}
	c.lock.Unlock()
	if c.onEvictedCB != nil && present {
		c.onEvictedCB(k, v)
	}
	return
}

// Resize changes the cache size.
func (llru *threadunsafeLLRU[K, V]) Resize(size int) (evicted int) {
	var ks []K
	var vs []V
	c.lock.Lock()
	evicted = c.lru.Resize(size)
	if c.onEvictedCB != nil && evicted > 0 {
		ks, vs = c.evictedKeys, c.evictedVals
		c.initEvictBuffers()
	}
	c.lock.Unlock()
	if c.onEvictedCB != nil && evicted > 0 {
		for i := 0; i < len(ks); i++ {
			c.onEvictedCB(ks[i], vs[i])
		}
	}
	return evicted
}

// RemoveOldest removes the oldest item from the cache.
func (llru *threadunsafeLLRU[K, V]) RemoveOldest() (key K, value V, ok bool) {
	var k K
	var v V
	c.lock.Lock()
	key, value, ok = c.lru.RemoveOldest()
	if c.onEvictedCB != nil && ok {
		k, v = c.evictedKeys[0], c.evictedVals[0]
		c.evictedKeys, c.evictedVals = c.evictedKeys[:0], c.evictedVals[:0]
	}
	c.lock.Unlock()
	if c.onEvictedCB != nil && ok {
		c.onEvictedCB(k, v)
	}
	return
}

// GetOldest returns the oldest entry
func (llru *threadunsafeLLRU[K, V]) GetOldest() (key K, value V, ok bool) {
	c.lock.RLock()
	key, value, ok = c.lru.GetOldest()
	c.lock.RUnlock()
	return
}

// Keys returns a slice of the keys in the cache, from oldest to newest.
func (llru *threadunsafeLLRU[K, V]) Keys() []K {
	c.lock.RLock()
	keys := c.lru.Keys()
	c.lock.RUnlock()
	return keys
}

// Values returns a slice of the values in the cache, from oldest to newest.
func (llru *threadunsafeLLRU[K, V]) Values() []V {
	c.lock.RLock()
	values := c.lru.Values()
	c.lock.RUnlock()
	return values
}

// Len returns the number of items in the cache.
func (llru *threadunsafeLLRU[K, V]) Len() int {
	c.lock.RLock()
	length := c.lru.Len()
	c.lock.RUnlock()
	return length
}