package controllers

import (
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/rate"
	"github.com/cnlh/nps/server"
)

type ClientController struct {
	BaseController
}

func (s *ClientController) List() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "client"
		s.SetInfo("客户端管理")
		s.display("client/list")
		return
	}
	start, length := s.GetAjaxParams()
	list, cnt := server.GetClientList(start, length)
	s.AjaxTable(list, cnt, cnt)
}

//添加客户端
func (s *ClientController) Add() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "client"
		s.SetInfo("新增")
		s.display()
	} else {
		t := &file.Client{
			VerifyKey: crypt.GetRandomString(16),
			Id:        file.GetCsvDb().GetClientId(),
			Status:    true,
			Remark:    s.GetString("remark"),
			Cnf: &file.Config{
				U:        s.GetString("u"),
				P:        s.GetString("p"),
				Compress: s.GetString("compress"),
				Crypt:    s.GetBoolNoErr("crypt"),
			},
			RateLimit: s.GetIntNoErr("rate_limit"),
			Flow: &file.Flow{
				ExportFlow: 0,
				InletFlow:  0,
				FlowLimit:  int64(s.GetIntNoErr("flow_limit")),
			},
		}
		if t.RateLimit > 0 {
			t.Rate = rate.NewRate(int64(t.RateLimit * 1024))
			t.Rate.Start()
		}
		file.GetCsvDb().NewClient(t)
		s.AjaxOk("添加成功")
	}
}
func (s *ClientController) GetClient() {
	if s.Ctx.Request.Method == "POST" {
		id := s.GetIntNoErr("id")
		data := make(map[string]interface{})
		if c, err := file.GetCsvDb().GetClient(id); err != nil {
			data["code"] = 0
		} else {
			data["code"] = 1
			data["data"] = c
		}
		s.Data["json"] = data
		s.ServeJSON()
	}
}

//修改客户端
func (s *ClientController) Edit() {
	id := s.GetIntNoErr("id")
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "client"
		if c, err := file.GetCsvDb().GetClient(id); err != nil {
			s.error()
		} else {
			s.Data["c"] = c
		}
		s.SetInfo("修改")
		s.display()
	} else {
		if c, err := file.GetCsvDb().GetClient(id); err != nil {
			s.error()
		} else {
			c.Remark = s.GetString("remark")
			c.Cnf.U = s.GetString("u")
			c.Cnf.P = s.GetString("p")
			c.Cnf.Compress = s.GetString("compress")
			c.Cnf.Crypt = s.GetBoolNoErr("crypt")
			c.Flow.FlowLimit = int64(s.GetIntNoErr("flow_limit"))
			c.RateLimit = s.GetIntNoErr("rate_limit")
			if c.Rate != nil {
				c.Rate.Stop()
			}
			if c.RateLimit > 0 {
				c.Rate = rate.NewRate(int64(c.RateLimit * 1024))
				c.Rate.Start()
			} else {
				c.Rate = nil
			}
			file.GetCsvDb().UpdateClient(c)
		}
		s.AjaxOk("修改成功")
	}
}

//更改状态
func (s *ClientController) ChangeStatus() {
	id := s.GetIntNoErr("id")
	if client, err := file.GetCsvDb().GetClient(id); err == nil {
		client.Status = s.GetBoolNoErr("status")
		if client.Status == false {
			server.DelClientConnect(client.Id)
		}
		s.AjaxOk("修改成功")
	}
	s.AjaxErr("修改失败")
}

//删除客户端
func (s *ClientController) Del() {
	id := s.GetIntNoErr("id")
	if err := file.GetCsvDb().DelClient(id); err != nil {
		s.AjaxErr("删除失败")
	}
	server.DelTunnelAndHostByClientId(id)
	server.DelClientConnect(id)
	s.AjaxOk("删除成功")
}
