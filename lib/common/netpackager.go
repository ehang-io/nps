package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/cnlh/nps/lib/pool"
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
			Self.Content = append(Self.Content, content.([]byte)...)
		case string:
			Self.Content = append(Self.Content, []byte(content.(string))...)
			Self.Content = append(Self.Content, []byte(CONN_DATA_SEQ)...)
		default:
			err = Self.marshal(content)
		}
	}
	Self.setLength()
	return
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
	Self.Content = pool.GetBufPoolCopy()
	Self.Content = Self.Content[:Self.Length]
	//n, err := io.ReadFull(reader, Self.Content)
	//if n != int(Self.Length) {
	//	err = io.ErrUnexpectedEOF
	//}
	err = binary.Read(reader, binary.LittleEndian, &Self.Content)
	return
}

func (Self *BasePackager) marshal(content interface{}) (err error) {
	tmp, err := json.Marshal(content)
	if err != nil {
		return err
	}
	Self.Content = append(Self.Content, tmp...)
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
	Self.Content = Self.Content[:0]
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
	Flag uint8
	Id   int32
	BasePackager
}

func (Self *MuxPackager) NewPac(flag uint8, id int32, content ...interface{}) (err error) {
	Self.Flag = flag
	Self.Id = id
	if flag == MUX_NEW_MSG {
		err = Self.BasePackager.NewPac(content...)
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
	return
}

func (Self *MuxPackager) UnPack(reader io.Reader) (err error) {
	Self.Length=0
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
	return
}
