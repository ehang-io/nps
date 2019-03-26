package file

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/rate"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

func NewCsv(runPath string) *Csv {
	return &Csv{
		RunPath: runPath,
	}
}

type Csv struct {
	Tasks            sync.Map
	Hosts            sync.Map //域名列表
	HostsTmp         sync.Map
	Clients          sync.Map //客户端
	RunPath          string   //存储根目录
	ClientIncreaseId int32    //客户端id
	TaskIncreaseId   int32    //任务自增ID
	HostIncreaseId   int32    //host increased id
}

func (s *Csv) StoreTasksToCsv() {
	// 创建文件
	csvFile, err := os.Create(filepath.Join(s.RunPath, "conf", "tasks.csv"))
	if err != nil {
		logs.Error(err.Error())
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	s.Tasks.Range(func(key, value interface{}) bool {
		task := value.(*Tunnel)
		if task.NoStore {
			return true
		}
		record := []string{
			strconv.Itoa(task.Port),
			task.Mode,
			task.Target,
			common.GetStrByBool(task.Status),
			strconv.Itoa(task.Id),
			strconv.Itoa(task.Client.Id),
			task.Remark,
			strconv.Itoa(int(task.Flow.ExportFlow)),
			strconv.Itoa(int(task.Flow.InletFlow)),
			task.Password,
		}
		err := writer.Write(record)
		if err != nil {
			logs.Error(err.Error())
		}
		return true
	})
	writer.Flush()
}

func (s *Csv) openFile(path string) ([][]string, error) {
	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 获取csv的reader
	reader := csv.NewReader(file)

	// 设置FieldsPerRecord为-1
	reader.FieldsPerRecord = -1

	// 读取文件中所有行保存到slice中
	return reader.ReadAll()
}

func (s *Csv) LoadTaskFromCsv() {
	path := filepath.Join(s.RunPath, "conf", "tasks.csv")
	records, err := s.openFile(path)
	if err != nil {
		logs.Error("Profile Opening Error:", path)
		os.Exit(0)
	}
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		post := &Tunnel{
			Port:     common.GetIntNoErrByStr(item[0]),
			Mode:     item[1],
			Target:   item[2],
			Status:   common.GetBoolByStr(item[3]),
			Id:       common.GetIntNoErrByStr(item[4]),
			Remark:   item[6],
			Password: item[9],
		}
		post.Flow = new(Flow)
		post.Flow.ExportFlow = int64(common.GetIntNoErrByStr(item[7]))
		post.Flow.InletFlow = int64(common.GetIntNoErrByStr(item[8]))
		if post.Client, err = s.GetClient(common.GetIntNoErrByStr(item[5])); err != nil {
			continue
		}
		s.Tasks.Store(post.Id, post)
		if post.Id > int(s.TaskIncreaseId) {
			s.TaskIncreaseId = int32(s.TaskIncreaseId)
		}
	}
}

func (s *Csv) GetIdByVerifyKey(vKey string, addr string) (id int, err error) {
	var exist bool
	s.Clients.Range(func(key, value interface{}) bool {
		v := value.(*Client)
		if common.Getverifyval(v.VerifyKey) == vKey && v.Status {
			v.Addr = common.GetIpByAddr(addr)
			id = v.Id
			exist = true
			return false
		}
		return true
	})
	if exist {
		return
	}
	return 0, errors.New("not found")
}

func (s *Csv) NewTask(t *Tunnel) (err error) {
	s.Tasks.Range(func(key, value interface{}) bool {
		v := value.(*Tunnel)
		if (v.Mode == "secret" || v.Mode == "p2p") && v.Password == t.Password {
			err = errors.New(fmt.Sprintf("Secret mode keys %s must be unique", t.Password))
			return false
		}
		return true
	})
	if err != nil {
		return
	}
	t.Flow = new(Flow)
	s.Tasks.Store(t.Id, t)
	s.StoreTasksToCsv()
	return
}

func (s *Csv) UpdateTask(t *Tunnel) error {
	s.Tasks.Store(t.Id, t)
	s.StoreTasksToCsv()
	return nil
}

func (s *Csv) DelTask(id int) error {
	s.Tasks.Delete(id)
	s.StoreTasksToCsv()
	return nil
}

//md5 password
func (s *Csv) GetTaskByMd5Password(p string) (t *Tunnel) {
	s.Tasks.Range(func(key, value interface{}) bool {
		if crypt.Md5(value.(*Tunnel).Password) == p {
			t = value.(*Tunnel)
			return false
		}
		return true
	})
	return
}

func (s *Csv) GetTask(id int) (t *Tunnel, err error) {
	if v, ok := s.Tasks.Load(id); ok {
		t = v.(*Tunnel)
		return
	}
	err = errors.New("not found")
	return
}

func (s *Csv) StoreHostToCsv() {
	// 创建文件
	csvFile, err := os.Create(filepath.Join(s.RunPath, "conf", "hosts.csv"))
	if err != nil {
		panic(err)
	}
	defer csvFile.Close()
	// 获取csv的Writer
	writer := csv.NewWriter(csvFile)
	// 将map中的Post转换成slice，因为csv的Write需要slice参数
	// 并写入csv文件
	s.Hosts.Range(func(key, value interface{}) bool {
		host := value.(*Host)
		if host.NoStore {
			return true
		}
		record := []string{
			host.Host,
			host.Target,
			strconv.Itoa(host.Client.Id),
			host.HeaderChange,
			host.HostChange,
			host.Remark,
			host.Location,
			strconv.Itoa(host.Id),
			strconv.Itoa(int(host.Flow.ExportFlow)),
			strconv.Itoa(int(host.Flow.InletFlow)),
			host.Scheme,
		}
		err1 := writer.Write(record)
		if err1 != nil {
			panic(err1)
		}
		return true
	})

	// 确保所有内存数据刷到csv文件
	writer.Flush()
}

func (s *Csv) LoadClientFromCsv() {
	path := filepath.Join(s.RunPath, "conf", "clients.csv")
	records, err := s.openFile(path)
	if err != nil {
		logs.Error("Profile Opening Error:", path)
		os.Exit(0)
	}
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		post := &Client{
			Id:        common.GetIntNoErrByStr(item[0]),
			VerifyKey: item[1],
			Remark:    item[2],
			Status:    common.GetBoolByStr(item[3]),
			RateLimit: common.GetIntNoErrByStr(item[8]),
			Cnf: &Config{
				U:        item[4],
				P:        item[5],
				Crypt:    common.GetBoolByStr(item[6]),
				Compress: common.GetBoolByStr(item[7]),
			},
			MaxConn: common.GetIntNoErrByStr(item[10]),
		}
		if post.Id > int(s.ClientIncreaseId) {
			s.ClientIncreaseId = int32(post.Id)
		}
		if post.RateLimit > 0 {
			post.Rate = rate.NewRate(int64(post.RateLimit * 1024))
			post.Rate.Start()
		} else {
			post.Rate = rate.NewRate(int64(2 << 23))
			post.Rate.Start()
		}
		post.Flow = new(Flow)
		post.Flow.FlowLimit = int64(common.GetIntNoErrByStr(item[9]))
		if len(item) >= 12 {
			post.ConfigConnAllow = common.GetBoolByStr(item[11])
		} else {
			post.ConfigConnAllow = true
		}
		s.Clients.Store(post.Id, post)
	}
}

func (s *Csv) LoadHostFromCsv() {
	path := filepath.Join(s.RunPath, "conf", "hosts.csv")
	records, err := s.openFile(path)
	if err != nil {
		logs.Error("Profile Opening Error:", path)
		os.Exit(0)
	}
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		post := &Host{
			Host:         item[0],
			Target:       item[1],
			HeaderChange: item[3],
			HostChange:   item[4],
			Remark:       item[5],
			Location:     item[6],
			Id:           common.GetIntNoErrByStr(item[7]),
		}
		if post.Client, err = s.GetClient(common.GetIntNoErrByStr(item[2])); err != nil {
			continue
		}
		post.Flow = new(Flow)
		post.Flow.ExportFlow = int64(common.GetIntNoErrByStr(item[8]))
		post.Flow.InletFlow = int64(common.GetIntNoErrByStr(item[9]))
		if len(item) > 10 {
			post.Scheme = item[10]
		} else {
			post.Scheme = "all"
		}
		s.Hosts.Store(post.Id, post)
		if post.Id > int(s.HostIncreaseId) {
			s.HostIncreaseId = int32(post.Id)
		}
		//store host to hostMap if the host url is none
	}
}

func (s *Csv) DelHost(id int) error {
	s.Hosts.Delete(id)
	s.StoreHostToCsv()
	return nil
}

func (s *Csv) GetMapLen(m sync.Map) int {
	var c int
	m.Range(func(key, value interface{}) bool {
		c++
		return true
	})
	return c
}

func (s *Csv) IsHostExist(h *Host) bool {
	var exist bool
	s.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*Host)
		if v.Host == h.Host && h.Location == v.Location && (v.Scheme == "all" || v.Scheme == h.Scheme) {
			exist = true
			return false
		}
		return true
	})
	return exist
}

func (s *Csv) NewHost(t *Host) error {
	if s.IsHostExist(t) {
		return errors.New("host has exist")
	}
	if t.Location == "" {
		t.Location = "/"
	}
	t.Flow = new(Flow)
	s.Hosts.Store(t.Id, t)
	s.StoreHostToCsv()
	return nil
}

func (s *Csv) GetHost(start, length int, id int, search string) ([]*Host, int) {
	list := make([]*Host, 0)
	var cnt int
	keys := common.GetMapKeys(s.Hosts)
	for _, key := range keys {
		if value, ok := s.Hosts.Load(key); ok {
			v := value.(*Host)
			if search != "" && !(v.Id == common.GetIntNoErrByStr(search) || strings.Contains(v.Host, search) || strings.Contains(v.Remark, search)) {
				continue
			}
			if id == 0 || v.Client.Id == id {
				cnt++
				if start--; start < 0 {
					if length--; length > 0 {
						list = append(list, v)
					}
				}
			}
		}
	}
	return list, cnt
}

func (s *Csv) DelClient(id int) error {
	s.Clients.Delete(id)
	s.StoreClientsToCsv()
	return nil
}

func (s *Csv) NewClient(c *Client) error {
	var isNotSet bool
reset:
	if c.VerifyKey == "" || isNotSet {
		isNotSet = true
		c.VerifyKey = crypt.GetRandomString(16)
	}
	if c.RateLimit == 0 {
		c.Rate = rate.NewRate(int64(2 << 23))
		c.Rate.Start()
	}
	if !s.VerifyVkey(c.VerifyKey, c.id) {
		if isNotSet {
			goto reset
		}
		return errors.New("Vkey duplicate, please reset")
	}
	if c.Id == 0 {
		c.Id = int(s.GetClientId())
	}
	if c.Flow == nil {
		c.Flow = new(Flow)
	}
	s.Clients.Store(c.Id, c)
	s.StoreClientsToCsv()
	return nil
}

func (s *Csv) VerifyVkey(vkey string, id int) (res bool) {
	res = true
	s.Clients.Range(func(key, value interface{}) bool {
		v := value.(*Client)
		if v.VerifyKey == vkey && v.Id != id {
			res = false
			return false
		}
		return true
	})
	return res
}

func (s *Csv) GetClientId() int32 {
	return atomic.AddInt32(&s.ClientIncreaseId, 1)
}

func (s *Csv) GetTaskId() int32 {
	return atomic.AddInt32(&s.TaskIncreaseId, 1)
}

func (s *Csv) GetHostId() int32 {
	return atomic.AddInt32(&s.HostIncreaseId, 1)
}

func (s *Csv) UpdateClient(t *Client) error {
	s.Clients.Store(t.Id, t)
	if t.RateLimit == 0 {
		t.Rate = rate.NewRate(int64(2 << 23))
		t.Rate.Start()
	}
	return nil
}

func (s *Csv) GetClientList(start, length int, search string) ([]*Client, int) {
	list := make([]*Client, 0)
	var cnt int
	keys := common.GetMapKeys(s.Clients)
	for _, key := range keys {
		if value, ok := s.Clients.Load(key); ok {
			v := value.(*Client)
			if v.NoDisplay {
				continue
			}
			if search != "" && !(v.Id == common.GetIntNoErrByStr(search) || strings.Contains(v.VerifyKey, search) || strings.Contains(v.Remark, search)) {
				continue
			}
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

func (s *Csv) GetClient(id int) (c *Client, err error) {
	if v, ok := s.Clients.Load(id); ok {
		c = v.(*Client)
		return
	}
	err = errors.New("未找到客户端")
	return
}
func (s *Csv) GetClientIdByVkey(vkey string) (id int, err error) {
	var exist bool
	s.Clients.Range(func(key, value interface{}) bool {
		v := value.(*Client)
		if crypt.Md5(v.VerifyKey) == vkey {
			exist = true
			id = v.Id
			return false
		}
		return true
	})
	if exist {
		return
	}
	err = errors.New("未找到客户端")
	return
}

func (s *Csv) GetHostById(id int) (h *Host, err error) {
	if v, ok := s.Hosts.Load(id); ok {
		h = v.(*Host)
		return
	}
	err = errors.New("The host could not be parsed")
	return
}

//get key by host from x
func (s *Csv) GetInfoByHost(host string, r *http.Request) (h *Host, err error) {
	var hosts []*Host
	//Handling Ported Access
	host = common.GetIpByAddr(host)
	s.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*Host)
		if v.IsClose {
			return true
		}
		//Remove http(s) http(s)://a.proxy.com
		//*.proxy.com *.a.proxy.com  Do some pan-parsing
		tmp := strings.Replace(v.Host, "*", `\w+?`, -1)
		var re *regexp.Regexp
		if re, err = regexp.Compile(tmp); err != nil {
			return true
		}
		if len(re.FindAllString(host, -1)) > 0 && (v.Scheme == "all" || v.Scheme == r.URL.Scheme) {
			//URL routing
			hosts = append(hosts, v)
		}
		return true
	})

	for _, v := range hosts {
		//If not set, default matches all
		if v.Location == "" {
			v.Location = "/"
		}
		if strings.Index(r.RequestURI, v.Location) == 0 {
			if h == nil || (len(v.Location) > len(h.Location)) {
				h = v
			}
		}
	}
	if h != nil {
		return
	}
	err = errors.New("The host could not be parsed")
	return
}

func (s *Csv) StoreClientsToCsv() {
	// 创建文件
	csvFile, err := os.Create(filepath.Join(s.RunPath, "conf", "clients.csv"))
	if err != nil {
		logs.Error(err.Error())
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	s.Clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.NoStore {
			return true
		}
		record := []string{
			strconv.Itoa(client.Id),
			client.VerifyKey,
			client.Remark,
			strconv.FormatBool(client.Status),
			client.Cnf.U,
			client.Cnf.P,
			common.GetStrByBool(client.Cnf.Crypt),
			strconv.FormatBool(client.Cnf.Compress),
			strconv.Itoa(client.RateLimit),
			strconv.Itoa(int(client.Flow.FlowLimit)),
			strconv.Itoa(int(client.MaxConn)),
			common.GetStrByBool(client.ConfigConnAllow),
		}
		err := writer.Write(record)
		if err != nil {
			logs.Error(err.Error())
		}
		return true
	})
	writer.Flush()
}
