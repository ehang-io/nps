package controller

import (
	"ehang.io/nps/core/rule"
	"ehang.io/nps/db"
	"encoding/json"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"net/http"
	"strconv"
)

type baseController struct {
	db        db.Db
	tableName string
}

func (bc *baseController) Page(c *gin.Context) {
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	pageSize, err := strconv.Atoi(c.Query("pageSize"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	dataArr, err := bc.db.QueryPage(bc.tableName, pageSize, (page-1)*pageSize, c.Query("key"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	list := make([]map[string]interface{}, 0)
	for _, s := range dataArr {
		dd := make(map[string]interface{}, 0)
		err = json.Unmarshal([]byte(s), &dd)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
			return
		}
		list = append(list, dd)
	}
	n, err := bc.db.Count(bc.tableName, c.Query("key"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"result": gin.H{
			"items": list,
			"total": n,
		},
	})
}

func (bc *baseController) Delete(c *gin.Context) {
	type uid struct {
		Uuid string
	}
	var js uid
	err := c.BindJSON(&js)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	err = bc.db.Delete(bc.tableName, js.Uuid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

type ruleServer struct {
	baseController
}

func (rs *ruleServer) Create(c *gin.Context) {
	var jr rule.JsonRule
	err := c.BindJSON(&jr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	jr.Uuid = uuid.NewV4().String()
	b, err := json.Marshal(jr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	err = rs.db.Insert(rs.tableName, jr.Uuid, string(b))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (rs *ruleServer) Update(c *gin.Context) {
	var jr rule.JsonRule
	err := c.BindJSON(&jr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	b, err := json.Marshal(jr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	err = rs.db.Update(rs.tableName, jr.Uuid, string(b))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (rs *ruleServer) One(c *gin.Context) {
	var js map[string]string
	err := c.BindJSON(&js)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	s, err := rs.db.QueryOne("rule", js["uuid"])
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	var r rule.JsonRule
	err = json.Unmarshal([]byte(s), &r)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "result": r, "message": "ok"})
}

func (rs *ruleServer) Field(c *gin.Context) {
	chains := rule.GetChains()
	c.JSON(http.StatusOK, gin.H{"code": 0, "result": chains, "message": "ok"})
}

func (rs *ruleServer) Limiter(c *gin.Context) {
	chains := rule.GetLimiters()
	c.JSON(http.StatusOK, gin.H{"code": 0, "result": chains, "message": "ok"})
}
