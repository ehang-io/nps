package controller

import (
	"ehang.io/nps/db"
	"ehang.io/nps/lib/logger"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net"
	"net/http"
)

func StartController(ln net.Listener, db db.Db, rootCert []byte, rootKey []byte, staticRootPath string, pagePath string) error {
	gin.SetMode(gin.ReleaseMode)

	cfgServer := &configServer{db: db}
	crtServer := &certServe{baseController: baseController{db: db, tableName: "cert"}}
	err := crtServer.Init(rootCert, rootKey)
	if err != nil {
		return err
	}
	ruleServer := &ruleServer{baseController: baseController{db: db, tableName: "rule"}}

	authMiddleware, err := newAuthMiddleware(db)
	if err != nil {
		return err
	}
	router := gin.New()
	router.Use(CORSMiddleware(), gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.Debug("http request",
			zap.String("client_ip", param.ClientIP),
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.String("proto", param.Request.Proto),
			zap.Duration("latency", param.Latency),
			zap.String("user_agent", param.Request.UserAgent()),
			zap.String("error_message", param.ErrorMessage),
			zap.Int("response_code", param.StatusCode),
		)
		return ""
	}))
	router.POST("/login", authMiddleware.LoginHandler)

	router.NoRoute(authMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		logger.Warn("NoRoute", zap.Any("claims", claims))
		c.JSON(404, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	auth := router.Group("/auth")
	auth.Use(authMiddleware.MiddlewareFunc())
	auth.GET("/refresh_token", authMiddleware.RefreshHandler)
	auth.GET("/userinfo", userinfo)

	v1 := router.Group("v1")
	v1.Use(authMiddleware.MiddlewareFunc())
	{
		v1.PUT("/config", cfgServer.ChangeSystemConfig)

		v1.GET("/status", status)

		v1.POST("/cert", crtServer.Create)
		v1.DELETE("/cert", crtServer.Delete)
		v1.GET("/cert/page", crtServer.Page)
		v1.PUT("/cert", crtServer.Update)

		v1.POST("/rule", ruleServer.Create)
		v1.DELETE("/rule", ruleServer.Delete)
		v1.PUT("/rule", ruleServer.Update)
		v1.GET("/rule", ruleServer.One)
		v1.GET("/rule/page", ruleServer.Page)
		v1.GET("/rule/field", ruleServer.Field)
		v1.GET("/rule/limiter", ruleServer.Limiter)
	}
	router.Static("/static/", staticRootPath)
	router.Static("/page/", pagePath)

	go storeSystemStatus()

	return router.RunListener(ln)
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func userinfo(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8",
		[]byte(`{"code":0,"result":{"userId":"1","username":"vben","realName":"Vben Admin","avatar":"https://q1.qlogo.cn/g?b=qq&nk=190848757&s=640","desc":"manager","password":"123456","token":"fakeToken1","homePath":"/dashboard/analysis","roles":[{"roleName":"Super Admin","value":"super"}]},"message":"ok","type":"success"}`))
}
