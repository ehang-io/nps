package alils

import (
	"fmt"
	"io"
	"math"

	"github.com/gogo/protobuf/proto"
	github_com_gogo_protobuf_proto "github.com/gogo/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

var (
	// ErrInvalidLengthLog invalid proto
	ErrInvalidLengthLog = fmt.Errorf("proto: negative length found during unmarshaling")
	// ErrIntOverflowLog overflow
	ErrIntOverflowLog = fmt.Errorf("proto: integer overflow")
)

// Log define the proto Log
type Log struct {
	Time            *uint32       `protobuf:"varint,1,req,name=Time" json:"Time,omitempty"`
	Contents        []*LogContent `protobuf:"bytes,2,rep,name=Contents" json:"Contents,omitempty"`
	XXXUnrecognized []byte        `json:"-"`
}

// Reset the Log
func (m *Log) Reset() { *m = Log{} }

// String return the Compact Log
func (m *Log) String() string { return proto.CompactTextString(m) }

// ProtoMessage not implemented
func (*Log) ProtoMessage() {}

// GetTime return the Log's Time
func (m *Log) GetTime() uint32 {
	if m != nil && m.Time != nil {
		return *m.Time
	}
	return 0
}

// GetContents return the Log's Contents
func (m *Log) GetContents() []*LogContent {
	if m != nil {
		return m.Contents
	}
	return nil
}

// LogContent define the Log content struct
type LogContent struct {
	Key             *string `protobuf:"bytes,1,req,name=Key" json:"Key,omitempty"`
	Value           *string `protobuf:"bytes,2,req,name=Value" json:"Value,omitempty"`
	XXXUnrecognized []byte  `json:"-"`
}

// Reset LogContent
func (m *LogContent) Reset() { *m = LogContent{} }

// String return the compact text
func (m *LogContent) String() string { return proto.CompactTextString(m) }

// ProtoMessage not implemented
func (*LogContent) ProtoMessage() {}

// GetKey return the Key
func (m *LogContent) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

// GetValue return the Value
func (m *LogContent) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

// LogGroup define the logs struct
type LogGroup struct {
	Logs            []*Log  `protobuf:"bytes,1,rep,name=Logs" json:"Logs,omitempty"`
	Reserved        *string `protobuf:"bytes,2,opt,name=Reserved" json:"Reserved,omitempty"`
	Topic           *string `protobuf:"bytes,3,opt,name=Topic" json:"Topic,omitempty"`
	Source          *string `protobuf:"bytes,4,opt,name=Source" json:"Source,omitempty"`
	XXXUnrecognized []byte  `json:"-"`
}

// Reset LogGroup
func (m *LogGroup) Reset() { *m = LogGroup{} }

// String return the compact text
func (m *LogGroup) String() string { return proto.CompactTextString(m) }

// ProtoMessage not implemented
func (*LogGroup) ProtoMessage() {}

// GetLogs return the loggroup logs
func (m *LogGroup) GetLogs() []*Log {
	if m != nil {
		return m.Logs
	}
	return nil
}

// GetReserved return Reserved
func (m *LogGroup) GetReserved() string {
	if m != nil && m.Reserved != nil {
		return *m.Reserved
	}
	return ""
}

// GetTopic return Topic
func (m *LogGroup) GetTopic() string {
	if m != nil && m.Topic != nil {
		return *m.Topic
	}
	return ""
}

// GetSource return Source
func (m *LogGroup) GetSource() string {
	if m != nil && m.Source != nil {
		return *m.Source
	}
	return ""
}

// LogGroupList define the LogGroups
type LogGroupList struct {
	LogGroups       []*LogGroup `protobuf:"bytes,1,rep,name=logGroups" json:"logGroups,omitempty"`
	XXXUnrecognized []byte      `json:"-"`
}

// Reset LogGroupList
func (m *LogGroupList) Reset() { *m = LogGroupList{} }

// String return compact text
func (m *LogGroupList) String() string { return proto.CompactTextString(m) }

// ProtoMessage not implemented
func (*LogGroupList) ProtoMessage() {}

// GetLogGroups return the LogGroups
func (m *LogGroupList) GetLogGroups() []*LogGroup {
	if m != nil {
		return m.LogGroups
	}
	return nil
}

// Marshal the logs to byte slice
func (m *Log) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

// MarshalTo data
func (m *Log) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Time == nil {
		return 0, github_com_gogo_protobuf_proto.NewRequiredNotSetError("Time")
	}
	data[i] = 0x8
	i++
	i = encodeVarintLog(data, i, uint64(*m.Time))
	if len(m.Contents) > 0 {
		for _, msg := range m.Contents {
			data[i] = 0x12
			i++
			i = encodeVarintLog(data, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(data[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	if m.XXXUnrecognized != nil {
		i += copy(data[i:], m.XXXUnrecognized)
	}
	return i, nil
}

// Marshal LogContent
func (m *LogContent) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

// MarshalTo logcontent to data
func (m *LogContent) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Key == nil {
		return 0, github_com_gogo_protobuf_proto.NewRequiredNotSetError("Key")
	}
	data[i] = 0xa
	i++
	i = encodeVarintLog(data, i, uint64(len(*m.Key)))
	i += copy(data[i:], *m.Key)

	if m.Value == nil {
		return 0, github_com_gogo_protobuf_proto.NewRequiredNotSetError("Value")
	}
	data[i] = 0x12
	i++
	i = encodeVarintLog(data, i, uint64(len(*m.Value)))
	i += copy(data[i:], *m.Value)
	if m.XXXUnrecognized != nil {
		i += copy(data[i:], m.XXXUnrecognized)
	}
	return i, nil
}

// Marshal LogGroup
func (m *LogGroup) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

// MarshalTo LogGroup to data
func (m *LogGroup) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Logs) > 0 {
		for _, msg := range m.Logs {
			data[i] = 0xa
			i++
			i = encodeVarintLog(data, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(data[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	if m.Reserved != nil {
		data[i] = 0x12
		i++
		i = encodeVarintLog(data, i, uint64(len(*m.Reserved)))
		i += copy(data[i:], *m.Reserved)
	}
	if m.Topic != nil {
		data[i] = 0x1a
		i++
		i = encodeVarintLog(data, i, uint64(len(*m.Topic)))
		i += copy(data[i:], *m.Topic)
	}
	if m.Source != nil {
		data[i] = 0x22
		i++
		i = encodeVarintLog(data, i, uint64(len(*m.Source)))
		i += copy(data[i:], *m.Source)
	}
	if m.XXXUnrecognized != nil {
		i += copy(data[i:], m.XXXUnrecognized)
	}
	return i, nil
}

// Marshal LogGroupList
func (m *LogGroupList) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

// MarshalTo LogGroupList to data
func (m *LogGroupList) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.LogGroups) > 0 {
		for _, msg := range m.LogGroups {
			data[i] = 0xa
			i++
			i = encodeVarintLog(data, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(data[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	if m.XXXUnrecognized != nil {
		i += copy(data[i:], m.XXXUnrecognized)
	}
	return i, nil
}

func encodeFixed64Log(data []byte, offset int, v uint64) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	data[offset+4] = uint8(v >> 32)
	data[offset+5] = uint8(v >> 40)
	data[offset+6] = uint8(v >> 48)
	data[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32Log(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintLog(data []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		data[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	data[offset] = uint8(v)
	return offset + 1
}

// Size return the log's size
func (m *Log) Size() (n int) {
	var l int
	_ = l
	if m.Time != nil {
		n += 1 + sovLog(uint64(*m.Time))
	}
	if len(m.Contents) > 0 {
		for _, e := range m.Contents {
			l = e.Size()
			n += 1 + l + sovLog(uint64(l))
		}
	}
	if m.XXXUnrecognized != nil {
		n += len(m.XXXUnrecognized)
	}
	return n
}

// Size return LogContent size based on Key and Value
func (m *LogContent) Size() (n int) {
	var l int
	_ = l
	if m.Key != nil {
		l = len(*m.Key)
		n += 1 + l + sovLog(uint64(l))
	}
	if m.Value != nil {
		l = len(*m.Value)
		n += 1 + l + sovLog(uint64(l))
	}
	if m.XXXUnrecognized != nil {
		n += len(m.XXXUnrecognized)
	}
	return n
}

// Size return LogGroup size based on Logs
func (m *LogGroup) Size() (n int) {
	var l int
	_ = l
	if len(m.Logs) > 0 {
		for _, e := range m.Logs {
			l = e.Size()
			n += 1 + l + sovLog(uint64(l))
		}
	}
	if m.Reserved != nil {
		l = len(*m.Reserved)
		n += 1 + l + sovLog(uint64(l))
	}
	if m.Topic != nil {
		l = len(*m.Topic)
		n += 1 + l + sovLog(uint64(l))
	}
	if m.Source != nil {
		l = len(*m.Source)
		n += 1 + l + sovLog(uint64(l))
	}
	if m.XXXUnrecognized != nil {
		n += len(m.XXXUnrecognized)
	}
	return n
}

// Size return LogGroupList size
func (m *LogGroupList) Size() (n int) {
	var l int
	_ = l
	if len(m.LogGroups) > 0 {
		for _, e := range m.LogGroups {
			l = e.Size()
			n += 1 + l + sovLog(uint64(l))
		}
	}
	if m.XXXUnrecognized != nil {
		n += len(m.XXXUnrecognized)
	}
	return n
}

func sovLog(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozLog(x uint64) (n int) {
	return sovLog((x << 1) ^ (x >> 63))
}

// Unmarshal data to log
func (m *Log) Unmarshal(data []byte) error {
	var hasFields [1]uint64
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowLog
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Log: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Log: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Time", wireType)
			}
			var v uint32
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				v |= (uint32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Time = &v
			hasFields[0] |= uint64(0x00000001)
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Contents", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Contents = append(m.Contents, &LogContent{})
			if err := m.Contents[len(m.Contents)-1].Unmarshal(data[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipLog(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthLog
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXXUnrecognized = append(m.XXXUnrecognized, data[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}
	if hasFields[0]&uint64(0x00000001) == 0 {
		return github_com_gogo_protobuf_proto.NewRequiredNotSetError("Time")
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// Unmarshal data to LogContent
func (m *LogContent) Unmarshal(data []byte) error {
	var hasFields [1]uint64
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowLog
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Content: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Content: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Key", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			s := string(data[iNdEx:postIndex])
			m.Key = &s
			iNdEx = postIndex
			hasFields[0] |= uint64(0x00000001)
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			s := string(data[iNdEx:postIndex])
			m.Value = &s
			iNdEx = postIndex
			hasFields[0] |= uint64(0x00000002)
		default:
			iNdEx = preIndex
			skippy, err := skipLog(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthLog
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXXUnrecognized = append(m.XXXUnrecognized, data[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}
	if hasFields[0]&uint64(0x00000001) == 0 {
		return github_com_gogo_protobuf_proto.NewRequiredNotSetError("Key")
	}
	if hasFields[0]&uint64(0x00000002) == 0 {
		return github_com_gogo_protobuf_proto.NewRequiredNotSetError("Value")
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// Unmarshal data to LogGroup
func (m *LogGroup) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowLog
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: LogGroup: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: LogGroup: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Logs", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Logs = append(m.Logs, &Log{})
			if err := m.Logs[len(m.Logs)-1].Unmarshal(data[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Reserved", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			s := string(data[iNdEx:postIndex])
			m.Reserved = &s
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Topic", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			s := string(data[iNdEx:postIndex])
			m.Topic = &s
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Source", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			s := string(data[iNdEx:postIndex])
			m.Source = &s
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipLog(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthLog
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXXUnrecognized = append(m.XXXUnrecognized, data[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// Unmarshal data to LogGroupList
func (m *LogGroupList) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowLog
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: LogGroupList: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: LogGroupList: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field LogGroups", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowLog
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthLog
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.LogGroups = append(m.LogGroups, &LogGroup{})
			if err := m.LogGroups[len(m.LogGroups)-1].Unmarshal(data[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipLog(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthLog
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXXUnrecognized = append(m.XXXUnrecognized, data[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func skipLog(data []byte) (n int, err error) {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowLog
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowLog
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if data[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowLog
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthLog
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowLog
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := data[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipLog(data[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}
