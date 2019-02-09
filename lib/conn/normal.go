package conn

import (
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/rate"
	"net"
)

type CryptConn struct {
	conn  net.Conn
	crypt bool
	rate  *rate.Rate
}

func NewCryptConn(conn net.Conn, crypt bool, rate *rate.Rate) *CryptConn {
	c := new(CryptConn)
	c.conn = conn
	c.crypt = crypt
	c.rate = rate
	return c
}

//加密写
func (s *CryptConn) Write(b []byte) (n int, err error) {
	n = len(b)
	if s.crypt {
		if b, err = crypt.AesEncrypt(b, []byte(cryptKey)); err != nil {
			return
		}
	}
	if b, err = GetLenBytes(b); err != nil {
		return
	}
	_, err = s.conn.Write(b)
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}

//解密读
func (s *CryptConn) Read(b []byte) (n int, err error) {
	var lens int
	var buf []byte
	var rb []byte
	c := NewConn(s.conn)
	if lens, err = c.GetLen(); err != nil {
		return
	}
	if buf, err = c.ReadLen(lens); err != nil {
		return
	}
	if s.crypt {
		if rb, err = crypt.AesDecrypt(buf, []byte(cryptKey)); err != nil {
			return
		}
	} else {
		rb = buf
	}
	copy(b, rb)
	n = len(rb)
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}
