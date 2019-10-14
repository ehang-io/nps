package core

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
)

func CopyBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := CopyBuff.Get()
	defer CopyBuff.Put(buf)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

func SendInfo(conn net.Conn, t interface{}) (int, error) {
	/*
		The task info is formed as follows:
		+----+-----+---------+
		|type| len | content |
		+----+---------------+
		| 4  |  4  |   ...   |
		+----+---------------+
	*/
	raw := bytes.NewBuffer([]byte{})
	b, err := json.Marshal(t)
	if err != nil {
		return 0, err
	}
	lenBytes, err := GetLenBytes(b)
	if err != nil {
		return 0, err
	}
	binary.Write(raw, binary.LittleEndian, lenBytes)
	return conn.Write(raw.Bytes())
}

// get the assembled amount data(len 4 and content)
func GetLenBytes(buf []byte) (b []byte, err error) {
	raw := bytes.NewBuffer([]byte{})
	if err = binary.Write(raw, binary.LittleEndian, int32(len(buf))); err != nil {
		return
	}
	if err = binary.Write(raw, binary.LittleEndian, buf); err != nil {
		return
	}
	b = raw.Bytes()
	return
}
