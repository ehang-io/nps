// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package beego

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	beecontext "github.com/cnlh/nps/vender/github.com/astaxie/beego/context"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/context/param"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/toolbox"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/utils"
)

// default filter execution points
const (
	BeforeStatic = iota
	BeforeRouter
	BeforeExec
	AfterExec
	FinishRouter
)

const (
	routerTypeBeego = iota
	routerTypeRESTFul
	routerTypeHandler
)

var (
	// HTTPMETHOD list the supported http methods.
	HTTPMETHOD = map[string]bool{
		"GET":       true,
		"POST":      true,
		"PUT":       true,
		"DELETE":    true,
		"PATCH":     true,
		"OPTIONS":   true,
		"HEAD":      true,
		"TRACE":     true,
		"CONNECT":   true,
		"MKCOL":     true,
		"COPY":      true,
		"MOVE":      true,
		"PROPFIND":  true,
		"PROPPATCH": true,
		"LOCK":      true,
		"UNLOCK":    true,
	}
	// these beego.Controller's methods shouldn't reflect to AutoRouter
	exceptMethod = []string{"Init", "Prepare", "Finish", "Render", "RenderString",
		"RenderBytes", "Redirect", "Abort", "StopRun", "UrlFor", "ServeJSON", "ServeJSONP",
		"ServeYAML", "ServeXML", "Input", "ParseForm", "GetString", "GetStrings", "GetInt", "GetBool",
		"GetFloat", "GetFile", "SaveToFile", "StartSession", "SetSession", "GetSession",
		"DelSession", "SessionRegenerateID", "DestroySession", "IsAjax", "GetSecureCookie",
		"SetSecureCookie", "XsrfToken", "CheckXsrfCookie", "XsrfFormHtml",
		"GetControllerAndAction", "ServeFormatted"}

	urlPlaceholder = "{{placeholder}}"
	// DefaultAccessLogFilter will skip the accesslog if return true
	DefaultAccessLogFilter FilterHandler = &logFilter{}
)

// FilterHandler is an interface for
type FilterHandler interface {
	Filter(*beecontext.Context) bool
}

// default log filter static file will not show
type logFilter struct {
}

func (l *logFilter) Filter(ctx *beecontext.Context) bool {
	requestPath := path.Clean(ctx.Request.URL.Path)
	if requestPath == "/favicon.ico" || requestPath == "/robots.txt" {
		return true
	}
	for prefix := range BConfig.WebConfig.StaticDir {
		if strings.HasPrefix(requestPath, prefix) {
			return true
		}
	}
	return false
}

// ExceptMethodAppend to append a slice's value into "exceptMethod", for controller's methods shouldn't reflect to AutoRouter
func ExceptMethodAppend(action string) {
	exceptMethod = append(exceptMethod, action)
}

// ControllerInfo holds information about the controller.
type ControllerInfo struct {
	pattern        string
	controllerType reflect.Type
	methods        map[string]string
	handler        http.Handler
	runFunction    FilterFunc
	routerType     int
	initialize     func() ControllerInterface
	methodParams   []*param.MethodParam
}

// ControllerRegister containers registered router rules, controller handlers and filters.
type ControllerRegister struct {
	routers      map[string]*Tree
	enablePolicy bool
	policies     map[string]*Tree
	enableFilter bool
	filters      [FinishRouter + 1][]*FilterRouter
	pool         sync.Pool
}

// NewControllerRegister returns a new ControllerRegister.
func NewControllerRegister() *ControllerRegister {
	cr := &ControllerRegister{
		routers:  make(map[string]*Tree),
		policies: make(map[string]*Tree),
	}
	cr.pool.New = func() interface{} {
		return beecontext.NewContext()
	}
	return cr
}

// Add controller handler and pattern rules to ControllerRegister.
// usage:
//	default methods is the same name as method
//	Add("/user",&UserController{})
//	Add("/api/list",&RestController{},"*:ListFood")
//	Add("/api/create",&RestController{},"post:CreateFood")
//	Add("/api/update",&RestController{},"put:UpdateFood")
//	Add("/api/delete",&RestController{},"delete:DeleteFood")
//	Add("/api",&RestController{},"get,post:ApiFunc"
//	Add("/simple",&SimpleController{},"get:GetFunc;post:PostFunc")
func (p *ControllerRegister) Add(pattern string, c ControllerInterface, mappingMethods ...string) {
	p.addWithMethodParams(pattern, c, nil, mappingMethods...)
}

func (p *ControllerRegister) addWithMethodParams(pattern string, c ControllerInterface, methodParams []*param.MethodParam, mappingMethods ...string) {
	reflectVal := reflect.ValueOf(c)
	t := reflect.Indirect(reflectVal).Type()
	methods := make(map[string]string)
	if len(mappingMethods) > 0 {
		semi := strings.Split(mappingMethods[0], ";")
		for _, v := range semi {
			colon := strings.Split(v, ":")
			if len(colon) != 2 {
				panic("method mapping format is invalid")
			}
			comma := strings.Split(colon[0], ",")
			for _, m := range comma {
				if m == "*" || HTTPMETHOD[strings.ToUpper(m)] {
					if val := reflectVal.MethodByName(colon[1]); val.IsValid() {
						methods[strings.ToUpper(m)] = colon[1]
					} else {
						panic("'" + colon[1] + "' method doesn't exist in the controller " + t.Name())
					}
				} else {
					panic(v + " is an invalid method mapping. Method doesn't exist " + m)
				}
			}
		}
	}

	route := &ControllerInfo{}
	route.pattern = pattern
	route.methods = methods
	route.routerType = routerTypeBeego
	route.controllerType = t
	route.initialize = func() ControllerInterface {
		vc := reflect.New(route.controllerType)
		execController, ok := vc.Interface().(ControllerInterface)
		if !ok {
			panic("controller is not ControllerInterface")
		}

		elemVal := reflect.ValueOf(c).Elem()
		elemType := reflect.TypeOf(c).Elem()
		execElem := reflect.ValueOf(execController).Elem()

		numOfFields := elemVal.NumField()
		for i := 0; i < numOfFields; i++ {
			fieldType := elemType.Field(i)
			elemField := execElem.FieldByName(fieldType.Name)
			if elemField.CanSet() {
				fieldVal := elemVal.Field(i)
				elemField.Set(fieldVal)
			}
		}

		return execController
	}

	route.methodParams = methodParams
	if len(methods) == 0 {
		for m := range HTTPMETHOD {
			p.addToRouter(m, pattern, route)
		}
	} else {
		for k := range methods {
			if k == "*" {
				for m := range HTTPMETHOD {
					p.addToRouter(m, pattern, route)
				}
			} else {
				p.addToRouter(k, pattern, route)
			}
		}
	}
}

func (p *ControllerRegister) addToRouter(method, pattern string, r *ControllerInfo) {
	if !BConfig.RouterCaseSensitive {
		pattern = strings.ToLower(pattern)
	}
	if t, ok := p.routers[method]; ok {
		t.AddRouter(pattern, r)
	} else {
		t := NewTree()
		t.AddRouter(pattern, r)
		p.routers[method] = t
	}
}

// Include only when the Runmode is dev will generate router file in the router/auto.go from the controller
// Include(&BankAccount{}, &OrderController{},&RefundController{},&ReceiptController{})
func (p *ControllerRegister) Include(cList ...ControllerInterface) {
	if BConfig.RunMode == DEV {
		skip := make(map[string]bool, 10)
		for _, c := range cList {
			reflectVal := reflect.ValueOf(c)
			t := reflect.Indirect(reflectVal).Type()
			wgopath := utils.GetGOPATHs()
			if len(wgopath) == 0 {
				panic("you are in dev mode. So please set gopath")
			}
			pkgpath := ""
			for _, wg := range wgopath {
				wg, _ = filepath.EvalSymlinks(filepath.Join(wg, "src", t.PkgPath()))
				if utils.FileExists(wg) {
					pkgpath = wg
					break
				}
			}
			if pkgpath != "" {
				if _, ok := skip[pkgpath]; !ok {
					skip[pkgpath] = true
					parserPkg(pkgpath, t.PkgPath())
				}
			}
		}
	}
	for _, c := range cList {
		reflectVal := reflect.ValueOf(c)
		t := reflect.Indirect(reflectVal).Type()
		key := t.PkgPath() + ":" + t.Name()
		if comm, ok := GlobalControllerRouter[key]; ok {
			for _, a := range comm {
				for _, f := range a.Filters {
					p.InsertFilter(f.Pattern, f.Pos, f.Filter, f.ReturnOnOutput, f.ResetParams)
				}

				p.addWithMethodParams(a.Router, c, a.MethodParams, strings.Join(a.AllowHTTPMethods, ",")+":"+a.Method)
			}
		}
	}
}

// Get add get method
// usage:
//    Get("/", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Get(pattern string, f FilterFunc) {
	p.AddMethod("get", pattern, f)
}

// Post add post method
// usage:
//    Post("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Post(pattern string, f FilterFunc) {
	p.AddMethod("post", pattern, f)
}

// Put add put method
// usage:
//    Put("/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Put(pattern string, f FilterFunc) {
	p.AddMethod("put", pattern, f)
}

// Delete add delete method
// usage:
//    Delete("/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Delete(pattern string, f FilterFunc) {
	p.AddMethod("delete", pattern, f)
}

// Head add head method
// usage:
//    Head("/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Head(pattern string, f FilterFunc) {
	p.AddMethod("head", pattern, f)
}

// Patch add patch method
// usage:
//    Patch("/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Patch(pattern string, f FilterFunc) {
	p.AddMethod("patch", pattern, f)
}

// Options add options method
// usage:
//    Options("/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Options(pattern string, f FilterFunc) {
	p.AddMethod("options", pattern, f)
}

// Any add all method
// usage:
//    Any("/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) Any(pattern string, f FilterFunc) {
	p.AddMethod("*", pattern, f)
}

// AddMethod add http method router
// usage:
//    AddMethod("get","/api/:id", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func (p *ControllerRegister) AddMethod(method, pattern string, f FilterFunc) {
	method = strings.ToUpper(method)
	if method != "*" && !HTTPMETHOD[method] {
		panic("not support http method: " + method)
	}
	route := &ControllerInfo{}
	route.pattern = pattern
	route.routerType = routerTypeRESTFul
	route.runFunction = f
	methods := make(map[string]string)
	if method == "*" {
		for val := range HTTPMETHOD {
			methods[val] = val
		}
	} else {
		methods[method] = method
	}
	route.methods = methods
	for k := range methods {
		if k == "*" {
			for m := range HTTPMETHOD {
				p.addToRouter(m, pattern, route)
			}
		} else {
			p.addToRouter(k, pattern, route)
		}
	}
}

// Handler add user defined Handler
func (p *ControllerRegister) Handler(pattern string, h http.Handler, options ...interface{}) {
	route := &ControllerInfo{}
	route.pattern = pattern
	route.routerType = routerTypeHandler
	route.handler = h
	if len(options) > 0 {
		if _, ok := options[0].(bool); ok {
			pattern = path.Join(pattern, "?:all(.*)")
		}
	}
	for m := range HTTPMETHOD {
		p.addToRouter(m, pattern, route)
	}
}

// AddAuto router to ControllerRegister.
// example beego.AddAuto(&MainContorlller{}),
// MainController has method List and Page.
// visit the url /main/list to execute List function
// /main/page to execute Page function.
func (p *ControllerRegister) AddAuto(c ControllerInterface) {
	p.AddAutoPrefix("/", c)
}

// AddAutoPrefix Add auto router to ControllerRegister with prefix.
// example beego.AddAutoPrefix("/admin",&MainContorlller{}),
// MainController has method List and Page.
// visit the url /admin/main/list to execute List function
// /admin/main/page to execute Page function.
func (p *ControllerRegister) AddAutoPrefix(prefix string, c ControllerInterface) {
	reflectVal := reflect.ValueOf(c)
	rt := reflectVal.Type()
	ct := reflect.Indirect(reflectVal).Type()
	controllerName := strings.TrimSuffix(ct.Name(), "Controller")
	for i := 0; i < rt.NumMethod(); i++ {
		if !utils.InSlice(rt.Method(i).Name, exceptMethod) {
			route := &ControllerInfo{}
			route.routerType = routerTypeBeego
			route.methods = map[string]string{"*": rt.Method(i).Name}
			route.controllerType = ct
			pattern := path.Join(prefix, strings.ToLower(controllerName), strings.ToLower(rt.Method(i).Name), "*")
			patternInit := path.Join(prefix, controllerName, rt.Method(i).Name, "*")
			patternFix := path.Join(prefix, strings.ToLower(controllerName), strings.ToLower(rt.Method(i).Name))
			patternFixInit := path.Join(prefix, controllerName, rt.Method(i).Name)
			route.pattern = pattern
			for m := range HTTPMETHOD {
				p.addToRouter(m, pattern, route)
				p.addToRouter(m, patternInit, route)
				p.addToRouter(m, patternFix, route)
				p.addToRouter(m, patternFixInit, route)
			}
		}
	}
}

// InsertFilter Add a FilterFunc with pattern rule and action constant.
// params is for:
//   1. setting the returnOnOutput value (false allows multiple filters to execute)
//   2. determining whether or not params need to be reset.
func (p *ControllerRegister) InsertFilter(pattern string, pos int, filter FilterFunc, params ...bool) error {
	mr := &FilterRouter{
		tree:           NewTree(),
		pattern:        pattern,
		filterFunc:     filter,
		returnOnOutput: true,
	}
	if !BConfig.RouterCaseSensitive {
		mr.pattern = strings.ToLower(pattern)
	}

	paramsLen := len(params)
	if paramsLen > 0 {
		mr.returnOnOutput = params[0]
	}
	if paramsLen > 1 {
		mr.resetParams = params[1]
	}
	mr.tree.AddRouter(pattern, true)
	return p.insertFilterRouter(pos, mr)
}

// add Filter into
func (p *ControllerRegister) insertFilterRouter(pos int, mr *FilterRouter) (err error) {
	if pos < BeforeStatic || pos > FinishRouter {
		err = fmt.Errorf("can not find your filter position")
		return
	}
	p.enableFilter = true
	p.filters[pos] = append(p.filters[pos], mr)
	return nil
}

// URLFor does another controller handler in this request function.
// it can access any controller method.
func (p *ControllerRegister) URLFor(endpoint string, values ...interface{}) string {
	paths := strings.Split(endpoint, ".")
	if len(paths) <= 1 {
		logs.Warn("urlfor endpoint must like path.controller.method")
		return ""
	}
	if len(values)%2 != 0 {
		logs.Warn("urlfor params must key-value pair")
		return ""
	}
	params := make(map[string]string)
	if len(values) > 0 {
		key := ""
		for k, v := range values {
			if k%2 == 0 {
				key = fmt.Sprint(v)
			} else {
				params[key] = fmt.Sprint(v)
			}
		}
	}
	controllName := strings.Join(paths[:len(paths)-1], "/")
	methodName := paths[len(paths)-1]
	for m, t := range p.routers {
		ok, url := p.geturl(t, "/", controllName, methodName, params, m)
		if ok {
			return url
		}
	}
	return ""
}

func (p *ControllerRegister) geturl(t *Tree, url, controllName, methodName string, params map[string]string, httpMethod string) (bool, string) {
	for _, subtree := range t.fixrouters {
		u := path.Join(url, subtree.prefix)
		ok, u := p.geturl(subtree, u, controllName, methodName, params, httpMethod)
		if ok {
			return ok, u
		}
	}
	if t.wildcard != nil {
		u := path.Join(url, urlPlaceholder)
		ok, u := p.geturl(t.wildcard, u, controllName, methodName, params, httpMethod)
		if ok {
			return ok, u
		}
	}
	for _, l := range t.leaves {
		if c, ok := l.runObject.(*ControllerInfo); ok {
			if c.routerType == routerTypeBeego &&
				strings.HasSuffix(path.Join(c.controllerType.PkgPath(), c.controllerType.Name()), controllName) {
				find := false
				if HTTPMETHOD[strings.ToUpper(methodName)] {
					if len(c.methods) == 0 {
						find = true
					} else if m, ok := c.methods[strings.ToUpper(methodName)]; ok && m == strings.ToUpper(methodName) {
						find = true
					} else if m, ok = c.methods["*"]; ok && m == methodName {
						find = true
					}
				}
				if !find {
					for m, md := range c.methods {
						if (m == "*" || m == httpMethod) && md == methodName {
							find = true
						}
					}
				}
				if find {
					if l.regexps == nil {
						if len(l.wildcards) == 0 {
							return true, strings.Replace(url, "/"+urlPlaceholder, "", 1) + toURL(params)
						}
						if len(l.wildcards) == 1 {
							if v, ok := params[l.wildcards[0]]; ok {
								delete(params, l.wildcards[0])
								return true, strings.Replace(url, urlPlaceholder, v, 1) + toURL(params)
							}
							return false, ""
						}
						if len(l.wildcards) == 3 && l.wildcards[0] == "." {
							if p, ok := params[":path"]; ok {
								if e, isok := params[":ext"]; isok {
									delete(params, ":path")
									delete(params, ":ext")
									return true, strings.Replace(url, urlPlaceholder, p+"."+e, -1) + toURL(params)
								}
							}
						}
						canskip := false
						for _, v := range l.wildcards {
							if v == ":" {
								canskip = true
								continue
							}
							if u, ok := params[v]; ok {
								delete(params, v)
								url = strings.Replace(url, urlPlaceholder, u, 1)
							} else {
								if canskip {
									canskip = false
									continue
								}
								return false, ""
							}
						}
						return true, url + toURL(params)
					}
					var i int
					var startreg bool
					regurl := ""
					for _, v := range strings.Trim(l.regexps.String(), "^$") {
						if v == '(' {
							startreg = true
							continue
						} else if v == ')' {
							startreg = false
							if v, ok := params[l.wildcards[i]]; ok {
								delete(params, l.wildcards[i])
								regurl = regurl + v
								i++
							} else {
								break
							}
						} else if !startreg {
							regurl = string(append([]rune(regurl), v))
						}
					}
					if l.regexps.MatchString(regurl) {
						ps := strings.Split(regurl, "/")
						for _, p := range ps {
							url = strings.Replace(url, urlPlaceholder, p, 1)
						}
						return true, url + toURL(params)
					}
				}
			}
		}
	}

	return false, ""
}

func (p *ControllerRegister) execFilter(context *beecontext.Context, urlPath string, pos int) (started bool) {
	var preFilterParams map[string]string
	for _, filterR := range p.filters[pos] {
		if filterR.returnOnOutput && context.ResponseWriter.Started {
			return true
		}
		if filterR.resetParams {
			preFilterParams = context.Input.Params()
		}
		if ok := filterR.ValidRouter(urlPath, context); ok {
			filterR.filterFunc(context)
			if filterR.resetParams {
				context.Input.ResetParams()
				for k, v := range preFilterParams {
					context.Input.SetParam(k, v)
				}
			}
		}
		if filterR.returnOnOutput && context.ResponseWriter.Started {
			return true
		}
	}
	return false
}

// Implement http.Handler interface.
func (p *ControllerRegister) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	var (
		runRouter    reflect.Type
		findRouter   bool
		runMethod    string
		methodParams []*param.MethodParam
		routerInfo   *ControllerInfo
		isRunnable   bool
	)
	context := p.pool.Get().(*beecontext.Context)
	context.Reset(rw, r)

	defer p.pool.Put(context)
	if BConfig.RecoverFunc != nil {
		defer BConfig.RecoverFunc(context)
	}

	context.Output.EnableGzip = BConfig.EnableGzip

	if BConfig.RunMode == DEV {
		context.Output.Header("Server", BConfig.ServerName)
	}

	var urlPath = r.URL.Path

	if !BConfig.RouterCaseSensitive {
		urlPath = strings.ToLower(urlPath)
	}

	// filter wrong http method
	if !HTTPMETHOD[r.Method] {
		http.Error(rw, "Method Not Allowed", 405)
		goto Admin
	}

	// filter for static file
	if len(p.filters[BeforeStatic]) > 0 && p.execFilter(context, urlPath, BeforeStatic) {
		goto Admin
	}

	serverStaticRouter(context)

	if context.ResponseWriter.Started {
		findRouter = true
		goto Admin
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if BConfig.CopyRequestBody && !context.Input.IsUpload() {
			context.Input.CopyBody(BConfig.MaxMemory)
		}
		context.Input.ParseFormOrMulitForm(BConfig.MaxMemory)
	}

	// session init
	if BConfig.WebConfig.Session.SessionOn {
		var err error
		context.Input.CruSession, err = GlobalSessions.SessionStart(rw, r)
		if err != nil {
			logs.Error(err)
			exception("503", context)
			goto Admin
		}
		defer func() {
			if context.Input.CruSession != nil {
				context.Input.CruSession.SessionRelease(rw)
			}
		}()
	}
	if len(p.filters[BeforeRouter]) > 0 && p.execFilter(context, urlPath, BeforeRouter) {
		goto Admin
	}
	// User can define RunController and RunMethod in filter
	if context.Input.RunController != nil && context.Input.RunMethod != "" {
		findRouter = true
		runMethod = context.Input.RunMethod
		runRouter = context.Input.RunController
	} else {
		routerInfo, findRouter = p.FindRouter(context)
	}

	//if no matches to url, throw a not found exception
	if !findRouter {
		exception("404", context)
		goto Admin
	}
	if splat := context.Input.Param(":splat"); splat != "" {
		for k, v := range strings.Split(splat, "/") {
			context.Input.SetParam(strconv.Itoa(k), v)
		}
	}

	//execute middleware filters
	if len(p.filters[BeforeExec]) > 0 && p.execFilter(context, urlPath, BeforeExec) {
		goto Admin
	}

	//check policies
	if p.execPolicy(context, urlPath) {
		goto Admin
	}

	if routerInfo != nil {
		//store router pattern into context
		context.Input.SetData("RouterPattern", routerInfo.pattern)
		if routerInfo.routerType == routerTypeRESTFul {
			if _, ok := routerInfo.methods[r.Method]; ok {
				isRunnable = true
				routerInfo.runFunction(context)
			} else {
				exception("405", context)
				goto Admin
			}
		} else if routerInfo.routerType == routerTypeHandler {
			isRunnable = true
			routerInfo.handler.ServeHTTP(rw, r)
		} else {
			runRouter = routerInfo.controllerType
			methodParams = routerInfo.methodParams
			method := r.Method
			if r.Method == http.MethodPost && context.Input.Query("_method") == http.MethodPost {
				method = http.MethodPut
			}
			if r.Method == http.MethodPost && context.Input.Query("_method") == http.MethodDelete {
				method = http.MethodDelete
			}
			if m, ok := routerInfo.methods[method]; ok {
				runMethod = m
			} else if m, ok = routerInfo.methods["*"]; ok {
				runMethod = m
			} else {
				runMethod = method
			}
		}
	}

	// also defined runRouter & runMethod from filter
	if !isRunnable {
		//Invoke the request handler
		var execController ControllerInterface
		if routerInfo.initialize != nil {
			execController = routerInfo.initialize()
		} else {
			vc := reflect.New(runRouter)
			var ok bool
			execController, ok = vc.Interface().(ControllerInterface)
			if !ok {
				panic("controller is not ControllerInterface")
			}
		}

		//call the controller init function
		execController.Init(context, runRouter.Name(), runMethod, execController)

		//call prepare function
		execController.Prepare()

		//if XSRF is Enable then check cookie where there has any cookie in the  request's cookie _csrf
		if BConfig.WebConfig.EnableXSRF {
			execController.XSRFToken()
			if r.Method == http.MethodPost || r.Method == http.MethodDelete || r.Method == http.MethodPut ||
				(r.Method == http.MethodPost && (context.Input.Query("_method") == http.MethodDelete || context.Input.Query("_method") == http.MethodPut)) {
				execController.CheckXSRFCookie()
			}
		}

		execController.URLMapping()

		if !context.ResponseWriter.Started {
			//exec main logic
			switch runMethod {
			case http.MethodGet:
				execController.Get()
			case http.MethodPost:
				execController.Post()
			case http.MethodDelete:
				execController.Delete()
			case http.MethodPut:
				execController.Put()
			case http.MethodHead:
				execController.Head()
			case http.MethodPatch:
				execController.Patch()
			case http.MethodOptions:
				execController.Options()
			default:
				if !execController.HandlerFunc(runMethod) {
					vc := reflect.ValueOf(execController)
					method := vc.MethodByName(runMethod)
					in := param.ConvertParams(methodParams, method.Type(), context)
					out := method.Call(in)

					//For backward compatibility we only handle response if we had incoming methodParams
					if methodParams != nil {
						p.handleParamResponse(context, execController, out)
					}
				}
			}

			//render template
			if !context.ResponseWriter.Started && context.Output.Status == 0 {
				if BConfig.WebConfig.AutoRender {
					if err := execController.Render(); err != nil {
						logs.Error(err)
					}
				}
			}
		}

		// finish all runRouter. release resource
		execController.Finish()
	}

	//execute middleware filters
	if len(p.filters[AfterExec]) > 0 && p.execFilter(context, urlPath, AfterExec) {
		goto Admin
	}

	if len(p.filters[FinishRouter]) > 0 && p.execFilter(context, urlPath, FinishRouter) {
		goto Admin
	}

Admin:
	//admin module record QPS

	statusCode := context.ResponseWriter.Status
	if statusCode == 0 {
		statusCode = 200
	}

	logAccess(context, &startTime, statusCode)

	if BConfig.Listen.EnableAdmin {
		timeDur := time.Since(startTime)
		pattern := ""
		if routerInfo != nil {
			pattern = routerInfo.pattern
		}

		if FilterMonitorFunc(r.Method, r.URL.Path, timeDur, pattern, statusCode) {
			if runRouter != nil {
				go toolbox.StatisticsMap.AddStatistics(r.Method, r.URL.Path, runRouter.Name(), timeDur)
			} else {
				go toolbox.StatisticsMap.AddStatistics(r.Method, r.URL.Path, "", timeDur)
			}
		}
	}

	if BConfig.RunMode == DEV && !BConfig.Log.AccessLogs {
		var devInfo string
		timeDur := time.Since(startTime)
		iswin := (runtime.GOOS == "windows")
		statusColor := logs.ColorByStatus(iswin, statusCode)
		methodColor := logs.ColorByMethod(iswin, r.Method)
		resetColor := logs.ColorByMethod(iswin, "")
		if findRouter {
			if routerInfo != nil {
				devInfo = fmt.Sprintf("|%15s|%s %3d %s|%13s|%8s|%s %-7s %s %-3s   r:%s", context.Input.IP(), statusColor, statusCode,
					resetColor, timeDur.String(), "match", methodColor, r.Method, resetColor, r.URL.Path,
					routerInfo.pattern)
			} else {
				devInfo = fmt.Sprintf("|%15s|%s %3d %s|%13s|%8s|%s %-7s %s %-3s", context.Input.IP(), statusColor, statusCode, resetColor,
					timeDur.String(), "match", methodColor, r.Method, resetColor, r.URL.Path)
			}
		} else {
			devInfo = fmt.Sprintf("|%15s|%s %3d %s|%13s|%8s|%s %-7s %s %-3s", context.Input.IP(), statusColor, statusCode, resetColor,
				timeDur.String(), "nomatch", methodColor, r.Method, resetColor, r.URL.Path)
		}
		if iswin {
			logs.W32Debug(devInfo)
		} else {
			logs.Debug(devInfo)
		}
	}
	// Call WriteHeader if status code has been set changed
	if context.Output.Status != 0 {
		context.ResponseWriter.WriteHeader(context.Output.Status)
	}
}

func (p *ControllerRegister) handleParamResponse(context *beecontext.Context, execController ControllerInterface, results []reflect.Value) {
	//looping in reverse order for the case when both error and value are returned and error sets the response status code
	for i := len(results) - 1; i >= 0; i-- {
		result := results[i]
		if result.Kind() != reflect.Interface || !result.IsNil() {
			resultValue := result.Interface()
			context.RenderMethodResult(resultValue)
		}
	}
	if !context.ResponseWriter.Started && len(results) > 0 && context.Output.Status == 0 {
		context.Output.SetStatus(200)
	}
}

// FindRouter Find Router info for URL
func (p *ControllerRegister) FindRouter(context *beecontext.Context) (routerInfo *ControllerInfo, isFind bool) {
	var urlPath = context.Input.URL()
	if !BConfig.RouterCaseSensitive {
		urlPath = strings.ToLower(urlPath)
	}
	httpMethod := context.Input.Method()
	if t, ok := p.routers[httpMethod]; ok {
		runObject := t.Match(urlPath, context)
		if r, ok := runObject.(*ControllerInfo); ok {
			return r, true
		}
	}
	return
}

func toURL(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	u := "?"
	for k, v := range params {
		u += k + "=" + v + "&"
	}
	return strings.TrimRight(u, "&")
}

func logAccess(ctx *beecontext.Context, startTime *time.Time, statusCode int) {
	//Skip logging if AccessLogs config is false
	if !BConfig.Log.AccessLogs {
		return
	}
	//Skip logging static requests unless EnableStaticLogs config is true
	if !BConfig.Log.EnableStaticLogs && DefaultAccessLogFilter.Filter(ctx) {
		return
	}
	var (
		requestTime time.Time
		elapsedTime time.Duration
		r           = ctx.Request
	)
	if startTime != nil {
		requestTime = *startTime
		elapsedTime = time.Since(*startTime)
	}
	record := &logs.AccessLogRecord{
		RemoteAddr:     ctx.Input.IP(),
		RequestTime:    requestTime,
		RequestMethod:  r.Method,
		Request:        fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto),
		ServerProtocol: r.Proto,
		Host:           r.Host,
		Status:         statusCode,
		ElapsedTime:    elapsedTime,
		HTTPReferrer:   r.Header.Get("Referer"),
		HTTPUserAgent:  r.Header.Get("User-Agent"),
		RemoteUser:     r.Header.Get("Remote-User"),
		BodyBytesSent:  0, //@todo this one is missing!
	}
	logs.AccessLog(record, BConfig.Log.AccessLogsFormat)
}
