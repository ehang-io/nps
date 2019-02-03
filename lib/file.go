package lib

import (
	"encoding/csv"
	"errors"
	"github.com/astaxie/beego"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	CsvDb *Csv
	once  sync.Once
)

func NewCsv() *Csv {
	return new(Csv)
}

type Csv struct {
	Tasks            []*Tunnel
	Path             string
	Hosts            []*Host   //域名列表
	Clients          []*Client //客户端
	ClientIncreaseId int       //客户端id
	TaskIncreaseId   int       //任务自增ID
	sync.Mutex
}

func (s *Csv) Init() {
	s.LoadClientFromCsv()
	s.LoadTaskFromCsv()
	s.LoadHostFromCsv()
}

func (s *Csv) StoreTasksToCsv() {
	// 创建文件
	csvFile, err := os.Create(beego.AppPath + "/conf/tasks.csv")
	if err != nil {
		Fatalf(err.Error())
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	for _, task := range s.Tasks {
		record := []string{
			strconv.Itoa(task.TcpPort),
			task.Mode,
			task.Target,
			task.Config.U,
			task.Config.P,
			task.Config.Compress,
			GetStrByBool(task.Status),
			GetStrByBool(task.Config.Crypt),
			strconv.Itoa(task.Config.CompressEncode),
			strconv.Itoa(task.Config.CompressDecode),
			strconv.Itoa(task.Id),
			strconv.Itoa(task.Client.Id),
			strconv.FormatBool(task.UseClientCnf),
			task.Remark,
		}
		err := writer.Write(record)
		if err != nil {
			Fatalf(err.Error())
		}
	}
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
	path := beego.AppPath + "/conf/tasks.csv"
	records, err := s.openFile(path)
	if err != nil {
		Fatalln("配置文件打开错误:", path)
	}
	var tasks []*Tunnel
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		post := &Tunnel{
			TcpPort: GetIntNoErrByStr(item[0]),
			Mode:    item[1],
			Target:  item[2],
			Config: &Config{
				U:              item[3],
				P:              item[4],
				Compress:       item[5],
				Crypt:          GetBoolByStr(item[7]),
				CompressEncode: GetIntNoErrByStr(item[8]),
				CompressDecode: GetIntNoErrByStr(item[9]),
			},
			Status:       GetBoolByStr(item[6]),
			Id:           GetIntNoErrByStr(item[10]),
			UseClientCnf: GetBoolByStr(item[12]),
			Remark:       item[13],
		}
		post.Flow = new(Flow)
		if post.Client, err = s.GetClient(GetIntNoErrByStr(item[11])); err != nil {
			continue
		}
		tasks = append(tasks, post)
		if post.Id > s.TaskIncreaseId {
			s.TaskIncreaseId = post.Id
		}
	}
	s.Tasks = tasks
}

func (s *Csv) GetTaskId() int {
	s.Lock()
	defer s.Unlock()
	s.TaskIncreaseId++
	return s.TaskIncreaseId
}

func (s *Csv) GetIdByVerifyKey(vKey string, addr string) (int, error) {
	s.Lock()
	defer s.Unlock()
	for _, v := range s.Clients {
		if Getverifyval(v.VerifyKey) == vKey && v.Status {
			if arr := strings.Split(addr, ":"); len(arr) > 0 {
				v.Addr = arr[0]
			}
			return v.Id, nil
		}
	}
	return 0, errors.New("not found")
}

func (s *Csv) NewTask(t *Tunnel) {
	t.Flow = new(Flow)
	s.Tasks = append(s.Tasks, t)
	s.StoreTasksToCsv()
}

func (s *Csv) UpdateTask(t *Tunnel) error {
	for k, v := range s.Tasks {
		if v.Id == t.Id {
			s.Tasks = append(s.Tasks[:k], s.Tasks[k+1:]...)
			s.Tasks = append(s.Tasks, t)
			s.StoreTasksToCsv()
			return nil
		}
	}
	return errors.New("不存在")
}

func (s *Csv) DelTask(id int) error {
	for k, v := range s.Tasks {
		if v.Id == id {
			s.Tasks = append(s.Tasks[:k], s.Tasks[k+1:]...)
			s.StoreTasksToCsv()
			return nil
		}
	}
	return errors.New("不存在")
}

func (s *Csv) GetTask(id int) (v *Tunnel, err error) {
	for _, v = range s.Tasks {
		if v.Id == id {
			return
		}
	}
	err = errors.New("未找到")
	return
}

func (s *Csv) StoreHostToCsv() {
	// 创建文件
	csvFile, err := os.Create(beego.AppPath + "/conf/hosts.csv")
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
			strconv.Itoa(host.Client.Id),
			host.HeaderChange,
			host.HostChange,
			host.Remark,
		}
		err1 := writer.Write(record)
		if err1 != nil {
			panic(err1)
		}
	}
	// 确保所有内存数据刷到csv文件
	writer.Flush()
}

func (s *Csv) LoadClientFromCsv() {
	path := beego.AppPath + "/conf/clients.csv"
	records, err := s.openFile(path)
	if err != nil {
		Fatalln("配置文件打开错误:", path)
	}
	var clients []*Client
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		post := &Client{
			Id:        GetIntNoErrByStr(item[0]),
			VerifyKey: item[1],
			Remark:    item[2],
			Status:    GetBoolByStr(item[3]),
			RateLimit: GetIntNoErrByStr(item[8]),
			Cnf: &Config{
				U:        item[4],
				P:        item[5],
				Crypt:    GetBoolByStr(item[6]),
				Compress: item[7],
			},
		}
		if post.Id > s.ClientIncreaseId {
			s.ClientIncreaseId = post.Id
		}
		if post.RateLimit > 0 {
			post.Rate = NewRate(int64(post.RateLimit * 1024))
			post.Rate.Start()
		}
		post.Flow = new(Flow)
		post.Flow.FlowLimit = int64(GetIntNoErrByStr(item[9]))
		clients = append(clients, post)
	}
	s.Clients = clients
}

func (s *Csv) LoadHostFromCsv() {
	path := beego.AppPath + "/conf/hosts.csv"
	records, err := s.openFile(path)
	if err != nil {
		Fatalln("配置文件打开错误:", path)
	}
	var hosts []*Host
	// 将每一行数据保存到内存slice中
	for _, item := range records {
		post := &Host{
			Host:         item[0],
			Target:       item[1],
			HeaderChange: item[3],
			HostChange:   item[4],
			Remark:       item[5],
		}
		if post.Client, err = s.GetClient(GetIntNoErrByStr(item[2])); err != nil {
			continue
		}
		post.Flow = new(Flow)
		hosts = append(hosts, post)
	}
	s.Hosts = hosts
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

func (s *Csv) NewHost(t *Host) {
	t.Flow = new(Flow)
	s.Hosts = append(s.Hosts, t)
	s.StoreHostToCsv()

}

func (s *Csv) UpdateHost(t *Host) error {
	for k, v := range s.Hosts {
		if v.Host == t.Host {
			s.Hosts = append(s.Hosts[:k], s.Hosts[k+1:]...)
			s.Hosts = append(s.Hosts, t)
			s.StoreHostToCsv()
			return nil
		}
	}
	return errors.New("不存在")
}

func (s *Csv) GetHost(start, length int, id int) ([]*Host, int) {
	list := make([]*Host, 0)
	var cnt int
	for _, v := range s.Hosts {
		if id == 0 || v.Client.Id == id {
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

func (s *Csv) DelClient(id int) error {
	for k, v := range s.Clients {
		if v.Id == id {
			s.Clients = append(s.Clients[:k], s.Clients[k+1:]...)
			s.StoreClientsToCsv()
			return nil
		}
	}
	return errors.New("不存在")
}

func (s *Csv) NewClient(c *Client) {
	s.Lock()
	defer s.Unlock()
	c.Flow = new(Flow)
	s.Clients = append(s.Clients, c)
	s.StoreClientsToCsv()
}

func (s *Csv) GetClientId() int {
	s.Lock()
	defer s.Unlock()
	s.ClientIncreaseId++
	return s.ClientIncreaseId
}

func (s *Csv) UpdateClient(t *Client) error {
	s.Lock()
	defer s.Unlock()
	for _, v := range s.Clients {
		if v.Id == t.Id {
			v.Cnf = t.Cnf
			v.VerifyKey = t.VerifyKey
			v.Remark = t.Remark
			v.RateLimit = t.RateLimit
			v.Flow = t.Flow
			v.Rate = t.Rate
			s.StoreClientsToCsv()
			return nil
		}
	}
	return errors.New("不存在")
}

func (s *Csv) GetClientList(start, length int) ([]*Client, int) {
	list := make([]*Client, 0)
	var cnt int
	for _, v := range s.Clients {
		cnt++
		if start--; start < 0 {
			if length--; length > 0 {
				list = append(list, v)
			}
		}
	}
	return list, cnt
}

func (s *Csv) GetClient(id int) (v *Client, err error) {
	for _, v = range s.Clients {
		if v.Id == id {
			return
		}
	}
	err = errors.New("未找到")
	return
}
func (s *Csv) StoreClientsToCsv() {
	// 创建文件
	csvFile, err := os.Create(beego.AppPath + "/conf/clients.csv")
	if err != nil {
		Fatalln(err.Error())
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	for _, client := range s.Clients {
		record := []string{
			strconv.Itoa(client.Id),
			client.VerifyKey,
			client.Remark,
			strconv.FormatBool(client.Status),
			client.Cnf.U,
			client.Cnf.P,
			GetStrByBool(client.Cnf.Crypt),
			client.Cnf.Compress,
			strconv.Itoa(client.RateLimit),
			strconv.Itoa(int(client.Flow.FlowLimit)),
		}
		err := writer.Write(record)
		if err != nil {
			Fatalln(err.Error())
		}
	}
	writer.Flush()
}

//init csv from file
func GetCsvDb() *Csv {
	once.Do(func() {
		CsvDb = NewCsv()
		CsvDb.Init()
	})
	return CsvDb
}
