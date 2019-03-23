package param

import (
	"fmt"
	"reflect"

	beecontext "github.com/cnlh/nps/vender/github.com/astaxie/beego/context"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
)

// ConvertParams converts http method params to values that will be passed to the method controller as arguments
func ConvertParams(methodParams []*MethodParam, methodType reflect.Type, ctx *beecontext.Context) (result []reflect.Value) {
	result = make([]reflect.Value, 0, len(methodParams))
	for i := 0; i < len(methodParams); i++ {
		reflectValue := convertParam(methodParams[i], methodType.In(i), ctx)
		result = append(result, reflectValue)
	}
	return
}

func convertParam(param *MethodParam, paramType reflect.Type, ctx *beecontext.Context) (result reflect.Value) {
	paramValue := getParamValue(param, ctx)
	if paramValue == "" {
		if param.required {
			ctx.Abort(400, fmt.Sprintf("Missing parameter %s", param.name))
		} else {
			paramValue = param.defaultValue
		}
	}

	reflectValue, err := parseValue(param, paramValue, paramType)
	if err != nil {
		logs.Debug(fmt.Sprintf("Error converting param %s to type %s. Value: %v, Error: %s", param.name, paramType, paramValue, err))
		ctx.Abort(400, fmt.Sprintf("Invalid parameter %s. Can not convert %v to type %s", param.name, paramValue, paramType))
	}

	return reflectValue
}

func getParamValue(param *MethodParam, ctx *beecontext.Context) string {
	switch param.in {
	case body:
		return string(ctx.Input.RequestBody)
	case header:
		return ctx.Input.Header(param.name)
	case path:
		return ctx.Input.Query(":" + param.name)
	default:
		return ctx.Input.Query(param.name)
	}
}

func parseValue(param *MethodParam, paramValue string, paramType reflect.Type) (result reflect.Value, err error) {
	if paramValue == "" {
		return reflect.Zero(paramType), nil
	}
	parser := getParser(param, paramType)
	value, err := parser.parse(paramValue, paramType)
	if err != nil {
		return result, err
	}

	return safeConvert(reflect.ValueOf(value), paramType)
}

func safeConvert(value reflect.Value, t reflect.Type) (result reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()
	result = value.Convert(t)
	return
}
