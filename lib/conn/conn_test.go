package conn

import (
	"net"
	"testing"
)

//回归测试：ReadFlag 必须返回真正读到的 4 字节 flag。
//原实现写在同一个 return 语句里（string(buf) 与 binary.Read 有求值顺序依赖），
//一旦 string(buf) 先求值就会永远返回 4 个 0 字节，导致桥接连接的类型判断全部失效。
func TestReadFlag(t *testing.T) {
	for _, want := range []string{"main", "chan", "conf", "file"} {
		server, client := net.Pipe()
		go func(flag string) {
			if _, err := client.Write([]byte(flag)); err != nil {
				t.Errorf("write flag error: %v", err)
			}
			client.Close()
		}(want)

		c := NewConn(server)
		got, err := c.ReadFlag()
		server.Close()
		if err != nil {
			t.Fatalf("ReadFlag error: %v", err)
		}
		if got != want {
			t.Fatalf("ReadFlag = %q, want %q", got, want)
		}
	}
}

func TestWriteLenContentAndGetShortLenContent(t *testing.T) {
	a, b := net.Pipe()
	payload := []byte("hello-nps")
	go func() {
		if err := NewConn(a).WriteLenContent(payload); err != nil {
			t.Errorf("WriteLenContent error: %v", err)
		}
		a.Close()
	}()

	got, err := NewConn(b).GetShortLenContent()
	if err != nil {
		t.Fatalf("GetShortLenContent error: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}
