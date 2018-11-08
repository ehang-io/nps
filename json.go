package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
)

//定义配置文件解析后的结构
type Server struct {
	Ip   string
	Port int
	Tcp  int
	Vkey string
	Num  int
}

type Site struct {
	Host string
	Url  string
	Port int
}
type Config struct {
	Server   Server
	SiteList []Site
	Replace  int
}
type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}
func (jst *JsonStruct) Load(filename string) (Config, error) {
	data, err := ioutil.ReadFile(filename)
	config := Config{}
	if err != nil {
		return config, errors.New("配置文件打开错误")
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, errors.New("配置文件解析错误")
	}
	if config.Server.Tcp <= 0 || config.Server.Tcp >= 65536 {
		return config, errors.New("请输入正确的tcp端口")
	}
	if config.Server.Vkey == "" {
		return config, errors.New("密钥不能为空！")
	}
	return config, nil
}
