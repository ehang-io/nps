package file

import "testing"

// A secret or p2p tunnel has no port, so a client re-registering its own key
// must still be recognised as the owner; otherwise a key orphaned by a lost
// "OK" reply is refused as a duplicate for as long as the client is offline.
func TestHasTunnel(t *testing.T) {
	once.Do(func() { Db = &DbUtils{JsonDb: &JsonDb{}} })
	tasks := &Db.JsonDb.Tasks
	owner, other := &Client{Id: 1}, &Client{Id: 2}
	tasks.Store(101, &Tunnel{Id: 101, Mode: "secret", Password: "s1", Client: owner})
	tasks.Store(102, &Tunnel{Id: 102, Mode: "p2p", Password: "p1", Client: owner})
	tasks.Store(103, &Tunnel{Id: 103, Mode: "tcp", Port: 10000, Client: owner})
	defer func() {
		for _, id := range []int{101, 102, 103} {
			tasks.Delete(id)
		}
	}()

	cases := []struct {
		name   string
		client *Client
		t      *Tunnel
		want   bool
	}{
		{"own secret key", owner, &Tunnel{Mode: "secret", Password: "s1"}, true},
		{"own p2p key", owner, &Tunnel{Mode: "p2p", Password: "p1"}, true},
		{"own tcp port", owner, &Tunnel{Mode: "tcp", Port: 10000}, true},
		{"another client's secret key", other, &Tunnel{Mode: "secret", Password: "s1"}, false},
		{"another client's tcp port", other, &Tunnel{Mode: "tcp", Port: 10000}, false},
		{"new secret key", owner, &Tunnel{Mode: "secret", Password: "s2"}, false},
		{"same key, other mode", owner, &Tunnel{Mode: "p2p", Password: "s1"}, false},
		{"portless non-keyed mode", owner, &Tunnel{Mode: "tcp", Port: 0}, false},
		{"empty key does not match a keyed tunnel of another mode", owner, &Tunnel{Mode: "secret"}, false},
	}
	for _, c := range cases {
		if got := c.client.HasTunnel(c.t); got != c.want {
			t.Errorf("%s: HasTunnel = %v, want %v", c.name, got, c.want)
		}
	}
	if own := owner.OwnTunnel(&Tunnel{Mode: "secret", Password: "s1"}); own == nil || own.Id != 101 {
		t.Errorf("OwnTunnel(own secret key) = %v, want tunnel 101", own)
	}
}
