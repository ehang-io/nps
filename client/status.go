package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/lg"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func GetTaskStatus(path string) {
	cnf, err := config.NewConfig(path)
	if err != nil {
		log.Fatalln(err)
	}
	c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
	if err != nil {
		log.Fatalln(err)
	}
	if _, err := c.Write([]byte(common.WORK_STATUS)); err != nil {
		log.Fatalln(err)
	}
	if f, err := common.ReadAllFromFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt")); err != nil {
		log.Fatalln(err)
	} else if _, err := c.Write([]byte(string(f))); err != nil {
		log.Fatalln(err)
	}
	if l, err := c.GetLen(); err != nil {
		log.Fatalln(err)
	} else if b, err := c.ReadLen(l); err != nil {
		lg.Fatalln(err)
	} else {
		arr := strings.Split(string(b), common.CONN_DATA_SEQ)
		for _, v := range cnf.Hosts {
			if common.InArr(arr, v.Remark) {
				log.Println(v.Remark, "ok")
			} else {
				log.Println(v.Remark, "not running")
			}
		}
		for _, v := range cnf.Tasks {
			if common.InArr(arr, v.Remark) {
				log.Println(v.Remark, "ok")
			} else {
				log.Println(v.Remark, "not running")
			}
		}
	}
	os.Exit(0)
}
