package context

import (
	"strconv"

	"net/http"
)

const (
	//BadRequest indicates http error 400
	BadRequest StatusCode = http.StatusBadRequest

	//NotFound indicates http error 404
	NotFound StatusCode = http.StatusNotFound
)

// StatusCode sets the http response status code
type StatusCode int

func (s StatusCode) Error() string {
	return strconv.Itoa(int(s))
}

// Render sets the http status code
func (s StatusCode) Render(ctx *Context) {
	ctx.Output.SetStatus(int(s))
}
