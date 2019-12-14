package conn

import (
	"io"

	"github.com/golang/snappy"
)

type SnappyConn struct {
	w *snappy.Writer
	r *snappy.Reader
}

func NewSnappyConn(conn io.ReadWriteCloser) *SnappyConn {
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
	return s.r.Read(b)
}

func (s *SnappyConn) Close() error {
	s.w.Close()
	return s.w.Close()
}
