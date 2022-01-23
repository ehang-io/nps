package controller

import (
	"ehang.io/nps/db"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

type configServer struct {
	db db.Db
}

func (cs *configServer) ChangeSystemConfig(c *gin.Context) {
	type config struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
		NewUsername string `json:"new_username"`
	}
	var cfg config
	err := c.BindJSON(&cfg)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	oldPassword, err := cs.db.GetConfig("admin_pass")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	fmt.Println(cfg, oldPassword)
	if cfg.OldPassword != oldPassword {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "old password does not match"})
		return
	}
	if err := cs.db.SetConfig("admin_pass", cfg.NewPassword); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	if cfg.NewUsername != "" {
		if err := cs.db.SetConfig("admin_pass", cfg.NewUsername); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}
