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
)

type LLRU[K comparable, V any] struct {
	tullru ThreadunsafeLLRU[K, V]
	lock sync.RWMutex //even though the underlying structures are threadsafe, we need to lock if we have to do 2 or more operations - which means we have to lock for every operation, otherwise we could deadlock if one call has locked the outer lock but is waiting on the inner lock, and another call has not locked the outer but has locked the inner
}

// New creates an LRU of the given size.
func New[K comparable, V any](size int) (*LLRU[K, V], error) {
	tullru, err := NewUnsafe[K, V](size)
	if err != nil {
		return nil, err
	}
	return &LLRU[K, V]{
		tullru: *tullru,
	}, nil
}

// NewWithEvict constructs a fixed size cache with the given eviction
// callback.
func NewWithEvict[K comparable, V any](size int, onEvicted func(key K, value V)) (*LLRU[K, V], error) {
	tullru, err := NewUnsafeWithEvict(size, onEvicted)
	if err != nil {
		return nil, err
	}
	return &LLRU[K, V]{
		tullru: *tullru,
	}, nil
}

// Add adds an unlocked value to the cache.
// If the value exists, it is updated. If it existed and was locked, it is unlocked.
// Returns `false, nil` if there was no room, otherwise returns true and the evicted entry, if any
func (llru *LLRU[K, V]) AddOrUpdateUnlocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	llru.lock.Lock()
	defer llru.lock.Unlock()
	return llru.tullru.AddOrUpdateUnlocked(key, value)
}

// Add adds a locked value to the cache.
// If the value exists, it is updated. If it existed and was unlocked, it is locked.
// Returns `false, nil` if there was no room, otherwise returns true and the evicted entry, if any
func (llru *LLRU[K, V]) AddOrUpdateLocked(key K, value V) (ok bool, evicted *Entry[K, V]) {
	llru.lock.Lock()
	defer llru.lock.Unlock()
	return llru.tullru.AddOrUpdateLocked(key, value)
}

func (llru *LLRU[K, V]) Lock(key K) (ok bool) {
	llru.lock.Lock()
	defer llru.lock.Unlock()
	return llru.tullru.Lock(key)
}

func (llru *LLRU[K, V]) Unlock(key K) (ok bool) {
	llru.lock.Lock()
	defer llru.lock.Unlock()
	return llru.tullru.Unlock(key)
}

func (llru *LLRU[K, V]) Get(key K) (value *V) {
	llru.lock.Lock()
	defer llru.lock.Unlock()
	return llru.tullru.Get(key)
}