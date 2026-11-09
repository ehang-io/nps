package cache

import (
	"container/list"
	"sync"
)

// Cache is an LRU cache. It is safe for concurrent access.
// 实现说明：container/list 不是并发安全的，原来的 sync.Map + list 组合在
// 并发 Add/Get 时会同时修改链表（MoveToFront / PushFront / Remove），
// 可能损坏链表结构；这里统一用一把互斥锁保护链表与 map。
type Cache struct {
	// MaxEntries is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	MaxEntries int

	//Execute this callback function when an element is culled
	OnEvicted func(key Key, value interface{})

	mu    sync.Mutex
	ll    *list.List //list
	cache map[Key]*list.Element
}

// A Key may be any value that is comparable. See http://golang.org/ref/spec#Comparison_operators
type Key interface{}

type entry struct {
	key   Key
	value interface{}
}

// New creates a new Cache.
// If maxEntries is 0, the cache has no length limit.
// that eviction is done by the caller.
func New(maxEntries int) *Cache {
	return &Cache{
		MaxEntries: maxEntries,
		ll:         list.New(),
		cache:      make(map[Key]*list.Element),
	}
}

// If the key value already exists, move the key to the front
func (c *Cache) Add(key Key, value interface{}) {
	var (
		evicted    bool
		evictedKey Key
		evictedVal interface{}
	)
	c.mu.Lock()
	if ee, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ee) // move to the front
		ee.Value.(*entry).value = value
		c.mu.Unlock()
		return
	}
	ele := c.ll.PushFront(&entry{key, value})
	c.cache[key] = ele
	if c.MaxEntries != 0 && c.ll.Len() > c.MaxEntries { // Remove the oldest element if the limit is exceeded
		if oldest := c.ll.Back(); oldest != nil {
			c.ll.Remove(oldest)
			e := oldest.Value.(*entry)
			delete(c.cache, e.key)
			evicted, evictedKey, evictedVal = true, e.key, e.value
		}
	}
	c.mu.Unlock()
	//回调放在锁外，避免回调里再次访问 Cache 造成死锁
	if evicted && c.OnEvicted != nil {
		c.OnEvicted(evictedKey, evictedVal)
	}
}

// Get looks up a key's value from the cache.
func (c *Cache) Get(key Key) (value interface{}, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return nil, false
}

// Remove removes the provided key from the cache.
func (c *Cache) Remove(key Key) {
	var (
		removed    bool
		removedKey Key
		removedVal interface{}
	)
	c.mu.Lock()
	if ele, hit := c.cache[key]; hit {
		c.ll.Remove(ele)
		e := ele.Value.(*entry)
		delete(c.cache, e.key)
		removed, removedKey, removedVal = true, e.key, e.value
	}
	c.mu.Unlock()
	if removed && c.OnEvicted != nil {
		c.OnEvicted(removedKey, removedVal)
	}
}

// RemoveOldest removes the oldest item from the cache.
func (c *Cache) RemoveOldest() {
	var (
		removed    bool
		removedKey Key
		removedVal interface{}
	)
	c.mu.Lock()
	if ele := c.ll.Back(); ele != nil {
		c.ll.Remove(ele)
		e := ele.Value.(*entry)
		delete(c.cache, e.key)
		removed, removedKey, removedVal = true, e.key, e.value
	}
	c.mu.Unlock()
	if removed && c.OnEvicted != nil {
		c.OnEvicted(removedKey, removedVal)
	}
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

// Clear purges all stored items from the cache.
func (c *Cache) Clear() {
	type kv struct {
		key   Key
		value interface{}
	}
	var items []kv
	c.mu.Lock()
	for e := c.ll.Front(); e != nil; e = e.Next() {
		ent := e.Value.(*entry)
		items = append(items, kv{ent.key, ent.value})
	}
	c.ll.Init()
	c.cache = make(map[Key]*list.Element)
	c.mu.Unlock()
	if c.OnEvicted != nil {
		for _, item := range items {
			c.OnEvicted(item.key, item.value)
		}
	}
}
