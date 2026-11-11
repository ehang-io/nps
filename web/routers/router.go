package routers

import (
	"strings"

	"ehang.io/nps/web/controllers"
	"github.com/astaxie/beego"
)

func Init() {
	// REST API 固定挂在 /api/v1，脚本与 CI 不受 web_base_url 影响；
	// 反向代理到子路径部署时，再额外挂一份到 web_base_url 下面。
	beego.AddNamespace(apiNamespace("/api/v1"))
	web_base_url := beego.AppConfig.String("web_base_url")
	if len(web_base_url) > 0 {
		beego.AddNamespace(apiNamespace(strings.TrimRight(web_base_url, "/") + "/api/v1"))
		ns := beego.NewNamespace(web_base_url,
			beego.NSRouter("/", &controllers.IndexController{}, "*:Index"),
			beego.NSAutoRouter(&controllers.IndexController{}),
			beego.NSAutoRouter(&controllers.LoginController{}),
			beego.NSAutoRouter(&controllers.ClientController{}),
			beego.NSAutoRouter(&controllers.AuthController{}),
			beego.NSAutoRouter(&controllers.SettingController{}),
		)
		beego.AddNamespace(ns)
	} else {
		beego.Router("/", &controllers.IndexController{}, "*:Index")
		beego.AutoRouter(&controllers.IndexController{})
		beego.AutoRouter(&controllers.LoginController{})
		beego.AutoRouter(&controllers.ClientController{})
		beego.AutoRouter(&controllers.AuthController{})
		beego.AutoRouter(&controllers.SettingController{})
	}
}

// apiNamespace 注册 /api/v1 下的全部路由。
// 认证在 ApiController.apiGuard 里统一完成，这里只负责路径到方法的映射。
func apiNamespace(prefix string) *beego.Namespace {
	return beego.NewNamespace(prefix,
		beego.NSRouter("/health", &controllers.ApiPublicController{}, "get:Health"),
		beego.NSRouter("/status", &controllers.ApiController{}, "get:Status"),

		beego.NSRouter("/tunnels", &controllers.ApiController{}, "get:ListTunnels;post:CreateTunnel"),
		beego.NSRouter("/tunnels/:id", &controllers.ApiController{}, "get:GetTunnel;put:UpdateTunnel;patch:UpdateTunnel;delete:DeleteTunnel"),
		beego.NSRouter("/tunnels/:id/start", &controllers.ApiController{}, "post:StartTunnel"),
		beego.NSRouter("/tunnels/:id/stop", &controllers.ApiController{}, "post:StopTunnel"),

		beego.NSRouter("/clients", &controllers.ApiController{}, "get:ListClients;post:CreateClient"),
		beego.NSRouter("/clients/:id", &controllers.ApiController{}, "get:GetClient;put:UpdateClient;patch:UpdateClient;delete:DeleteClient"),

		beego.NSRouter("/hosts", &controllers.ApiController{}, "get:ListHosts;post:CreateHost"),
		beego.NSRouter("/hosts/:id", &controllers.ApiController{}, "get:GetHost;delete:DeleteHost"),
	)
}
