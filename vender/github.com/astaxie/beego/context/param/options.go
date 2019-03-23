package param

import (
	"fmt"
)

// MethodParamOption defines a func which apply options on a MethodParam
type MethodParamOption func(*MethodParam)

// IsRequired indicates that this param is required and can not be omitted from the http request
var IsRequired MethodParamOption = func(p *MethodParam) {
	p.required = true
}

// InHeader indicates that this param is passed via an http header
var InHeader MethodParamOption = func(p *MethodParam) {
	p.in = header
}

// InPath indicates that this param is part of the URL path
var InPath MethodParamOption = func(p *MethodParam) {
	p.in = path
}

// InBody indicates that this param is passed as an http request body
var InBody MethodParamOption = func(p *MethodParam) {
	p.in = body
}

// Default provides a default value for the http param
func Default(defaultValue interface{}) MethodParamOption {
	return func(p *MethodParam) {
		if defaultValue != nil {
			p.defaultValue = fmt.Sprint(defaultValue)
		}
	}
}
