package file

import (
	"github.com/cnlh/nps/lib/common"
	"sync"
)

var (
	CsvDb *Csv
	once  sync.Once
)
//init csv from file
func GetCsvDb() *Csv {
	once.Do(func() {
		CsvDb = NewCsv(common.GetRunPath())
		CsvDb.Init()
	})
	return CsvDb
}
