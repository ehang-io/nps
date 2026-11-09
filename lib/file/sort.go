package file

import (
	"reflect"
	"sort"
	"sync"
)

// A data structure to hold a key/value pair.
type Pair struct {
	key   string //sort key
	cId   int
	order string
	isNum bool
	num   int64
	str   string
}

// A slice of Pairs that implements sort.Interface to sort by Value.
type PairList []*Pair

func (p PairList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p PairList) Len() int      { return len(p) }
func (p PairList) Less(i, j int) bool {
	var less, greater bool
	if p[i].isNum && p[j].isNum {
		less, greater = p[i].num < p[j].num, p[i].num > p[j].num
	} else {
		less, greater = p[i].str < p[j].str, p[i].str > p[j].str
	}
	if p[i].order == "desc" {
		return less
	}
	return greater
}

//reflect 取值时不复制结构体（Flow 含 sync.RWMutex，复制会触发 copylocks）
func fieldValue(v reflect.Value, key string) (isNum bool, num int64, str string, ok bool) {
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return false, 0, "", false
	}
	f := v.FieldByName(key)
	if !f.IsValid() {
		return false, 0, "", false
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true, f.Int(), "", true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true, int64(f.Uint()), "", true
	case reflect.Float32, reflect.Float64:
		return true, int64(f.Float()), "", true
	case reflect.Bool:
		if f.Bool() {
			return true, 1, "", true
		}
		return true, 0, "", true
	case reflect.String:
		return false, 0, f.String(), true
	}
	return false, 0, "", false
}

//支持按 Client 字段（Id/Remark/Version/...）或 Flow 字段（InletFlow/ExportFlow/FlowLimit）排序，
//未知字段返回 false，避免原来 FieldByName 取到非法值后 .Int() panic
func clientSortValue(c *Client, key string) (isNum bool, num int64, str string) {
	if c == nil {
		return true, 0, ""
	}
	if isNum, num, str, ok := fieldValue(reflect.ValueOf(c).Elem(), key); ok {
		return isNum, num, str
	}
	if c.Flow != nil {
		if isNum, num, str, ok := fieldValue(reflect.ValueOf(c.Flow).Elem(), key); ok {
			return isNum, num, str
		}
	}
	return true, 0, ""
}

// A function to turn a map into a PairList, then sort and return it.
func sortClientByKey(m *sync.Map, sortKey, order string) (res []int) {
	p := make(PairList, 0)
	m.Range(func(key, value interface{}) bool {
		c := value.(*Client)
		isNum, num, str := clientSortValue(c, sortKey)
		p = append(p, &Pair{key: sortKey, cId: c.Id, order: order, isNum: isNum, num: num, str: str})
		return true
	})
	sort.Sort(p)
	for _, v := range p {
		res = append(res, v.cId)
	}
	return
}
