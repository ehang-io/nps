package pb

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMarshal(t *testing.T) {
	app := &AppInfo{
		ConnType: ConnType_udp,
		AppAddr:   "127.0.0.1:8080",
	}
	var buf []byte
	b := bytes.NewBuffer(buf)

	_, err := WriteMessage(b, app)
	assert.NoError(t, err)

	appRecv := &AppInfo{}
	_, err = ReadMessage(b, appRecv)
	assert.NoError(t, err)
	assert.Equal(t, app.AppAddr, appRecv.AppAddr)
	assert.Equal(t, app.ConnType, appRecv.ConnType)
}
