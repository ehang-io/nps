package conn

import (
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/lib/rate"
	"github.com/cnlh/nps/lib/snappy"
	"log"
	"net"
)

type SnappyConn struct {
	w     *snappy.Writer
	r     *snappy.Reader
	crypt bool
	rate  *rate.Rate
}

func NewSnappyConn(conn net.Conn, crypt bool, rate *rate.Rate) *SnappyConn {
	c := new(SnappyConn)
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	c.crypt = crypt
	c.rate = rate
	return c
}

//snappy压缩写 包含加密
func (s *SnappyConn) Write(b []byte) (n int, err error) {
	n = len(b)
	if s.crypt {
		if b, err = crypt.AesEncrypt(b, []byte(cryptKey)); err != nil {
			lg.Println("encode crypt error:", err)
			return
		}
	}
	if _, err = s.w.Write(b); err != nil {
		return
	}
	if err = s.w.Flush(); err != nil {
		return
	}
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}

//snappy压缩读 包含解密
func (s *SnappyConn) Read(b []byte) (n int, err error) {
	buf := pool.BufPool.Get().([]byte)
	defer pool.BufPool.Put(buf)
	if n, err = s.r.Read(buf); err != nil {
		return
	}
	var bs []byte
	if s.crypt {
		if bs, err = crypt.AesDecrypt(buf[:n], []byte(cryptKey)); err != nil {
			log.Println("decode crypt error:", err)
			return
		}
	} else {
		bs = buf[:n]
	}
	n = len(bs)
	copy(b, bs)
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}
