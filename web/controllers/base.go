package controllers

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/server"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"strconv"
	"strings"
	"time"
)

type BaseController struct {
	beego.Controller
	controllerName string
	actionName     string
}

//初始化参数
func (s *BaseController) Prepare() {
	controllerName, actionName := s.GetControllerAndAction()
	s.controllerName = strings.ToLower(controllerName[0 : len(controllerName)-10])
	s.actionName = strings.ToLower(actionName)
	// web api verify
	// param 1 is md5(authKey+Current timestamp)
	// param 2 is timestamp (It's limited to 20 seconds.)
	md5Key := s.GetString("auth_key")
	timestamp := s.GetIntNoErr("timestamp")
	configKey := beego.AppConfig.String("authKey")
	if !(time.Now().Unix()-int64(timestamp) < 20 && time.Now().Unix()-int64(timestamp) > 0 && crypt.Md5(configKey+strconv.Itoa(timestamp)) == md5Key) {
		if s.GetSession("auth") != true {
			s.Redirect("/login/index", 302)
		}
	}
}

//加载模板
func (s *BaseController) display(tpl ...string) {
	var tplname string
	if s.Data["menu"] == nil {
		s.Data["menu"] = s.actionName
	}
	if len(tpl) > 0 {
		tplname = strings.Join([]string{tpl[0], "html"}, ".")
	} else {
		tplname = s.controllerName + "/" + s.actionName + ".html"
	}
	ip := s.Ctx.Request.Host
	if strings.LastIndex(ip, ":") > 0 {
		arr := strings.Split(common.GetHostByName(ip), ":")
		s.Data["ip"] = arr[0]
	}
	s.Data["p"] = server.Bridge.TunnelPort
	s.Data["proxyPort"] = beego.AppConfig.String("hostPort")
	s.Layout = "public/layout.html"
	s.TplName = tplname
}

//错误
func (s *BaseController) error() {
	s.Layout = "public/layout.html"
	s.TplName = "public/error.html"
}

//去掉没有err返回值的int
func (s *BaseController) GetIntNoErr(key string, def ...int) int {
	strv := s.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0]
	}
	val, _ := strconv.Atoi(strv)
	return val
}

//获取去掉错误的bool值
func (s *BaseController) GetBoolNoErr(key string, def ...bool) bool {
	strv := s.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0]
	}
	val, _ := strconv.ParseBool(strv)
	return val
}

//ajax正确返回
func (s *BaseController) AjaxOk(str string) {
	s.Data["json"] = ajax(str, 1)
	s.ServeJSON()
	s.StopRun()
}

//ajax错误返回
func (s *BaseController) AjaxErr(str string) {
	s.Data["json"] = ajax(str, 0)
	s.ServeJSON()
	s.StopRun()
}

//组装ajax
func ajax(str string, status int) map[string]interface{} {
	json := make(map[string]interface{})
	json["status"] = status
	json["msg"] = str
	return json
}

//ajax table返回
func (s *BaseController) AjaxTable(list interface{}, cnt int, recordsTotal int) {
	json := make(map[string]interface{})
	json["rows"] = list
	json["total"] = recordsTotal
	s.Data["json"] = json
	s.ServeJSON()
	s.StopRun()
}

//ajax table参数
func (s *BaseController) GetAjaxParams() (start, limit int) {
	return s.GetIntNoErr("offset"), s.GetIntNoErr("limit")
}

func (s *BaseController) SetInfo(name string) {
	s.Data["name"] = name
}

func (s *BaseController) SetType(name string) {
	s.Data["type"] = name
}
