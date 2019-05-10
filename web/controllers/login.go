package controllers

import (
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
)

type LoginController struct {
	beego.Controller
}

func (self *LoginController) Index() {
	self.Data["web_base_url"] = beego.AppConfig.String("web_base_url")
	self.TplName = "login/index.html"
}
func (self *LoginController) Verify() {
	if self.GetString("password") == beego.AppConfig.String("web_password") && self.GetString("username") == beego.AppConfig.String("web_username") {
		self.SetSession("auth", true)
		self.Data["json"] = map[string]interface{}{"status": 1, "msg": "login success"}
		self.ServeJSON()
	} else {
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "username or password incorrect"}
		self.ServeJSON()
	}
}
func (self *LoginController) Out() {
	self.SetSession("auth", false)
	self.Redirect(beego.AppConfig.String("web_base_url")+"/login/index", 302)
}
