package db

import (
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestSqlite(t *testing.T) {
	err := os.Remove(filepath.Join(os.TempDir(), "test.db"))
	assert.NoError(t, err)
	d := NewSqliteDb(filepath.Join(os.TempDir(), "test.db"))
	err = d.Init()
	assert.NoError(t, err)
	for _, tableName := range []string{"rule", "cert"} {
		var firstUuid, lastUuid string
		for i := 0; i < 1000; i++ {
			uid := uuid.NewV4().String()
			if i == 0 {
				firstUuid = uid
			}
			lastUuid = uid
			err = d.Insert(tableName, uid, "test"+strconv.Itoa(i))
			assert.NoError(t, err)
		}
		n, err := d.Count(tableName,"")
		assert.NoError(t, err)
		assert.Equal(t, int(n), 1000)
		list, err := d.QueryAll(tableName, "")
		assert.NoError(t, err)
		assert.Equal(t, len(list), 1000)
		one, err := d.QueryOne(tableName, firstUuid)
		assert.NoError(t, err)
		assert.Equal(t, one, "test0")
		list, err = d.QueryPage(tableName, 10, 10, "")
		assert.NoError(t, err)
		assert.Equal(t, len(list), 10)
		assert.Equal(t, list[0], "test989")
		err = d.Delete(tableName, lastUuid)
		assert.NoError(t, err)
		n, err = d.Count(tableName,"")
		assert.NoError(t, err)
		assert.Equal(t, n, int64(999))
		one, err = d.QueryOne(tableName, firstUuid)
		assert.NoError(t, err)
		err = d.Update(tableName, firstUuid, "test_new")
		assert.NoError(t, err)
		one, err = d.QueryOne(tableName, firstUuid)
		assert.NoError(t, err)
		assert.Equal(t, one, "test_new")
	}
	err = d.SetConfig("test_key1", "test_val1")
	assert.NoError(t, err)
	v, err := d.GetConfig("test_key1")
	assert.NoError(t, err)
	assert.Equal(t, v, "test_val1")
	v, err = d.GetConfig("test_key2")
	assert.Error(t, err)
	assert.Equal(t, v, "")
	err = d.SetConfig("test_key2", "test_val2")
	assert.NoError(t, err)
	v, err = d.GetConfig("test_key2")
	assert.NoError(t, err)
	assert.Equal(t, v, "test_val2")
	err = d.SetConfig("test_key1", "test_val2")
	assert.NoError(t, err)
	v, err = d.GetConfig("test_key1")
	assert.NoError(t, err)
	assert.Equal(t, v, "test_val2")
}
