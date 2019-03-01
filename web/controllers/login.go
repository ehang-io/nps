package controllers

import (
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
)

type LoginController struct {
	beego.Controller
}

func (self *LoginController) Index() {
	self.TplName = "login/index.html"
}
func (self *LoginController) Verify() {
	if self.GetString("password") == beego.AppConfig.String("password") && self.GetString("username") == beego.AppConfig.String("username") {
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
	self.Redirect("/login/index", 302)
}
