package mux

import (
	"bytes"
	"encoding/binary"
	"io"
)

//write bytes with int32 length
func WriteLenBytes(buf []byte, w io.Writer) (int, error) {
	raw := bytes.NewBuffer([]byte{})
	if err := binary.Write(raw, binary.LittleEndian, int32(len(buf))); err != nil {
		return 0, err
	}
	if err := binary.Write(raw, binary.LittleEndian, buf); err != nil {
		return 0, err
	}
	return w.Write(raw.Bytes())
}

//read bytes by length
func ReadLenBytes(buf []byte, r io.Reader) (int, error) {
	var l uint32
	var err error
	if binary.Read(r, binary.LittleEndian, &l) != nil {
		return 0, err
	}
	if _, err = io.ReadFull(r, buf[:l]); err != nil {
		return 0, err
	}
	return int(l), nil
}
