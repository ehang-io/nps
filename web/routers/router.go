package routers

import (
	"github.com/astaxie/beego"
	"github.com/cnlh/nps/web/controllers"
)

func init() {
	beego.Router("/", &controllers.IndexController{}, "*:Index")
	beego.AutoRouter(&controllers.IndexController{})
	beego.AutoRouter(&controllers.LoginController{})
	beego.AutoRouter(&controllers.ClientController{})
	beego.AutoRouter(&controllers.AuthController{})
}
