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
	if self.GetString("psd") == beego.AppConfig.String("password") {
		self.SetSession("auth", true)
		self.Data["json"] = map[string]interface{}{"status": 1, "msg": "验证成功"}
		self.ServeJSON()
	} else {
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "验证失败"}
		self.ServeJSON()
	}
}
func (self *LoginController) Out() {
	self.SetSession("auth", false)
	self.Redirect("/login/index", 302)
}
