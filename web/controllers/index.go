package controllers

import (
	"github.com/cnlh/nps/server"
	"github.com/cnlh/nps/utils"
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

func (s *IndexController) GetTunnel() {
	start, length := s.GetAjaxParams()
	taskType := s.GetString("type")
	clientId := s.GetIntNoErr("client_id")
	list, cnt := server.GetTunnel(start, length, taskType, clientId)
	s.AjaxTable(list, cnt, cnt)
}

func (s *IndexController) Add() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["type"] = s.GetString("type")
		s.Data["client_id"] = s.GetString("client_id")
		s.SetInfo("新增")
		s.display()
	} else {
		t := &utils.Tunnel{
			TcpPort: s.GetIntNoErr("port"),
			Mode:    s.GetString("type"),
			Target:  s.GetString("target"),
			Config: &utils.Config{
				U:        s.GetString("u"),
				P:        s.GetString("p"),
				Compress: s.GetString("compress"),
				Crypt:    s.GetBoolNoErr("crypt"),
			},
			Id:           server.CsvDb.GetTaskId(),
			UseClientCnf: s.GetBoolNoErr("use_client"),
			Status:       true,
			Remark:       s.GetString("remark"),
			Flow:         &utils.Flow{},
		}
		var err error
		if t.Client, err = server.CsvDb.GetClient(s.GetIntNoErr("client_id")); err != nil {
			s.AjaxErr(err.Error())
		}
		server.CsvDb.NewTask(t)
		if err := server.AddTask(t); err != nil {
			s.AjaxErr(err.Error())
		} else {
			s.AjaxOk("添加成功")
		}
	}
}
func (s *IndexController) GetOneTunnel() {
	id := s.GetIntNoErr("id")
	data := make(map[string]interface{})
	if t, err := server.CsvDb.GetTask(id); err != nil {
		data["code"] = 0
	} else {
		data["code"] = 1
		data["data"] = t
	}
	s.Data["json"] = data
	s.ServeJSON()
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
			t.Client.Id = s.GetIntNoErr("client_id")
			t.Config.U = s.GetString("u")
			t.Config.P = s.GetString("p")
			t.Config.Compress = s.GetString("compress")
			t.Config.Crypt = s.GetBoolNoErr("crypt")
			t.UseClientCnf = s.GetBoolNoErr("use_client")
			t.Remark = s.GetString("remark")
			if t.Client, err = server.CsvDb.GetClient(s.GetIntNoErr("client_id")); err != nil {
				s.AjaxErr("修改失败")
			}
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
		list, cnt := server.CsvDb.GetHost(start, length, clientId)
		s.AjaxTable(list, cnt, cnt)
	}
}

func (s *IndexController) GetHost() {
	if s.Ctx.Request.Method == "POST" {
		data := make(map[string]interface{})
		if h, err := server.GetInfoByHost(s.GetString("host")); err != nil {
			data["code"] = 0
		} else {
			data["data"] = h
			data["code"] = 1
		}
		s.Data["json"] = data
		s.ServeJSON()
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
		h := &utils.Host{
			Host:         s.GetString("host"),
			Target:       s.GetString("target"),
			HeaderChange: s.GetString("header"),
			HostChange:   s.GetString("hostchange"),
			Remark:       s.GetString("remark"),
			Flow:         &utils.Flow{},
		}
		var err error
		if h.Client, err = server.CsvDb.GetClient(s.GetIntNoErr("client_id")); err != nil {
			s.AjaxErr("添加失败")
		}
		server.CsvDb.NewHost(h)
		s.AjaxOk("添加成功")
	}
}

func (s *IndexController) EditHost() {
	host := s.GetString("host")
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "host"
		if h, err := server.GetInfoByHost(host); err != nil {
			s.error()
		} else {
			s.Data["h"] = h
		}
		s.SetInfo("修改")
		s.display("index/hedit")
	} else {
		if h, err := server.GetInfoByHost(host); err != nil {
			s.error()
		} else {
			h.Host = s.GetString("nhost")
			h.Target = s.GetString("target")
			h.HeaderChange = s.GetString("header")
			h.HostChange = s.GetString("hostchange")
			h.Remark = s.GetString("remark")
			h.TargetArr = nil
			server.CsvDb.UpdateHost(h)
			var err error
			if h.Client, err = server.CsvDb.GetClient(s.GetIntNoErr("client_id")); err != nil {
				s.AjaxErr("修改失败")
			}
		}
		s.AjaxOk("修改成功")
	}
}
