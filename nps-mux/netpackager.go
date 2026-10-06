package nps_mux

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

type basePackager struct {
	buf []byte
	// buf contain the mux protocol struct binary data, we copy data to buf firstly.
	// replace binary.Read/Write method, it may use reflect shows slowly.
	// also reduce conn.Read/Write calls which use syscall.
	// due to our test, conn.Write method reduce by two-thirds CPU times,
	// conn.Write method has 20% reduction of the CPU times,
	// totally provides more than twice of the CPU performance improvement.
	length  uint16
	content []byte
}

func (Self *basePackager) Set(content []byte) (err error) {
	Self.reset()
	if content != nil {
		n := len(content)
		if n == 0 {
			err = errors.New("mux:packer: newpack content is zero length")
		}
		if n > maximumSegmentSize {
			err = errors.New("mux:packer: newpack content segment too large")
			return
		}
		Self.content = Self.content[:n]
		copy(Self.content, content)
	} else {
		err = errors.New("mux:packer: newpack content is nil")
	}
	Self.setLength()
	return
}

func (Self *basePackager) UnPack(reader io.Reader) (n uint16, err error) {
	Self.reset()
	l, err := io.ReadFull(reader, Self.buf[5:7])
	if err != nil {
		return
	}
	n += uint16(l)
	Self.length = binary.LittleEndian.Uint16(Self.buf[5:7])
	if int(Self.length) > cap(Self.content) {
		err = errors.New("mux:packer: unpack err, content length too large")
		return
	}
	if Self.length > maximumSegmentSize {
		err = errors.New("mux:packer: unpack content segment too large")
		return
	}
	Self.content = Self.content[:int(Self.length)]
	l, err = io.ReadFull(reader, Self.content)
	n += uint16(l)
	return
}

func (Self *basePackager) setLength() {
	Self.length = uint16(len(Self.content))
	return
}

func (Self *basePackager) reset() {
	Self.length = 0
	Self.content = Self.content[:0] // reset length
}

type muxPackager struct {
	flag   uint8
	id     int32
	window uint64
	basePackager
}

func (Self *muxPackager) Set(flag uint8, id int32, content interface{}) (err error) {
	Self.buf = windowBuff.Get()
	Self.flag = flag
	Self.id = id
	switch flag {
	case muxPingFlag, muxPingReturn, muxNewMsg, muxNewMsgPart:
		Self.content = windowBuff.Get()
		err = Self.basePackager.Set(content.([]byte))
	case muxMsgSendOk:
		// MUX_MSG_SEND_OK contains one data
		Self.window = content.(uint64)
	}
	return
}

func (Self *muxPackager) Pack(writer io.Writer) (err error) {
	bufs := Self.appendTo(nil)
	_, err = bufs.WriteTo(writer)
	Self.free()
	return
}

// appendTo encodes the header and appends the packager's wire bytes to bufs,
// so that several packagers go out in one writev. The slices alias the
// packager's own buffers: free it only after bufs has been written.
func (Self *muxPackager) appendTo(bufs net.Buffers) net.Buffers {
	Self.buf = Self.buf[0:13]
	Self.buf[0] = byte(Self.flag)
	binary.LittleEndian.PutUint32(Self.buf[1:5], uint32(Self.id))
	switch Self.flag {
	case muxNewMsg, muxNewMsgPart, muxPingFlag, muxPingReturn:
		binary.LittleEndian.PutUint16(Self.buf[5:7], Self.length)
		return append(bufs, Self.buf[:7], Self.content[:Self.length])
	case muxMsgSendOk:
		binary.LittleEndian.PutUint64(Self.buf[5:13], Self.window)
		return append(bufs, Self.buf[:13])
	default:
		return append(bufs, Self.buf[:5])
	}
}

// wireSize is the number of bytes appendTo puts on the wire.
func (Self *muxPackager) wireSize() int {
	switch Self.flag {
	case muxNewMsg, muxNewMsgPart, muxPingFlag, muxPingReturn:
		return 7 + int(Self.length)
	case muxMsgSendOk:
		return 13
	default:
		return 5
	}
}

// free returns the packager's buffers to the pool once it has been written.
func (Self *muxPackager) free() {
	switch Self.flag {
	case muxNewMsg, muxNewMsgPart, muxPingFlag, muxPingReturn:
		windowBuff.Put(Self.content)
	}
	windowBuff.Put(Self.buf)
}

func (Self *muxPackager) UnPack(reader io.Reader) (n uint16, err error) {
	Self.buf = windowBuff.Get()
	Self.buf = Self.buf[0:13]
	l, err := io.ReadFull(reader, Self.buf[:5])
	if err != nil {
		return
	}
	n += uint16(l)
	Self.flag = uint8(Self.buf[0])
	Self.id = int32(binary.LittleEndian.Uint32(Self.buf[1:5]))
	switch Self.flag {
	case muxNewMsg, muxNewMsgPart, muxPingFlag, muxPingReturn:
		var m uint16
		Self.content = windowBuff.Get() // need Get a window buf from pool
		m, err = Self.basePackager.UnPack(reader)
		n += m
	case muxMsgSendOk:
		l, err = io.ReadFull(reader, Self.buf[5:13])
		Self.window = binary.LittleEndian.Uint64(Self.buf[5:13])
		n += uint16(l) // uint64
	}
	windowBuff.Put(Self.buf)
	return
}

func (Self *muxPackager) reset() {
	Self.id = 0
	Self.flag = 0
	Self.length = 0
	Self.content = nil
	Self.window = 0
	Self.buf = nil
}
