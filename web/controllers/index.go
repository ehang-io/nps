package controllers

import (
	"github.com/cnlh/easyProxy/server"
	"github.com/cnlh/easyProxy/utils"
)

type IndexController struct {
	BaseController
}

func (s *IndexController) Index() {
	s.Data["data"] = server.GetDashboardData()
	s.SetInfo("dashboard")
	s.display("index/index")
}
func (s *IndexController) Help() {
	s.SetInfo("使用说明")
	s.display("index/help")
}

func (s *IndexController) Tcp() {
	s.SetInfo("tcp隧道管理")
	s.SetType("tunnelServer")
	s.display("index/list")
}

func (s *IndexController) Udp() {
	s.SetInfo("udp隧道管理")
	s.SetType("udpServer")
	s.display("index/list")
}

func (s *IndexController) Socks5() {
	s.SetInfo("socks5管理")
	s.SetType("socks5Server")
	s.display("index/list")
}

func (s *IndexController) Http() {
	s.SetInfo("http代理管理")
	s.SetType("httpProxyServer")
	s.display("index/list")
}

func (s *IndexController) Host() {
	s.SetInfo("host模式管理")
	s.SetType("hostServer")
	s.display("index/list")
}

func (s *IndexController) All() {
	s.Data["menu"] = "client"
	clientId := s.GetString("client_id")
	s.Data["client_id"] = clientId
	s.SetInfo("客户端" + clientId + "的所有隧道")
	s.display("index/list")
}

func (s *IndexController) GetServerConfig() {
	start, length := s.GetAjaxParams()
	taskType := s.GetString("type")
	clientId := s.GetIntNoErr("client_id")
	list, cnt := server.GetServerConfig(start, length, taskType, clientId)
	s.AjaxTable(list, cnt, cnt)
}

func (s *IndexController) Add() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["type"] = s.GetString("type")
		s.Data["client_id"] = s.GetString("client_id")
		s.SetInfo("新增")
		s.display()
	} else {
		t := &utils.ServerConfig{
			TcpPort:      s.GetIntNoErr("port"),
			Mode:         s.GetString("type"),
			Target:       s.GetString("target"),
			U:            s.GetString("u"),
			P:            s.GetString("p"),
			Compress:     s.GetString("compress"),
			Crypt:        s.GetBoolNoErr("crypt"),
			Mux:          s.GetBoolNoErr("mux"),
			IsRun:        0,
			Id:           server.CsvDb.GetTaskId(),
			ClientId:     s.GetIntNoErr("client_id"),
			UseClientCnf: s.GetBoolNoErr("use_client"),
			Start:        1,
			Remark:       s.GetString("remark"),
		}
		server.CsvDb.NewTask(t)
		if err := server.AddTask(t); err != nil {
			s.AjaxErr(err.Error())
		} else {
			s.AjaxOk("添加成功")
		}
	}
}

func (s *IndexController) Edit() {
	id := s.GetIntNoErr("id")
	if s.Ctx.Request.Method == "GET" {
		if t, err := server.CsvDb.GetTask(id); err != nil {
			s.error()
		} else {
			s.Data["t"] = t
		}
		s.SetInfo("修改")
		s.display()
	} else {
		if t, err := server.CsvDb.GetTask(id); err != nil {
			s.error()
		} else {
			t.TcpPort = s.GetIntNoErr("port")
			t.Mode = s.GetString("type")
			t.Target = s.GetString("target")
			t.Id = id
			t.ClientId = s.GetIntNoErr("client_id")
			t.U = s.GetString("u")
			t.P = s.GetString("p")
			t.Compress = s.GetString("compress")
			t.Crypt = s.GetBoolNoErr("crypt")
			t.Mux = s.GetBoolNoErr("mux")
			t.UseClientCnf = s.GetBoolNoErr("use_client")
			t.Remark = s.GetString("remark")
			server.CsvDb.UpdateTask(t)
		}
		s.AjaxOk("修改成功")
	}
}

func (s *IndexController) Stop() {
	id := s.GetIntNoErr("id")
	if err := server.StopServer(id); err != nil {
		s.AjaxErr("停止失败")
	}
	s.AjaxOk("停止成功")
}

func (s *IndexController) Del() {
	id := s.GetIntNoErr("id")
	if err := server.DelTask(id); err != nil {
		s.AjaxErr("删除失败")
	}
	s.AjaxOk("删除成功")
}

func (s *IndexController) Start() {
	id := s.GetIntNoErr("id")
	if err := server.StartTask(id); err != nil {
		s.AjaxErr("开启失败")
	}
	s.AjaxOk("开启成功")
}

func (s *IndexController) HostList() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["client_id"] = s.GetString("client_id")
		s.Data["menu"] = "host"
		s.SetInfo("域名列表")
		s.display("index/hlist")
	} else {
		start, length := s.GetAjaxParams()
		clientId := s.GetIntNoErr("client_id")
		list, cnt := server.CsvDb.GetHostList(start, length, clientId)
		s.AjaxTable(list, cnt, cnt)
	}
}

func (s *IndexController) DelHost() {
	host := s.GetString("host")
	if err := server.CsvDb.DelHost(host); err != nil {
		s.AjaxErr("删除失败")
	}
	s.AjaxOk("删除成功")
}

func (s *IndexController) AddHost() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["client_id"] = s.GetString("client_id")
		s.Data["menu"] = "host"
		s.SetInfo("新增")
		s.display("index/hadd")
	} else {
		h := &utils.HostList{
			ClientId:     s.GetIntNoErr("client_id"),
			Host:         s.GetString("host"),
			Target:       s.GetString("target"),
			HeaderChange: s.GetString("header"),
			HostChange:   s.GetString("hostchange"),
			Remark:       s.GetString("remark"),
		}
		server.CsvDb.NewHost(h)
		s.AjaxOk("添加成功")
	}
}

func (s *IndexController) EditHost() {
	host := s.GetString("host")
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "host"
		if h, _, err := server.GetKeyByHost(host); err != nil {
			s.error()
		} else {
			s.Data["h"] = h
		}
		s.SetInfo("修改")
		s.display("index/hedit")
	} else {
		if h, _, err := server.GetKeyByHost(host); err != nil {
			s.error()
		} else {
			h.ClientId = s.GetIntNoErr("client_id")
			h.Host = s.GetString("nhost")
			h.Target = s.GetString("target")
			h.HeaderChange = s.GetString("header")
			h.HostChange = s.GetString("hostchange")
			h.Remark = s.GetString("remark")
			server.CsvDb.UpdateHost(h)
		}
		s.AjaxOk("修改成功")
	}
}
