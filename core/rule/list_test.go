package rule

import (
	"ehang.io/nps/core/process"
	"testing"
)

func TestGetFields(t *testing.T) {
	h := process.HttpsServeProcess{HttpServeProcess: process.HttpServeProcess{}}
	if len(getFieldName(h)) < 3 {
		t.Fail()
	}
}
