package config

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/file"
	"regexp"
	"strings"
)

type CommonConfig struct {
	Server           string
	VKey             string
	Tp               string //bridgeType kcp or tcp
	AutoReconnection bool
	Cnf              *file.Config
}
type Config struct {
	content      string
	title        []string
	CommonConfig *CommonConfig
	Hosts        []*file.Host
	Tasks        []*file.Tunnel
}

func NewConfig(path string) (c *Config, err error) {
	c = new(Config)
	var b []byte
	if b, err = common.ReadAllFromFile(path); err != nil {
		return
	} else {
		c.content = string(b)
		if c.title, err = getAllTitle(c.content); err != nil {
			return
		}
		var nowIndex int
		var nextIndex int
		var nowContent string
		for i := 0; i < len(c.title); i++ {
			nowIndex = strings.Index(c.content, c.title[i]) + len(c.title[i])
			if i < len(c.title)-1 {
				nextIndex = strings.Index(c.content, c.title[i+1])
			} else {
				nextIndex = len(c.content)
			}
			nowContent = c.content[nowIndex:nextIndex]
			switch c.title[i] {
			case "[common]":
				c.CommonConfig = dealCommon(nowContent)
			default:
				if strings.Index(nowContent, "host") > -1 {
					h := dealHost(nowContent)
					h.Remark = getTitleContent(c.title[i])
					c.Hosts = append(c.Hosts, h)
				} else {
					t := dealTunnel(nowContent)
					t.Remark = getTitleContent(c.title[i])
					c.Tasks = append(c.Tasks, t)
				}
			}
		}

	}
	return
}

func getTitleContent(s string) string {
	re, _ := regexp.Compile(`[\[\]]`)
	return re.ReplaceAllString(s, "")
}
func dealCommon(s string) *CommonConfig {
	c := &CommonConfig{}
	c.Cnf = new(file.Config)
	for _, v := range strings.Split(s, "\n") {
		item := strings.Split(v, "=")
		if len(item) == 0 {
			continue
		} else if len(item) == 1 {
			item = append(item, "")
		}
		switch item[0] {
		case "server":
			c.Server = item[1]
		case "vkey":
			c.VKey = item[1]
		case "tp":
			c.Tp = item[1]
		case "auto_reconnection":
			c.AutoReconnection = common.GetBoolByStr(item[1])
		case "username":
			c.Cnf.U = item[1]
		case "password":
			c.Cnf.P = item[1]
		case "compress":
			c.Cnf.Compress = item[1]
		case "crypt":
			c.Cnf.Crypt = common.GetBoolByStr(item[1])
		}
	}
	return c
}
func dealHost(s string) *file.Host {
	h := &file.Host{}
	var headerChange string
	for _, v := range strings.Split(s, "\n") {
		item := strings.Split(v, "=")
		if len(item) == 0 {
			continue
		} else if len(item) == 1 {
			item = append(item, "")
		}
		switch item[0] {
		case "host":
			h.Host = item[1]
		case "target":
			h.Target = strings.Replace(item[1], ",", "\n", -1)
		case "host_change":
			h.HostChange = item[1]
		default:
			if strings.Contains(item[0], "header") {
				headerChange += strings.Replace(item[0], "header_", "", -1) + ":" + item[1] + "\n"
			}
			h.HeaderChange = headerChange
		}
	}
	return h
}

func dealTunnel(s string) *file.Tunnel {
	t := &file.Tunnel{}
	for _, v := range strings.Split(s, "\n") {
		item := strings.Split(v, "=")
		if len(item) == 0 {
			continue
		} else if len(item) == 1 {
			item = append(item, "")
		}
		switch item[0] {
		case "port":
			t.Port = common.GetIntNoErrByStr(item[1])
		case "mode":
			t.Mode = item[1]
		case "target":
			t.Target = item[1]
		}
	}
	return t

}

func getAllTitle(content string) (arr []string, err error) {
	var re *regexp.Regexp
	re, err = regexp.Compile(`\[.+?\]`)
	if err != nil {
		return
	}
	arr = re.FindAllString(content, -1)
	return
}
