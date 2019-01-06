package lib

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/astaxie/beego"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

type ServerConfig struct {
	TcpPort        int    //服务端与客户端通信端口
	Mode           string //启动方式
	Target         string //目标
	VerifyKey      string //flag
	U              string //socks5验证用户名
	P              string //socks5验证密码
	Compress       string //压缩方式
	Start          int    //是否开启
	IsRun          int    //是否在运行
	ClientStatus   int    //客s户端状态
	Crypt          bool   //是否加密
	Mux            bool   //是否加密
	CompressEncode int
	CompressDecode int
}

type HostList struct {
	Vkey   string //服务端与客户端通信端口
	Host   string //启动方式
	Target string //目标
}

func NewCsv(bridge *Tunnel, runList map[string]interface{}) *Csv {
	c := new(Csv)
	c.Bridge = bridge
	c.RunList = runList
	return c
}

type Csv struct {
	Tasks   []*ServerConfig
	Path    string
	Bridge  *Tunnel
	RunList map[string]interface{}
	Hosts   []*HostList //域名列表
}

func (s *Csv) Init() {
	s.LoadTaskFromCsv()
	s.LoadHostFromCsv()
}

func (s *Csv) StoreTasksToCsv() {
	// 创建文件
	csvFile, err := os.Create(beego.AppPath + "/conf/tasks.csv")
	if err != nil {
		log.Fatalf(err.Error())
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	for _, task := range s.Tasks {
		record := []string{
			strconv.Itoa(task.TcpPort),
			task.Mode,
			task.Target,
			task.VerifyKey,
			task.U,
			task.P,
			task.Compress,
			strconv.Itoa(task.Start),
			GetStrByBool(task.Crypt),
			GetStrByBool(task.Mux),
			strconv.Itoa(task.CompressEncode),
			strconv.Itoa(task.CompressDecode),
		}
		err := writer.Write(record)
		if err != nil {
			log.Fatalf(err.Error())
		}
	}
	writer.Flush()
}

func (s *Csv) LoadTaskFromCsv() {
	// 打开文件
	file, err := os.Open(beego.AppPath + "/conf/tasks.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 获取csv的reader
	reader := csv.NewReader(file)

	// 设置FieldsPerRecord为-1
	reader.FieldsPerRecord = -1

	// 读取文件中所有行保存到slice中
	records, err := reader.ReadAll()
	if err != nil {
		panic(err)
	}
	var tasks []*ServerConfig
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		tcpPort, _ := strconv.Atoi(item[0])
		Start, _ := strconv.Atoi(item[7])
		post := &ServerConfig{
			TcpPort:        tcpPort,
			Mode:           item[1],
			Target:         item[2],
			VerifyKey:      item[3],
			U:              item[4],
			P:              item[5],
			Compress:       item[6],
			Start:          Start,
			Crypt:          GetBoolByStr(item[8]),
			Mux:            GetBoolByStr(item[9]),
			CompressEncode: GetIntNoerrByStr(item[10]),
			CompressDecode: GetIntNoerrByStr(item[11]),
		}
		tasks = append(tasks, post)
	}
	s.Tasks = tasks
}

func (s *Csv) StoreHostToCsv() {
	// 创建文件
	csvFile, err := os.Create(s.Path + "hosts.csv")
	if err != nil {
		panic(err)
	}
	defer csvFile.Close()
	// 获取csv的Writer
	writer := csv.NewWriter(csvFile)
	// 将map中的Post转换成slice，因为csv的Write需要slice参数
	// 并写入csv文件
	for _, host := range s.Hosts {
		record := []string{
			host.Host,
			host.Target,
			host.Vkey,
		}
		err1 := writer.Write(record)
		if err1 != nil {
			panic(err1)
		}
	}
	// 确保所有内存数据刷到csv文件
	writer.Flush()
}

func (s *Csv) LoadHostFromCsv() {
	// 打开文件
	file, err := os.Open(beego.AppPath + "/conf/hosts.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 获取csv的reader
	reader := csv.NewReader(file)

	// 设置FieldsPerRecord为-1
	reader.FieldsPerRecord = -1

	// 读取文件中所有行保存到slice中
	records, err := reader.ReadAll()
	if err != nil {
		panic(err)
	}
	var hosts []*HostList
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		post := &HostList{
			Vkey:   item[2],
			Host:   item[0],
			Target: item[1],
		}
		hosts = append(hosts, post)
	}
	s.Hosts = hosts
}

func (s *Csv) GetServerConfig(start, length int, typeVal string) ([]*ServerConfig, int) {
	list := make([]*ServerConfig, 0)
	var cnt int
	for _, v := range s.Tasks {
		if v.Mode != typeVal {
			continue
		}
		cnt++
		if start--; start < 0 {
			if length--; length > 0 {
				if _, ok := s.RunList[v.VerifyKey]; ok {
					v.IsRun = 1
				} else {
					v.IsRun = 0
				}
				if s, ok := s.Bridge.signalList[getverifyval(v.VerifyKey)]; ok {
					if s.Len() > 0 {
						v.ClientStatus = 1
					} else {
						v.ClientStatus = 0
					}
				} else {
					v.ClientStatus = 0
				}
				list = append(list, v)
			}
		}

	}
	return list, cnt
}

func (s *Csv) NewTask(t *ServerConfig) {
	s.Tasks = append(s.Tasks, t)
	s.StoreTasksToCsv()
}

func (s *Csv) UpdateTask(t *ServerConfig) error {
	for k, v := range s.Tasks {
		if v.VerifyKey == t.VerifyKey {
			s.Tasks = append(s.Tasks[:k], s.Tasks[k+1:]...)
			s.Tasks = append(s.Tasks, t)
			s.StoreTasksToCsv()
			return nil
		}
	}
	//TODO:待测试
	return errors.New("不存在")
}

func (s *Csv) AddRunList(vKey string, svr interface{}) {
	s.RunList[vKey] = svr
}

func (s *Csv) DelRunList(vKey string) {
	delete(s.RunList, vKey)
}

func (s *Csv) DelTask(vKey string) error {
	for k, v := range s.Tasks {
		if v.VerifyKey == vKey {
			s.Tasks = append(s.Tasks[:k], s.Tasks[k+1:]...)
			s.StoreTasksToCsv()
			return nil
		}
	}
	return errors.New("不存在")
}

func (s *Csv) GetTask(vKey string) (v *ServerConfig, err error) {
	for _, v = range s.Tasks {
		if v.VerifyKey == vKey {
			return
		}
	}
	err = errors.New("未找到")
	return
}

func (s *Csv) DelHost(host string) error {
	for k, v := range s.Hosts {
		if v.Host == host {
			s.Hosts = append(s.Hosts[:k], s.Hosts[k+1:]...)
			s.StoreHostToCsv()
			return nil
		}
	}
	return errors.New("不存在")
}

func (s *Csv) NewHost(t *HostList) {
	s.Hosts = append(s.Hosts, t)
	s.StoreHostToCsv()

}

func (s *Csv) GetHostList(start, length int, vKey string) ([]*HostList, int) {
	list := make([]*HostList, 0)
	var cnt int
	for _, v := range s.Hosts {
		if v.Vkey == vKey {
			cnt++
			if start--; start < 0 {
				if length--; length > 0 {
					list = append(list, v)
				}
			}
		}
	}
	return list, cnt
}

type Site struct {
	Host string
	Url  string
	Port int
}
type Config struct {
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
	return config, nil
}
