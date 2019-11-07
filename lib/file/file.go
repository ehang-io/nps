package file

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/rate"
)

func NewJsonDb(runPath string) *JsonDb {
	return &JsonDb{
		RunPath:        runPath,
		TaskFilePath:   filepath.Join(runPath, "conf", "tasks.json"),
		HostFilePath:   filepath.Join(runPath, "conf", "hosts.json"),
		ClientFilePath: filepath.Join(runPath, "conf", "clients.json"),
	}
}

type JsonDb struct {
	Tasks            sync.Map
	Hosts            sync.Map
	HostsTmp         sync.Map
	Clients          sync.Map
	RunPath          string
	ClientIncreaseId int32  //client increased id
	TaskIncreaseId   int32  //task increased id
	HostIncreaseId   int32  //host increased id
	TaskFilePath     string //task file path
	HostFilePath     string //host file path
	ClientFilePath   string //client file path
}

func (s *JsonDb) LoadTaskFromJsonFile() {
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

func (s *JsonDb) LoadClientFromJsonFile() {
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
		post.NowConn = 0
		s.Clients.Store(post.Id, post)
		if post.Id > int(s.ClientIncreaseId) {
			s.ClientIncreaseId = int32(post.Id)
		}
	})
}

func (s *JsonDb) LoadHostFromJsonFile() {
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

func (s *JsonDb) GetClient(id int) (c *Client, err error) {
	if v, ok := s.Clients.Load(id); ok {
		c = v.(*Client)
		return
	}
	err = errors.New("未找到客户端")
	return
}

func (s *JsonDb) StoreHostToJsonFile() {
	storeSyncMapToFile(s.Hosts, s.HostFilePath)
}

func (s *JsonDb) StoreTasksToJsonFile() {
	storeSyncMapToFile(s.Tasks, s.TaskFilePath)
}

func (s *JsonDb) StoreClientsToJsonFile() {
	storeSyncMapToFile(s.Clients, s.ClientFilePath)
}

func (s *JsonDb) GetClientId() int32 {
	return atomic.AddInt32(&s.ClientIncreaseId, 1)
}

func (s *JsonDb) GetTaskId() int32 {
	return atomic.AddInt32(&s.TaskIncreaseId, 1)
}

func (s *JsonDb) GetHostId() int32 {
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
