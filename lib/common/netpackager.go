package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"strconv"
)

type NetPackager interface {
	Pack(writer io.Writer) (err error)
	UnPack(reader io.Reader) (err error)
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
		addr.Host = net.IP(b[pos : pos+net.IPv4len]).String()
		pos += net.IPv4len
	case ipV6:
		addr.Host = net.IP(b[pos : pos+net.IPv6len]).String()
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
