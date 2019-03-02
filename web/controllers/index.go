package controllers

import (
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/server"
	"github.com/cnlh/nps/server/tool"
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
	s.SetInfo("about")
	s.display("index/help")
}

func (s *IndexController) Tcp() {
	s.SetInfo("tcp")
	s.SetType("tcp")
	s.display("index/list")
}

func (s *IndexController) Udp() {
	s.SetInfo("udp")
	s.SetType("udp")
	s.display("index/list")
}

func (s *IndexController) Socks5() {
	s.SetInfo("socks5")
	s.SetType("socks5")
	s.display("index/list")
}

func (s *IndexController) Http() {
	s.SetInfo("http proxy")
	s.SetType("httpProxy")
	s.display("index/list")
}
func (s *IndexController) File() {
	s.SetInfo("file server")
	s.SetType("file")
	s.display("index/list")
}

func (s *IndexController) Secret() {
	s.SetInfo("secret")
	s.SetType("secret")
	s.display("index/list")
}
func (s *IndexController) P2p() {
	s.SetInfo("p2p")
	s.SetType("p2p")
	s.display("index/list")
}

func (s *IndexController) Host() {
	s.SetInfo("host")
	s.SetType("hostServer")
	s.display("index/list")
}

func (s *IndexController) All() {
	s.Data["menu"] = "client"
	clientId := s.GetString("client_id")
	s.Data["client_id"] = clientId
	s.SetInfo("client id:" + clientId)
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
		s.SetInfo("add tunnel")
		s.display()
	} else {
		t := &file.Tunnel{
			Port:      s.GetIntNoErr("port"),
			Mode:      s.GetString("type"),
			Target:    s.GetString("target"),
			Id:        file.GetCsvDb().GetTaskId(),
			Status:    true,
			Remark:    s.GetString("remark"),
			Password:  s.GetString("password"),
			LocalPath: s.GetString("local_path"),
			StripPre:  s.GetString("strip_pre"),
			Flow:      &file.Flow{},
		}
		if !tool.TestServerPort(t.Port, t.Mode) {
			s.AjaxErr("The port cannot be opened because it may has been occupied or is no longer allowed.")
		}
		var err error
		if t.Client, err = file.GetCsvDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
			s.AjaxErr(err.Error())
		}
		if err := file.GetCsvDb().NewTask(t); err != nil {
			s.AjaxErr(err.Error())
		}
		if err := server.AddTask(t); err != nil {
			s.AjaxErr(err.Error())
		} else {
			s.AjaxOk("add success")
		}
	}
}
func (s *IndexController) GetOneTunnel() {
	id := s.GetIntNoErr("id")
	data := make(map[string]interface{})
	if t, err := file.GetCsvDb().GetTask(id); err != nil {
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
		if t, err := file.GetCsvDb().GetTask(id); err != nil {
			s.error()
		} else {
			s.Data["t"] = t
		}
		s.SetInfo("edit tunnel")
		s.display()
	} else {
		if t, err := file.GetCsvDb().GetTask(id); err != nil {
			s.error()
		} else {
			t.Port = s.GetIntNoErr("port")
			t.Mode = s.GetString("type")
			t.Target = s.GetString("target")
			t.Password = s.GetString("password")
			t.Id = id
			t.LocalPath = s.GetString("local_path")
			t.StripPre = s.GetString("strip_pre")
			t.Remark = s.GetString("remark")
			if t.Client, err = file.GetCsvDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
				s.AjaxErr("modified error")
			}
			file.GetCsvDb().UpdateTask(t)
			server.StopServer(t.Id)
			server.StartTask(t.Id)
		}
		s.AjaxOk("modified success")
	}
}

func (s *IndexController) Stop() {
	id := s.GetIntNoErr("id")
	if err := server.StopServer(id); err != nil {
		s.AjaxErr("stop error")
	}
	s.AjaxOk("stop success")
}

func (s *IndexController) Del() {
	id := s.GetIntNoErr("id")
	if err := server.DelTask(id); err != nil {
		s.AjaxErr("delete error")
	}
	s.AjaxOk("delete success")
}

func (s *IndexController) Start() {
	id := s.GetIntNoErr("id")
	if err := server.StartTask(id); err != nil {
		s.AjaxErr("start error")
	}
	s.AjaxOk("start success")
}

func (s *IndexController) HostList() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["client_id"] = s.GetString("client_id")
		s.Data["menu"] = "host"
		s.SetInfo("host list")
		s.display("index/hlist")
	} else {
		start, length := s.GetAjaxParams()
		clientId := s.GetIntNoErr("client_id")
		list, cnt := file.GetCsvDb().GetHost(start, length, clientId)
		s.AjaxTable(list, cnt, cnt)
	}
}

func (s *IndexController) GetHost() {
	if s.Ctx.Request.Method == "POST" {
		data := make(map[string]interface{})
		if h, err := file.GetCsvDb().GetHostById(s.GetIntNoErr("id")); err != nil {
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
	id := s.GetIntNoErr("id")
	if err := file.GetCsvDb().DelHost(id); err != nil {
		s.AjaxErr("delete error")
	}
	s.AjaxOk("delete success")
}

func (s *IndexController) AddHost() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["client_id"] = s.GetString("client_id")
		s.Data["menu"] = "host"
		s.SetInfo("add host")
		s.display("index/hadd")
	} else {
		h := &file.Host{
			Id:           file.GetCsvDb().GetHostId(),
			Host:         s.GetString("host"),
			Target:       s.GetString("target"),
			HeaderChange: s.GetString("header"),
			HostChange:   s.GetString("hostchange"),
			Remark:       s.GetString("remark"),
			Location:     s.GetString("location"),
			Flow:         &file.Flow{},
		}
		var err error
		if h.Client, err = file.GetCsvDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
			s.AjaxErr("add error")
		}
		if err := file.GetCsvDb().NewHost(h); err != nil {
			s.AjaxErr("add fail" + err.Error())
		}
		s.AjaxOk("add success")
	}
}

func (s *IndexController) EditHost() {
	id := s.GetIntNoErr("id")
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "host"
		if h, err := file.GetCsvDb().GetHostById(id); err != nil {
			s.error()
		} else {
			s.Data["h"] = h
		}
		s.SetInfo("edit")
		s.display("index/hedit")
	} else {
		if h, err := file.GetCsvDb().GetHostById(id); err != nil {
			s.error()
		} else {
			h.Host = s.GetString("host")
			h.Target = s.GetString("target")
			h.HeaderChange = s.GetString("header")
			h.HostChange = s.GetString("hostchange")
			h.Remark = s.GetString("remark")
			h.TargetArr = nil
			h.Location = s.GetString("location")
			file.GetCsvDb().UpdateHost(h)
			var err error
			if h.Client, err = file.GetCsvDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
				s.AjaxErr("modified error")
			}
		}
		s.AjaxOk("modified success")
	}
}
