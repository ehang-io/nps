package controllers

import (
	"time"

	"github.com/astaxie/beego"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/server"
)

type LoginController struct {
	beego.Controller
}

func (self *LoginController) Index() {
	self.Data["register_allow"], _ = beego.AppConfig.Bool("allow_user_register")
	self.TplName = "login/index.html"
}
func (self *LoginController) Verify() {
	var auth bool
	if self.GetString("password") == beego.AppConfig.String("web_password") && self.GetString("username") == beego.AppConfig.String("web_username") {
		self.SetSession("isAdmin", true)
		auth = true
		server.Bridge.Register.Store(common.GetIpByAddr(self.Ctx.Input.IP()), time.Now().Add(time.Hour*time.Duration(2)))
	}
	b, err := beego.AppConfig.Bool("allow_user_login")
	if err == nil && b && !auth {
		file.GetDb().JsonDb.Clients.Range(func(key, value interface{}) bool {
			v := value.(*file.Client)
			if !v.Status || v.NoDisplay {
				return true
			}
			if v.WebUserName == "" && v.WebPassword == "" {
				if self.GetString("username") != "user" || v.VerifyKey != self.GetString("password") {
					return true
				} else {
					auth = true
				}
			}
			if !auth && v.WebPassword == self.GetString("password") && self.GetString("username") == v.WebUserName {
				auth = true
			}
			if auth {
				self.SetSession("isAdmin", false)
				self.SetSession("clientId", v.Id)
				self.SetSession("username", v.WebUserName)
				return false
			}
			return true
		})
	}
	if auth {
		self.SetSession("auth", true)
		self.Data["json"] = map[string]interface{}{"status": 1, "msg": "login success"}
	} else {
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "username or password incorrect"}
	}
	self.ServeJSON()
}
func (self *LoginController) Register() {
	if self.Ctx.Request.Method == "GET" {
		self.TplName = "login/register.html"
	} else {
		if b, err := beego.AppConfig.Bool("allow_user_register"); err != nil || !b {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "register is not allow"}
			self.ServeJSON()
			return
		}
		if self.GetString("username") == "" || self.GetString("password") == "" || self.GetString("username") == beego.AppConfig.String("web_username") {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "please check your input"}
			self.ServeJSON()
			return
		}
		t := &file.Client{
			Id:          int(file.GetDb().JsonDb.GetClientId()),
			Status:      true,
			Cnf:         &file.Config{},
			WebUserName: self.GetString("username"),
			WebPassword: self.GetString("password"),
			Flow:        &file.Flow{},
		}
		if err := file.GetDb().NewClient(t); err != nil {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": err.Error()}
		} else {
			self.Data["json"] = map[string]interface{}{"status": 1, "msg": "register success"}
		}
		self.ServeJSON()
	}
}

func (self *LoginController) Out() {
	self.SetSession("auth", false)
	self.Redirect("/login/index", 302)
}
