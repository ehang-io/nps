package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
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

type ConnPackager struct { // Todo
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
