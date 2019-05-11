package cache

import (
	"container/list"
	"sync"
)

// Cache is an LRU cache. It is safe for concurrent access.
type Cache struct {
	// MaxEntries is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	MaxEntries int

	//Execute this callback function when an element is culled
	OnEvicted func(key Key, value interface{})

	ll    *list.List //list
	cache sync.Map
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
		//cache:      make(map[interface{}]*list.Element),
	}
}

// If the key value already exists, move the key to the front
func (c *Cache) Add(key Key, value interface{}) {
	if ee, ok := c.cache.Load(key); ok {
		c.ll.MoveToFront(ee.(*list.Element)) // move to the front
		ee.(*list.Element).Value.(*entry).value = value
		return
	}
	ele := c.ll.PushFront(&entry{key, value})
	c.cache.Store(key, ele)
	if c.MaxEntries != 0 && c.ll.Len() > c.MaxEntries { // Remove the oldest element if the limit is exceeded
		c.RemoveOldest()
	}
}

// Get looks up a key's value from the cache.
func (c *Cache) Get(key Key) (value interface{}, ok bool) {
	if ele, hit := c.cache.Load(key); hit {
		c.ll.MoveToFront(ele.(*list.Element))
		return ele.(*list.Element).Value.(*entry).value, true
	}
	return
}

// Remove removes the provided key from the cache.
func (c *Cache) Remove(key Key) {
	if ele, hit := c.cache.Load(key); hit {
		c.removeElement(ele.(*list.Element))
	}
}

// RemoveOldest removes the oldest item from the cache.
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry)
	c.cache.Delete(kv.key)
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)
	}
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	return c.ll.Len()
}

// Clear purges all stored items from the cache.
func (c *Cache) Clear() {
	if c.OnEvicted != nil {
		c.cache.Range(func(key, value interface{}) bool {
			kv := value.(*list.Element).Value.(*entry)
			c.OnEvicted(kv.key, kv.value)
			return true
		})
	}
	c.ll = nil
}
