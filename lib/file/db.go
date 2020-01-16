package file

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/rate"
)

type DbUtils struct {
	JsonDb *JsonDb
}

var (
	Db   *DbUtils
	once sync.Once
)

//init csv from file
func GetDb() *DbUtils {
	once.Do(func() {
		jsonDb := NewJsonDb(common.GetRunPath())
		jsonDb.LoadClientFromJsonFile()
		jsonDb.LoadTaskFromJsonFile()
		jsonDb.LoadHostFromJsonFile()
		Db = &DbUtils{JsonDb: jsonDb}
	})
	return Db
}

func GetMapKeys(m sync.Map, isSort bool, sortKey, order string) (keys []int) {
	if sortKey != "" && isSort {
		return sortClientByKey(m, sortKey, order)
	}
	m.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(int))
		return true
	})
	sort.Ints(keys)
	return
}

func (s *DbUtils) GetClientList(start, length int, search, sort, order string, clientId int) ([]*Client, int) {
	list := make([]*Client, 0)
	var cnt int
	keys := GetMapKeys(s.JsonDb.Clients, true, sort, order)
	for _, key := range keys {
		if value, ok := s.JsonDb.Clients.Load(key); ok {
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
				if length--; length >= 0 {
					list = append(list, v)
				}
			}
		}
	}
	return list, cnt
}

func (s *DbUtils) GetIdByVerifyKey(vKey string, addr string) (id int, err error) {
	var exist bool
	s.JsonDb.Clients.Range(func(key, value interface{}) bool {
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

func (s *DbUtils) NewTask(t *Tunnel) (err error) {
	s.JsonDb.Tasks.Range(func(key, value interface{}) bool {
		v := value.(*Tunnel)
		if (v.Mode == "secret" || v.Mode == "p2p") && v.Password == t.Password {
			err = errors.New(fmt.Sprintf("secret mode keys %s must be unique", t.Password))
			return false
		}
		return true
	})
	if err != nil {
		return
	}
	t.Flow = new(Flow)
	s.JsonDb.Tasks.Store(t.Id, t)
	s.JsonDb.StoreTasksToJsonFile()
	return
}

func (s *DbUtils) UpdateTask(t *Tunnel) error {
	s.JsonDb.Tasks.Store(t.Id, t)
	s.JsonDb.StoreTasksToJsonFile()
	return nil
}

func (s *DbUtils) DelTask(id int) error {
	s.JsonDb.Tasks.Delete(id)
	s.JsonDb.StoreTasksToJsonFile()
	return nil
}

//md5 password
func (s *DbUtils) GetTaskByMd5Password(p string) (t *Tunnel) {
	s.JsonDb.Tasks.Range(func(key, value interface{}) bool {
		if crypt.Md5(value.(*Tunnel).Password) == p {
			t = value.(*Tunnel)
			return false
		}
		return true
	})
	return
}

func (s *DbUtils) GetTask(id int) (t *Tunnel, err error) {
	if v, ok := s.JsonDb.Tasks.Load(id); ok {
		t = v.(*Tunnel)
		return
	}
	err = errors.New("not found")
	return
}

func (s *DbUtils) DelHost(id int) error {
	s.JsonDb.Hosts.Delete(id)
	s.JsonDb.StoreHostToJsonFile()
	return nil
}

func (s *DbUtils) IsHostExist(h *Host) bool {
	var exist bool
	s.JsonDb.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*Host)
		if v.Id != h.Id && v.Host == h.Host && h.Location == v.Location && (v.Scheme == "all" || v.Scheme == h.Scheme) {
			exist = true
			return false
		}
		return true
	})
	return exist
}

func (s *DbUtils) NewHost(t *Host) error {
	if t.Location == "" {
		t.Location = "/"
	}
	if s.IsHostExist(t) {
		return errors.New("host has exist")
	}
	t.Flow = new(Flow)
	s.JsonDb.Hosts.Store(t.Id, t)
	s.JsonDb.StoreHostToJsonFile()
	return nil
}

func (s *DbUtils) GetHost(start, length int, id int, search string) ([]*Host, int) {
	list := make([]*Host, 0)
	var cnt int
	keys := GetMapKeys(s.JsonDb.Hosts, false, "", "")
	for _, key := range keys {
		if value, ok := s.JsonDb.Hosts.Load(key); ok {
			v := value.(*Host)
			if search != "" && !(v.Id == common.GetIntNoErrByStr(search) || strings.Contains(v.Host, search) || strings.Contains(v.Remark, search)) {
				continue
			}
			if id == 0 || v.Client.Id == id {
				cnt++
				if start--; start < 0 {
					if length--; length >= 0 {
						list = append(list, v)
					}
				}
			}
		}
	}
	return list, cnt
}

func (s *DbUtils) DelClient(id int) error {
	s.JsonDb.Clients.Delete(id)
	s.JsonDb.StoreClientsToJsonFile()
	return nil
}

func (s *DbUtils) NewClient(c *Client) error {
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
	} else if c.Rate == nil {
		c.Rate = rate.NewRate(int64(c.RateLimit * 1024))
	}
	c.Rate.Start()
	if !s.VerifyVkey(c.VerifyKey, c.Id) {
		if isNotSet {
			goto reset
		}
		return errors.New("Vkey duplicate, please reset")
	}
	if c.Id == 0 {
		c.Id = int(s.JsonDb.GetClientId())
	}
	if c.Flow == nil {
		c.Flow = new(Flow)
	}
	s.JsonDb.Clients.Store(c.Id, c)
	s.JsonDb.StoreClientsToJsonFile()
	return nil
}

func (s *DbUtils) VerifyVkey(vkey string, id int) (res bool) {
	res = true
	s.JsonDb.Clients.Range(func(key, value interface{}) bool {
		v := value.(*Client)
		if v.VerifyKey == vkey && v.Id != id {
			res = false
			return false
		}
		return true
	})
	return res
}

func (s *DbUtils) VerifyUserName(username string, id int) (res bool) {
	res = true
	s.JsonDb.Clients.Range(func(key, value interface{}) bool {
		v := value.(*Client)
		if v.WebUserName == username && v.Id != id {
			res = false
			return false
		}
		return true
	})
	return res
}

func (s *DbUtils) UpdateClient(t *Client) error {
	s.JsonDb.Clients.Store(t.Id, t)
	if t.RateLimit == 0 {
		t.Rate = rate.NewRate(int64(2 << 23))
		t.Rate.Start()
	}
	return nil
}

func (s *DbUtils) IsPubClient(id int) bool {
	client, err := s.GetClient(id)
	if err == nil {
		return client.NoDisplay
	}
	return false
}

func (s *DbUtils) GetClient(id int) (c *Client, err error) {
	if v, ok := s.JsonDb.Clients.Load(id); ok {
		c = v.(*Client)
		return
	}
	err = errors.New("未找到客户端")
	return
}

func (s *DbUtils) GetClientIdByVkey(vkey string) (id int, err error) {
	var exist bool
	s.JsonDb.Clients.Range(func(key, value interface{}) bool {
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

func (s *DbUtils) GetHostById(id int) (h *Host, err error) {
	if v, ok := s.JsonDb.Hosts.Load(id); ok {
		h = v.(*Host)
		return
	}
	err = errors.New("The host could not be parsed")
	return
}

//get key by host from x
func (s *DbUtils) GetInfoByHost(host string, r *http.Request) (h *Host, err error) {
	var hosts []*Host
	//Handling Ported Access
	host = common.GetIpByAddr(host)
	s.JsonDb.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*Host)
		if v.IsClose {
			return true
		}
		//Remove http(s) http(s)://a.proxy.com
		//*.proxy.com *.a.proxy.com  Do some pan-parsing
		if v.Scheme != "all" && v.Scheme != r.URL.Scheme {
			return true
		}
		tmpHost := v.Host
		if strings.Contains(tmpHost, "*") {
			tmpHost = strings.Replace(tmpHost, "*", "", -1)
			if strings.Contains(host, tmpHost) {
				hosts = append(hosts, v)
			}
		} else if v.Host == host {
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
