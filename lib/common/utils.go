package common

import (
	"encoding/binary"
	"github.com/pkg/errors"
	"io"
	"strings"
)

// CopyBuffer is an implement of io.Copy with buffer pool
func CopyBuffer(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = errors.New("short write")
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

// HostContains tests whether the string host contained ruleHost
func HostContains(ruleHost string, host string) bool {
	return strings.HasSuffix(host, strings.Replace(ruleHost, "*", "", -1))
}

// WriteLenBytes is used to write length and bytes to writer
func WriteLenBytes(w io.Writer, b []byte) (int, error) {
	err := binary.Write(w, binary.LittleEndian, uint32(len(b)))
	if err != nil {
		return 0, errors.Wrap(err, "write len")
	}
	n, err := w.Write(b)
	if err != nil {
		return 0, errors.Wrap(err, "write bytes")
	}
	return n, nil
}

// ReadLenBytes is used to read bytes from reader
func ReadLenBytes(r io.Reader, b []byte) (int, error) {
	var l int32
	err := binary.Read(r, binary.LittleEndian, &l)
	if err != nil {
		return 0, errors.Wrap(err, "read len")
	}
	if int(l) > len(b) {
		return 0, errors.Errorf("data is too long(%d)", l)
	}
	n, err := io.ReadAtLeast(r, b, int(l))
	if err != nil {
		return n, errors.Wrap(err, "read data error")
	}
	return n, nil
}
