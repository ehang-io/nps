package cache

import (
	"strconv"
	"sync"
	"testing"
)

func TestCacheEvictsOldest(t *testing.T) {
	c := New(2)
	c.Add("a", 1)
	c.Add("b", 2)
	if v, ok := c.Get("a"); !ok || v.(int) != 1 {
		t.Fatalf("Get(a) = %v, %v", v, ok)
	}
	//a 刚被访问，淘汰的是 b
	c.Add("c", 3)
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
	if _, ok := c.Get("b"); ok {
		t.Error("b 应当被淘汰")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("a 不应当被淘汰")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("c 不应当被淘汰")
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	//配合 go test -race 使用：原实现用 sync.Map + 无锁链表，并发下会损坏链表
	c := New(64)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				key := "k" + strconv.Itoa(j%32)
				c.Add(key, j)
				c.Get(key)
				if j%7 == 0 {
					c.Remove(key)
				}
				if j%97 == 0 {
					c.RemoveOldest()
				}
			}
		}(i)
	}
	wg.Wait()
	if c.Len() > 64 {
		t.Fatalf("Len = %d, want <= 64", c.Len())
	}
}
