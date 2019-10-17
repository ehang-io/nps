package core

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
	Length  uint32
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
func (Self *BasePackager) UnPack(reader io.Reader) (err error) {
	Self.clean()
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
	Self.Length = uint32(len(Self.Content))
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

func (Self *ConnPackager) UnPack(reader io.Reader) (err error) {
	err = binary.Read(reader, binary.LittleEndian, &Self.ConnType)
	if err != nil && err != io.EOF {
		return
	}
	err = Self.BasePackager.UnPack(reader)
	return
}

type MuxPackager struct {
	Flag   uint8
	Id     int32
	Window uint16
	BasePackager
}

func (Self *MuxPackager) NewPac(flag uint8, id int32, content ...interface{}) (err error) {
	Self.Flag = flag
	Self.Id = id
	if flag == MUX_NEW_MSG {
		err = Self.BasePackager.NewPac(content...)
	}
	if flag == MUX_MSG_SEND_OK {
		// MUX_MSG_SEND_OK only allows one data
		switch content[0].(type) {
		case int:
			Self.Window = uint16(content[0].(int))
		case uint16:
			Self.Window = content[0].(uint16)
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
	if Self.Flag == MUX_NEW_MSG {
		err = Self.BasePackager.Pack(writer)
	}
	if Self.Flag == MUX_MSG_SEND_OK {
		err = binary.Write(writer, binary.LittleEndian, Self.Window)
	}
	return
}

func (Self *MuxPackager) UnPack(reader io.Reader) (err error) {
	Self.BasePackager.clean() // also clean the content
	err = binary.Read(reader, binary.LittleEndian, &Self.Flag)
	if err != nil {
		return
	}
	err = binary.Read(reader, binary.LittleEndian, &Self.Id)
	if err != nil {
		return
	}
	if Self.Flag == MUX_NEW_MSG {
		err = Self.BasePackager.UnPack(reader)
	}
	if Self.Flag == MUX_MSG_SEND_OK {
		err = binary.Read(reader, binary.LittleEndian, &Self.Window)
	}
	return
}
