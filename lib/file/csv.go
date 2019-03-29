package file

import (
	"github.com/cnlh/nps/lib/common"
	"sort"
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
		CsvDb.LoadClientFromCsv()
		CsvDb.LoadTaskFromCsv()
		CsvDb.LoadHostFromCsv()
	})
	return CsvDb
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
