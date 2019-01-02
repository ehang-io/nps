package controllers

import (
	"github.com/cnlh/easyProxy/lib"
)

type IndexController struct {
	BaseController
}

func (s *IndexController) Index() {
	s.SetInfo("使用说明")
	s.display("index/index")
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
	s.SetType("sock5Server")
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

func (s *IndexController) GetTaskList() {
	start, length := s.GetAjaxParams()
	taskType := s.GetString("type")
	list, cnt := lib.CsvDb.GetTaskList(start, length, taskType)
	s.AjaxTable(list, cnt, cnt)
}

func (s *IndexController) Add() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["type"] = s.GetString("type")
		s.SetInfo("新增")
		s.display()
	} else {
		t := &lib.TaskList{
			TcpPort:   s.GetIntNoErr("port"),
			Mode:      s.GetString("type"),
			Target:    s.GetString("target"),
			VerifyKey: lib.GetRandomString(16),
			U:         s.GetString("u"),
			P:         s.GetString("p"),
			Compress:  s.GetString("compress"),
			Crypt:     s.GetString("crypt"),
			IsRun:     0,
		}
		lib.CsvDb.NewTask(t)
		if err := lib.AddTask(t); err != nil {
			s.AjaxErr(err.Error())
		} else {
			s.AjaxOk("添加成功")
		}
	}
}

func (s *IndexController) Edit() {
	if s.Ctx.Request.Method == "GET" {
		vKey := s.GetString("vKey")
		if t, err := lib.CsvDb.GetTask(vKey); err != nil {
			s.error()
		} else {
			s.Data["t"] = t
		}
		s.SetInfo("修改")
		s.display()
	} else {
		vKey := s.GetString("vKey")
		if t, err := lib.CsvDb.GetTask(vKey); err != nil {
			s.error()
		} else {
			t.TcpPort = s.GetIntNoErr("port")
			t.Mode = s.GetString("type")
			t.Target = s.GetString("target")
			t.U = s.GetString("u")
			t.P = s.GetString("p")
			t.Compress = s.GetString("compress")
			t.Crypt = s.GetString("crypt")
			lib.CsvDb.UpdateTask(t)
			lib.StopServer(t.VerifyKey)
			lib.StartTask(t.VerifyKey)
		}
		s.AjaxOk("修改成功")
	}
}

func (s *IndexController) Stop() {
	vKey := s.GetString("vKey")
	if err := lib.StopServer(vKey); err != nil {
		s.AjaxErr("停止失败")
	}
	s.AjaxOk("停止成功")
}
func (s *IndexController) Del() {
	vKey := s.GetString("vKey")
	if err := lib.DelTask(vKey); err != nil {
		s.AjaxErr("删除失败")
	}
	s.AjaxOk("删除成功")
}

func (s *IndexController) Start() {
	vKey := s.GetString("vKey")
	if err := lib.StartTask(vKey); err != nil {
		s.AjaxErr("开启失败")
	}
	s.AjaxOk("开启成功")
}

func (s *IndexController) HostList() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["vkey"] = s.GetString("vkey")
		s.SetInfo("域名列表")
		s.display("index/hlist")
	} else {
		start, length := s.GetAjaxParams()
		vkey := s.GetString("vkey")
		list, cnt := lib.CsvDb.GetHostList(start, length, vkey)
		s.AjaxTable(list, cnt, cnt)
	}
}

func (s *IndexController) DelHost() {
	host := s.GetString("host")
	if err := lib.CsvDb.DelHost(host); err != nil {
		s.AjaxErr("删除失败")
	}
	s.AjaxOk("删除成功")
}

func (s *IndexController) AddHost() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["vkey"] = s.GetString("vkey")
		s.SetInfo("新增")
		s.display("index/hadd")
	} else {
		h := &lib.HostList{
			Vkey:   s.GetString("vkey"),
			Host:   s.GetString("host"),
			Target: s.GetString("target"),
		}
		lib.CsvDb.NewHost(h)
		s.AjaxOk("添加成功")
	}
}
