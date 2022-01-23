package db

import (
	"ehang.io/nps/lib/logger"
	"github.com/pkg/errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
	"log"
	"reflect"
	"time"
)

type Db interface {
	Init() error
	SetConfig(key string, val string) error
	GetConfig(key string) (string, error)
	Insert(table string, uuid string, data string) error
	Delete(table string, uuid string) error
	Update(table string, uuid string, data string) error
	Count(table string, filterValue string) (int64, error)
	QueryOne(table string, uuid string) (string, error)
	QueryAll(table string, filterValue string) ([]string, error)
	QueryPage(table string, limit int, offset int, filterValue string) ([]string, error)
}

type dbLogger struct{}

func (_ dbLogger) Write(p []byte) (n int, err error) {
	logger.Warn(string(p))
	return len(p), nil
}

var _ Db = (*SqliteDb)(nil)

type SqliteDb struct {
	Path string
	db   *gorm.DB
}

func NewSqliteDb(path string) *SqliteDb {
	return &SqliteDb{Path: path}
}

func (sd *SqliteDb) SetConfig(key string, val string) error {
	c := &Config{Key: key}
	err := sd.db.First(c, "key = ?", key).Error
	if err != nil {
		return sd.db.Create(&Config{Key: key, Val: val}).Error
	}
	return sd.db.Model(c).Update("val", val).Error
}

func (sd *SqliteDb) GetConfig(key string) (string, error) {
	c := &Config{}
	err := sd.db.First(c, "key = ?", key).Error
	return c.Val, err
}

func (sd *SqliteDb) Count(table string, filterValue string) (int64, error) {
	i, err := sd.GetTable(table, "", "")
	if err != nil {
		return 0, err
	}
	var count int64
	err = sd.db.Where("data LIKE ?", "%"+filterValue+"%").Model(i).Count(&count).Error
	return count, err
}

func (sd *SqliteDb) QueryOne(table string, uuid string) (string, error) {
	i, err := sd.GetTable(table, "", "")
	if err != nil {
		return "", err
	}
	err = sd.db.First(i, "uuid = ?", uuid).Error
	if err != nil {
		return "", err
	}

	return getData(i), err
}

func getData(i interface{}) string {
	immutable := reflect.ValueOf(i)
	return immutable.Elem().FieldByName("Data").String()
}

func (sd *SqliteDb) QueryAll(table string, filterValue string) (data []string, err error) {
	list := make([]string, 0)
	switch table {
	case "rule":
		var r []Rule
		d := sd.db.Order("id desc")
		if filterValue != "" {
			d = d.Where("data LIKE ?", "%"+filterValue+"%")
		}
		err = d.Find(&r).Error
		for _, v := range r {
			list = append(list, v.Data)
		}
	case "cert":
		var c []Cert
		d := sd.db.Order("id desc")
		if filterValue != "" {
			d = d.Where("data LIKE ?", "%"+filterValue+"%")
		}
		err = d.Find(&c).Error
		for _, v := range c {
			list = append(list, v.Data)
		}
	default:
		err = errors.New("error table")
	}
	return list, err
}

func (sd *SqliteDb) QueryPage(table string, limit int, offset int, filterValue string) (data []string, err error) {
	list := make([]string, 0)
	switch table {
	case "rule":
		var r []Rule
		d := sd.db.Limit(limit).Offset(offset)
		if filterValue != "" {
			d = d.Where("data LIKE ?", "%"+filterValue+"%")
		}
		err = d.Order("id desc").Find(&r).Error
		for _, v := range r {
			list = append(list, v.Data)
		}
	case "cert":
		var c []Cert
		d := sd.db.Limit(limit).Offset(offset)
		if filterValue != "" {
			d = d.Where("data LIKE ?", "%"+filterValue+"%")
		}
		err = d.Order("id desc").Find(&c).Error
		for _, v := range c {
			list = append(list, v.Data)
		}
	default:
		err = errors.New("error table")
	}
	return list, err
}

func (sd *SqliteDb) Insert(table string, uuid string, data string) error {
	i, err := sd.GetTable(table, uuid, data)
	if err != nil {
		return err
	}
	return sd.db.Create(i).Error
}

func (sd *SqliteDb) Delete(table string, uuid string) error {
	i, err := sd.GetTable(table, uuid, "")
	if err != nil {
		return err
	}
	return sd.db.Where("uuid", uuid).Delete(i).Error
}

func (sd *SqliteDb) Update(table string, uuid string, data string) error {
	i, err := sd.GetTable(table, uuid, "")
	if err != nil {
		return err
	}
	return sd.db.Model(i).Where("uuid", uuid).Update("data", data).Error
}

func (sd *SqliteDb) Init() error {
	var err error
	newLogger := glog.New(
		log.New(dbLogger{}, "\r\n", log.LstdFlags),
		glog.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  glog.Silent,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	sd.db, err = gorm.Open(sqlite.Open(sd.Path), &gorm.Config{Logger: newLogger})
	if err != nil {
		return err
	}
	return sd.db.AutoMigrate(&Cert{}, &Rule{}, &Config{})
}

func (sd *SqliteDb) GetTable(table string, uuid string, data string) (i interface{}, err error) {
	switch table {
	case "rule":
		i = &Rule{Uuid: uuid, Data: data}
	case "cert":
		i = &Cert{Uuid: uuid, Data: data}
	default:
		err = errors.New("error table")
	}
	return
}

type Cert struct {
	ID   uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Uuid string `gorm:"column:uuid" json:"uuid"`
	Data string `gorm:"column:data" json:"data"`
}

type Rule struct {
	ID   uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Uuid string `gorm:"column:uuid" json:"uuid"`
	Data string `gorm:"column:data" json:"data"`
}

type Config struct {
	ID  uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Key string `gorm:"column:key"  json:"key"`
	Val string `gorm:"column:val"  json:"val"`
}
