package lb

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type test struct {
	id int
}

func TestLb(t *testing.T) {
	lb := NewLoadBalancer()

	for i := 1; i <= 100; i++ {
		err := lb.SetClient("test", &test{id: i})
		assert.NoError(t, err)
	}

	m := make(map[int]bool)

	for i := 1; i <= 100; i++ {
		tt, err := lb.GetClient("test")
		assert.NoError(t, err)
		if _, ok := m[tt.(*test).id]; ok {
			t.Fail()
		}
		m[tt.(*test).id] = true
	}

	for i := 1; i <= 50; i++ {
		tt, err := lb.GetClient("test")
		assert.NoError(t, err)
		err = lb.RemoveClient("test", tt)
		assert.NoError(t, err)
	}

	m = make(map[int]bool)
	for i := 1; i <= 50; i++ {
		tt, err := lb.GetClient("test")
		assert.NoError(t, err)
		if _, ok := m[tt.(*test).id]; ok {
			t.Fail()
		}
		m[tt.(*test).id] = true
	}


}
