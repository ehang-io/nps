package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
)

type NetPackager interface {
	Pack(writer io.Writer) (err error)
	UnPack(reader io.Reader) (err error)
}

type BasePackager struct {
	Length  uint16
	Content []byte
}

func (Self *BasePackager) NewPac(contents ...interface{}) (err error) {
	Self.clean()
	for _, content := range contents {
		switch content.(type) {
		case nil:
			Self.Content = Self.Content[:0]
		case []byte:
			err = Self.appendByte(content.([]byte))
		case string:
			err = Self.appendByte([]byte(content.(string)))
			if err != nil {
				return
			}
			err = Self.appendByte([]byte(CONN_DATA_SEQ))
		default:
			err = Self.marshal(content)
		}
	}
	Self.setLength()
	return
}

func (Self *BasePackager) appendByte(data []byte) (err error) {
	m := len(Self.Content)
	n := m + len(data)
	if n <= cap(Self.Content) {
		Self.Content = Self.Content[0:n] // grow the length for copy
		copy(Self.Content[m:n], data)
		return nil
	} else {
		return errors.New("pack content too large")
	}
}

//似乎这里涉及到父类作用域问题，当子类调用父类的方法时，其struct仅仅为父类的
func (Self *BasePackager) Pack(writer io.Writer) (err error) {
	err = binary.Write(writer, binary.LittleEndian, Self.Length)
	if err != nil {
		return
	}
	err = binary.Write(writer, binary.LittleEndian, Self.Content)
	return
}

//Unpack 会导致传入的数字类型转化成float64！！
//主要原因是json unmarshal并未传入正确的数据类型
func (Self *BasePackager) UnPack(reader io.Reader) (n uint16, err error) {
	Self.clean()
	n += 2 // uint16
	err = binary.Read(reader, binary.LittleEndian, &Self.Length)
	if err != nil {
		return
	}
	if int(Self.Length) > cap(Self.Content) {
		err = errors.New("unpack err, content length too large")
	}
	Self.Content = Self.Content[:int(Self.Length)]
	//n, err := io.ReadFull(reader, Self.Content)
	//if n != int(Self.Length) {
	//	err = io.ErrUnexpectedEOF
	//}
	err = binary.Read(reader, binary.LittleEndian, Self.Content)
	n += Self.Length
	return
}

func (Self *BasePackager) marshal(content interface{}) (err error) {
	tmp, err := json.Marshal(content)
	if err != nil {
		return err
	}
	err = Self.appendByte(tmp)
	return
}

func (Self *BasePackager) Unmarshal(content interface{}) (err error) {
	err = json.Unmarshal(Self.Content, content)
	if err != nil {
		return err
	}
	return
}

func (Self *BasePackager) setLength() {
	Self.Length = uint16(len(Self.Content))
	return
}

func (Self *BasePackager) clean() {
	Self.Length = 0
	Self.Content = Self.Content[:0] // reset length
}

func (Self *BasePackager) Split() (strList []string) {
	n := bytes.IndexByte(Self.Content, 0)
	strList = strings.Split(string(Self.Content[:n]), CONN_DATA_SEQ)
	strList = strList[0 : len(strList)-1]
	return
}

type ConnPackager struct {
	// Todo
	ConnType uint8
	BasePackager
}

func (Self *ConnPackager) NewPac(connType uint8, content ...interface{}) (err error) {
	Self.ConnType = connType
	err = Self.BasePackager.NewPac(content...)
	return
}

func (Self *ConnPackager) Pack(writer io.Writer) (err error) {
	err = binary.Write(writer, binary.LittleEndian, Self.ConnType)
	if err != nil {
		return
	}
	err = Self.BasePackager.Pack(writer)
	return
}

func (Self *ConnPackager) UnPack(reader io.Reader) (n uint16, err error) {
	err = binary.Read(reader, binary.LittleEndian, &Self.ConnType)
	if err != nil && err != io.EOF {
		return
	}
	n, err = Self.BasePackager.UnPack(reader)
	n += 2
	return
}

type MuxPackager struct {
	Flag       uint8
	Id         int32
	Window     uint32
	ReadLength uint32
	BasePackager
}

func (Self *MuxPackager) NewPac(flag uint8, id int32, content ...interface{}) (err error) {
	Self.Flag = flag
	Self.Id = id
	switch flag {
	case MUX_PING_FLAG, MUX_PING_RETURN, MUX_NEW_MSG, MUX_NEW_MSG_PART:
		Self.Content = WindowBuff.Get()
		err = Self.BasePackager.NewPac(content...)
		//logs.Warn(Self.Length, string(Self.Content))
	case MUX_MSG_SEND_OK:
		// MUX_MSG_SEND_OK contains two data
		switch content[0].(type) {
		case int:
			Self.Window = uint32(content[0].(int))
		case uint32:
			Self.Window = content[0].(uint32)
		}
		switch content[1].(type) {
		case int:
			Self.ReadLength = uint32(content[1].(int))
		case uint32:
			Self.ReadLength = content[1].(uint32)
		}
	}
	return
}

func (Self *MuxPackager) Pack(writer io.Writer) (err error) {
	err = binary.Write(writer, binary.LittleEndian, Self.Flag)
	if err != nil {
		return
	}
	err = binary.Write(writer, binary.LittleEndian, Self.Id)
	if err != nil {
		return
	}
	switch Self.Flag {
	case MUX_NEW_MSG, MUX_NEW_MSG_PART, MUX_PING_FLAG, MUX_PING_RETURN:
		err = Self.BasePackager.Pack(writer)
		WindowBuff.Put(Self.Content)
	case MUX_MSG_SEND_OK:
		err = binary.Write(writer, binary.LittleEndian, Self.Window)
		if err != nil {
			return
		}
		err = binary.Write(writer, binary.LittleEndian, Self.ReadLength)
	}
	return
}

func (Self *MuxPackager) UnPack(reader io.Reader) (n uint16, err error) {
	err = binary.Read(reader, binary.LittleEndian, &Self.Flag)
	if err != nil {
		return
	}
	err = binary.Read(reader, binary.LittleEndian, &Self.Id)
	if err != nil {
		return
	}
	switch Self.Flag {
	case MUX_NEW_MSG, MUX_NEW_MSG_PART, MUX_PING_FLAG, MUX_PING_RETURN:
		Self.Content = WindowBuff.Get() // need get a window buf from pool
		Self.BasePackager.clean()       // also clean the content
		n, err = Self.BasePackager.UnPack(reader)
		//logs.Warn("unpack", Self.Length, string(Self.Content))
	case MUX_MSG_SEND_OK:
		err = binary.Read(reader, binary.LittleEndian, &Self.Window)
		if err != nil {
			return
		}
		n += 4 // uint32
		err = binary.Read(reader, binary.LittleEndian, &Self.ReadLength)
		n += 4 // uint32
	}
	n += 5 //uint8 int32
	return
}

const (
	ipV4       = 1
	domainName = 3
	ipV6       = 4
)

type UDPHeader struct {
	Rsv  uint16
	Frag uint8
	Addr *Addr
}

func NewUDPHeader(rsv uint16, frag uint8, addr *Addr) *UDPHeader {
	return &UDPHeader{
		Rsv:  rsv,
		Frag: frag,
		Addr: addr,
	}
}

type Addr struct {
	Type uint8
	Host string
	Port uint16
}

func (addr *Addr) String() string {
	return net.JoinHostPort(addr.Host, strconv.Itoa(int(addr.Port)))
}

func (addr *Addr) Decode(b []byte) error {
	addr.Type = b[0]
	pos := 1
	switch addr.Type {
	case ipV4:
		addr.Host = net.IP(b[pos:pos+net.IPv4len]).String()
		pos += net.IPv4len
	case ipV6:
		addr.Host = net.IP(b[pos:pos+net.IPv6len]).String()
		pos += net.IPv6len
	case domainName:
		addrlen := int(b[pos])
		pos++
		addr.Host = string(b[pos : pos+addrlen])
		pos += addrlen
	default:
		return errors.New("decode error")
	}

	addr.Port = binary.BigEndian.Uint16(b[pos:])

	return nil
}

func (addr *Addr) Encode(b []byte) (int, error) {
	b[0] = addr.Type
	pos := 1
	switch addr.Type {
	case ipV4:
		ip4 := net.ParseIP(addr.Host).To4()
		if ip4 == nil {
			ip4 = net.IPv4zero.To4()
		}
		pos += copy(b[pos:], ip4)
	case domainName:
		b[pos] = byte(len(addr.Host))
		pos++
		pos += copy(b[pos:], []byte(addr.Host))
	case ipV6:
		ip16 := net.ParseIP(addr.Host).To16()
		if ip16 == nil {
			ip16 = net.IPv6zero.To16()
		}
		pos += copy(b[pos:], ip16)
	default:
		b[0] = ipV4
		copy(b[pos:pos+4], net.IPv4zero.To4())
		pos += 4
	}
	binary.BigEndian.PutUint16(b[pos:], addr.Port)
	pos += 2

	return pos, nil
}

func (h *UDPHeader) Write(w io.Writer) error {
	b := BufPoolUdp.Get().([]byte)
	defer BufPoolUdp.Put(b)

	binary.BigEndian.PutUint16(b[:2], h.Rsv)
	b[2] = h.Frag

	addr := h.Addr
	if addr == nil {
		addr = &Addr{}
	}
	length, _ := addr.Encode(b[3:])

	_, err := w.Write(b[:3+length])
	return err
}

type UDPDatagram struct {
	Header *UDPHeader
	Data   []byte
}

func ReadUDPDatagram(r io.Reader) (*UDPDatagram, error) {
	b := BufPoolUdp.Get().([]byte)
	defer BufPoolUdp.Put(b)

	// when r is a streaming (such as TCP connection), we may read more than the required data,
	// but we don't know how to handle it. So we use io.ReadFull to instead of io.ReadAtLeast
	// to make sure that no redundant data will be discarded.
	n, err := io.ReadFull(r, b[:5])
	if err != nil {
		return nil, err
	}

	header := &UDPHeader{
		Rsv:  binary.BigEndian.Uint16(b[:2]),
		Frag: b[2],
	}

	atype := b[3]
	hlen := 0
	switch atype {
	case ipV4:
		hlen = 10
	case ipV6:
		hlen = 22
	case domainName:
		hlen = 7 + int(b[4])
	default:
		return nil, errors.New("addr not support")
	}
	dlen := int(header.Rsv)
	if dlen == 0 { // standard SOCKS5 UDP datagram
		extra, err := ioutil.ReadAll(r) // we assume no redundant data
		if err != nil {
			return nil, err
		}
		copy(b[n:], extra)
		n += len(extra) // total length
		dlen = n - hlen // data length
	} else { // extended feature, for UDP over TCP, using reserved field as data length
		if _, err := io.ReadFull(r, b[n:hlen+dlen]); err != nil {
			return nil, err
		}
		n = hlen + dlen
	}
	header.Addr = new(Addr)
	if err := header.Addr.Decode(b[3:hlen]); err != nil {
		return nil, err
	}
	data := make([]byte, dlen)
	copy(data, b[hlen:n])
	d := &UDPDatagram{
		Header: header,
		Data:   data,
	}
	return d, nil
}

func NewUDPDatagram(header *UDPHeader, data []byte) *UDPDatagram {
	return &UDPDatagram{
		Header: header,
		Data:   data,
	}
}

func (d *UDPDatagram) Write(w io.Writer) error {
	h := d.Header
	if h == nil {
		h = &UDPHeader{}
	}
	buf := bytes.Buffer{}
	if err := h.Write(&buf); err != nil {
		return err
	}
	if _, err := buf.Write(d.Data); err != nil {
		return err
	}

	_, err := buf.WriteTo(w)
	return err
}

func ToSocksAddr(addr net.Addr) *Addr {
	host := "0.0.0.0"
	port := 0
	if addr != nil {
		h, p, _ := net.SplitHostPort(addr.String())
		host = h
		port, _ = strconv.Atoi(p)
	}
	return &Addr{
		Type: ipV4,
		Host: host,
		Port: uint16(port),
	}
}
