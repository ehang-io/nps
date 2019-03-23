package ssdb

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/cnlh/nps/vender/github.com/astaxie/beego/session"
	"github.com/ssdb/gossdb/ssdb"
)

var ssdbProvider = &Provider{}

// Provider holds ssdb client and configs
type Provider struct {
	client      *ssdb.Client
	host        string
	port        int
	maxLifetime int64
}

func (p *Provider) connectInit() error {
	var err error
	if p.host == "" || p.port == 0 {
		return errors.New("SessionInit First")
	}
	p.client, err = ssdb.Connect(p.host, p.port)
	return err
}

// SessionInit init the ssdb with the config
func (p *Provider) SessionInit(maxLifetime int64, savePath string) error {
	p.maxLifetime = maxLifetime
	address := strings.Split(savePath, ":")
	p.host = address[0]

	var err error
	if p.port, err = strconv.Atoi(address[1]); err != nil {
		return err
	}
	return p.connectInit()
}

// SessionRead return a ssdb client session Store
func (p *Provider) SessionRead(sid string) (session.Store, error) {
	if p.client == nil {
		if err := p.connectInit(); err != nil {
			return nil, err
		}
	}
	var kv map[interface{}]interface{}
	value, err := p.client.Get(sid)
	if err != nil {
		return nil, err
	}
	if value == nil || len(value.(string)) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = session.DecodeGob([]byte(value.(string)))
		if err != nil {
			return nil, err
		}
	}
	rs := &SessionStore{sid: sid, values: kv, maxLifetime: p.maxLifetime, client: p.client}
	return rs, nil
}

// SessionExist judged whether sid is exist in session
func (p *Provider) SessionExist(sid string) bool {
	if p.client == nil {
		if err := p.connectInit(); err != nil {
			panic(err)
		}
	}
	value, err := p.client.Get(sid)
	if err != nil {
		panic(err)
	}
	if value == nil || len(value.(string)) == 0 {
		return false
	}
	return true
}

// SessionRegenerate regenerate session with new sid and delete oldsid
func (p *Provider) SessionRegenerate(oldsid, sid string) (session.Store, error) {
	//conn.Do("setx", key, v, ttl)
	if p.client == nil {
		if err := p.connectInit(); err != nil {
			return nil, err
		}
	}
	value, err := p.client.Get(oldsid)
	if err != nil {
		return nil, err
	}
	var kv map[interface{}]interface{}
	if value == nil || len(value.(string)) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = session.DecodeGob([]byte(value.(string)))
		if err != nil {
			return nil, err
		}
		_, err = p.client.Del(oldsid)
		if err != nil {
			return nil, err
		}
	}
	_, e := p.client.Do("setx", sid, value, p.maxLifetime)
	if e != nil {
		return nil, e
	}
	rs := &SessionStore{sid: sid, values: kv, maxLifetime: p.maxLifetime, client: p.client}
	return rs, nil
}

// SessionDestroy destroy the sid
func (p *Provider) SessionDestroy(sid string) error {
	if p.client == nil {
		if err := p.connectInit(); err != nil {
			return err
		}
	}
	_, err := p.client.Del(sid)
	return err
}

// SessionGC not implemented
func (p *Provider) SessionGC() {
}

// SessionAll not implemented
func (p *Provider) SessionAll() int {
	return 0
}

// SessionStore holds the session information which stored in ssdb
type SessionStore struct {
	sid         string
	lock        sync.RWMutex
	values      map[interface{}]interface{}
	maxLifetime int64
	client      *ssdb.Client
}

// Set the key and value
func (s *SessionStore) Set(key, value interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.values[key] = value
	return nil
}

// Get return the value by the key
func (s *SessionStore) Get(key interface{}) interface{} {
	s.lock.Lock()
	defer s.lock.Unlock()
	if value, ok := s.values[key]; ok {
		return value
	}
	return nil
}

// Delete the key in session store
func (s *SessionStore) Delete(key interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.values, key)
	return nil
}

// Flush delete all keys and values
func (s *SessionStore) Flush() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.values = make(map[interface{}]interface{})
	return nil
}

// SessionID return the sessionID
func (s *SessionStore) SessionID() string {
	return s.sid
}

// SessionRelease Store the keyvalues into ssdb
func (s *SessionStore) SessionRelease(w http.ResponseWriter) {
	b, err := session.EncodeGob(s.values)
	if err != nil {
		return
	}
	s.client.Do("setx", s.sid, string(b), s.maxLifetime)
}

func init() {
	session.Register("ssdb", ssdbProvider)
}
