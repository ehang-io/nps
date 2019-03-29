package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/rate"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

func NewCsv(runPath string) *Csv {
	return &Csv{
		RunPath:        runPath,
		TaskFilePath:   filepath.Join(runPath, "conf", "tasks.json"),
		HostFilePath:   filepath.Join(runPath, "conf", "hosts.json"),
		ClientFilePath: filepath.Join(runPath, "conf", "clients.json"),
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
	TaskFilePath     string
	HostFilePath     string
	ClientFilePath   string
}

func (s *Csv) LoadTaskFromCsv() {
	loadSyncMapFromFile(s.TaskFilePath, func(v string) {
		var err error
		post := new(Tunnel)
		if json.Unmarshal([]byte(v), &post) != nil {
			return
		}
		if post.Client, err = s.GetClient(post.Client.Id); err != nil {
			return
		}
		s.Tasks.Store(post.Id, post)
		if post.Id > int(s.TaskIncreaseId) {
			s.TaskIncreaseId = int32(post.Id)
		}
	})
}

func (s *Csv) LoadClientFromCsv() {
	loadSyncMapFromFile(s.ClientFilePath, func(v string) {
		post := new(Client)
		if json.Unmarshal([]byte(v), &post) != nil {
			return
		}
		if post.RateLimit > 0 {
			post.Rate = rate.NewRate(int64(post.RateLimit * 1024))
		} else {
			post.Rate = rate.NewRate(int64(2 << 23))
		}
		post.Rate.Start()
		s.Clients.Store(post.Id, post)
		if post.Id > int(s.ClientIncreaseId) {
			s.ClientIncreaseId = int32(post.Id)
		}
	})
}

func (s *Csv) LoadHostFromCsv() {
	loadSyncMapFromFile(s.HostFilePath, func(v string) {
		var err error
		post := new(Host)
		if json.Unmarshal([]byte(v), &post) != nil {
			return
		}
		if post.Client, err = s.GetClient(post.Client.Id); err != nil {
			return
		}
		s.Hosts.Store(post.Id, post)
		if post.Id > int(s.HostIncreaseId) {
			s.HostIncreaseId = int32(post.Id)
		}
	})
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
	storeSyncMapToFile(s.Hosts, s.HostFilePath)
}

func (s *Csv) StoreTasksToCsv() {
	storeSyncMapToFile(s.Tasks, s.TaskFilePath)
}

func (s *Csv) StoreClientsToCsv() {
	storeSyncMapToFile(s.Clients, s.ClientFilePath)
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
	if t.Location == "" {
		t.Location = "/"
	}
	if s.IsHostExist(t) {
		return errors.New("host has exist")
	}
	t.Flow = new(Flow)
	s.Hosts.Store(t.Id, t)
	s.StoreHostToCsv()
	return nil
}

func (s *Csv) GetHost(start, length int, id int, search string) ([]*Host, int) {
	list := make([]*Host, 0)
	var cnt int
	keys := GetMapKeys(s.Hosts, false, "", "")
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
	if c.WebUserName != "" && !s.VerifyUserName(c.WebUserName, c.Id) {
		return errors.New("web login username duplicate, please reset")
	}
reset:
	if c.VerifyKey == "" || isNotSet {
		isNotSet = true
		c.VerifyKey = crypt.GetRandomString(16)
	}
	if c.RateLimit == 0 {
		c.Rate = rate.NewRate(int64(2 << 23))
		c.Rate.Start()
	}
	if !s.VerifyVkey(c.VerifyKey, c.Id) {
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

func (s *Csv) VerifyUserName(username string, id int) (res bool) {
	res = true
	s.Clients.Range(func(key, value interface{}) bool {
		v := value.(*Client)
		if v.WebUserName == username && v.Id != id {
			res = false
			return false
		}
		return true
	})
	return res
}

func (s *Csv) UpdateClient(t *Client) error {
	s.Clients.Store(t.Id, t)
	if t.RateLimit == 0 {
		t.Rate = rate.NewRate(int64(2 << 23))
		t.Rate.Start()
	}
	return nil
}

func (s *Csv) GetClientList(start, length int, search, sort, order string, clientId int) ([]*Client, int) {
	list := make([]*Client, 0)
	var cnt int
	keys := GetMapKeys(s.Clients, true, sort, order)
	for _, key := range keys {
		if value, ok := s.Clients.Load(key); ok {
			v := value.(*Client)
			if v.NoDisplay {
				continue
			}
			if clientId != 0 && clientId != v.Id {
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

func (s *Csv) IsPubClient(id int) bool {
	client, err := s.GetClient(id)
	if err == nil {
		return client.NoDisplay
	}
	return false
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

func (s *Csv) GetClientId() int32 {
	return atomic.AddInt32(&s.ClientIncreaseId, 1)
}

func (s *Csv) GetTaskId() int32 {
	return atomic.AddInt32(&s.TaskIncreaseId, 1)
}

func (s *Csv) GetHostId() int32 {
	return atomic.AddInt32(&s.HostIncreaseId, 1)
}

func loadSyncMapFromFile(filePath string, f func(value string)) {
	b, err := common.ReadAllFromFile(filePath)
	if err != nil {
		panic(err)
	}
	for _, v := range strings.Split(string(b), "\n"+common.CONN_DATA_SEQ) {
		f(v)
	}
}

func storeSyncMapToFile(m sync.Map, filePath string) {
	file, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	m.Range(func(key, value interface{}) bool {
		var b []byte
		var err error
		switch value.(type) {
		case *Tunnel:
			obj := value.(*Tunnel)
			if obj.NoStore {
				return true
			}
			b, err = json.Marshal(obj)
		case *Host:
			obj := value.(*Host)
			if obj.NoStore {
				return true
			}
			b, err = json.Marshal(obj)
		case *Client:
			obj := value.(*Client)
			if obj.NoStore {
				return true
			}
			b, err = json.Marshal(obj)
		default:
			return true
		}
		if err != nil {
			return true
		}
		_, err = file.Write(b)
		if err != nil {
			panic(err)
		}
		_, err = file.Write([]byte("\n" + common.CONN_DATA_SEQ))
		if err != nil {
			panic(err)
		}
		return true
	})
}
