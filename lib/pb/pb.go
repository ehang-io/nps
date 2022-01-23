package pb

import (
	"ehang.io/nps/lib/common"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"io"
)

// WriteMessage is used to write a message to writer
func WriteMessage(w io.Writer, message proto.Message) (int, error) {
	b, err := proto.Marshal(message)
	if err != nil {
		return 0, errors.Wrap(err, "proto Marshal")
	}
	n, err := common.WriteLenBytes(w, b)
	return n, err
}

// ReadMessage is used to read a message from reader
func ReadMessage(r io.Reader, message proto.Message) (int, error) {
	message.Reset()
	b := make([]byte, 4096)
	n, err := common.ReadLenBytes(r, b)
	if err != nil {
		return 0, errors.Wrap(err, "read proto message")
	}
	return n, proto.Unmarshal(b[:n], message)
}
