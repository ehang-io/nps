package conn

import (
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/vender/github.com/golang/snappy"
	"github.com/fatedier/frp/utils/net"
	"io"
)

type SnappyConn struct {
	w *snappy.Writer
	r *snappy.Reader
}

func NewSnappyConn(conn io.ReadWriteCloser) net.Conn {
	c := new(SnappyConn)
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	return c
}

//snappy压缩写
func (s *SnappyConn) Write(b []byte) (n int, err error) {
	if n, err = s.w.Write(b); err != nil {
		return
	}
	if err = s.w.Flush(); err != nil {
		return
	}
	return
}

//snappy压缩读
func (s *SnappyConn) Read(b []byte) (n int, err error) {
	buf := pool.BufPool.Get().([]byte)
	defer pool.BufPool.Put(buf)
	if n, err = s.r.Read(buf); err != nil {
		return
	}
	copy(b, buf[:n])
	return
}

func (s *SnappyConn) Close() error {
	s.w.Close()
	return s.w.Close()
}
