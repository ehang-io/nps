package handler

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httputil"
	"testing"
)

func TestHandleHttpConn(t *testing.T) {

	h := HttpHandler{}
	rule := &testRule{}
	h.AddRule(rule)

	r, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)

	b, err := httputil.DumpRequest(r, false)
	assert.NoError(t, err)

	res, err := h.HandleConn(b, nil)

	assert.NoError(t, err)
	assert.Equal(t, true, res)
	assert.Equal(t, true, rule.run)
}
