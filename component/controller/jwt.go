package controller

import (
	"ehang.io/nps/db"
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

var identityKey = "id"

type User struct {
	UserName string
}

func newAuthMiddleware(db db.Db) (authMiddleware *jwt.GinJWTMiddleware, err error) {
	authMiddleware, err = jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "nps",
		Key:         []byte("secret key"),
		Timeout:     time.Hour * 24,
		MaxRefresh:  time.Hour * 72,
		IdentityKey: identityKey,
		SendCookie:  true,
		LoginResponse: func(c *gin.Context, code int, message string, time time.Time) {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"result": gin.H{
					"token": message,
				},
				"message": "ok",
			})
		},
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*User); ok {
				return jwt.MapClaims{
					identityKey: v.UserName,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &User{
				UserName: claims[identityKey].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var loginVals login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			userID := loginVals.Username
			password := loginVals.Password
			adminUser, err := db.GetConfig("admin_user")
			if err != nil {
				return "", jwt.ErrFailedAuthentication
			}
			adminPass, err := db.GetConfig("admin_pass")
			if err != nil {
				return "", jwt.ErrFailedAuthentication
			}
			if userID == adminUser && password == adminPass {
				return &User{
					UserName: userID,
				}, nil
			}

			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			adminUser, err := db.GetConfig("admin_user")
			if err != nil {
				return false
			}
			if v, ok := data.(*User); ok && v.UserName ==adminUser {
				return true
			}
			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		TokenLookup:   "header: Authorization, query: token, cookie: jwt",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})
	if err != nil {
		return
	}
	err = authMiddleware.MiddlewareInit()
	if err != nil {
		return
	}
	return
}
