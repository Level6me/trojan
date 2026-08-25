package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	dbPath     = "/var/lib/trojan-manager"
	backupFile = "/var/lib/trojan-manager/kv_backup.json"
	globalDB   *leveldb.DB
	dbMutex    sync.RWMutex
)

// getDB 获取全局单例 LevelDB 实例（带重试机制，杜绝并发文件锁争抢）
func getDB() (*leveldb.DB, error) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if globalDB != nil {
		return globalDB, nil
	}

	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, err
	}

	var db *leveldb.DB
	var err error

	opts := &opt.Options{
		OpenFilesCacheCapacity: 50,
	}

	// 尝试最多重试 5 次，避免瞬间锁冲突
	for i := 0; i < 5; i++ {
		db, err = leveldb.OpenFile(dbPath, opts)
		if err == nil {
			globalDB = db
			return globalDB, nil
		}
		time.Sleep(time.Millisecond * 50)
	}

	return nil, err
}

// readBackup 从本地磁盘 JSON 冗余备份中读取
func readBackup(key string) string {
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return ""
	}
	var kvMap map[string]string
	if err := json.Unmarshal(data, &kvMap); err != nil {
		return ""
	}
	return kvMap[key]
}

// writeBackup 同步写入本地磁盘 JSON 冗余备份
func writeBackup(key string, value string) {
	var kvMap = make(map[string]string)
	if data, err := os.ReadFile(backupFile); err == nil {
		_ = json.Unmarshal(data, &kvMap)
	}
	if value == "" {
		delete(kvMap, key)
	} else {
		kvMap[key] = value
	}
	data, err := json.MarshalIndent(kvMap, "", "  ")
	if err == nil {
		_ = os.MkdirAll(filepath.Dir(backupFile), 0755)
		_ = os.WriteFile(backupFile, data, 0600)
	}
}

// GetValue 获取 leveldb 值（双保险：LevelDB + JSON 自动回退与自愈，彻底杜绝 Issue #873）
func GetValue(key string) (string, error) {
	db, err := getDB()
	if err == nil {
		result, err := db.Get([]byte(key), nil)
		if err == nil && len(result) > 0 {
			val := string(result)
			go writeBackup(key, val)
			return val, nil
		}
	}

	// LevelDB 异常或为空时，自动从持久化备份中回退检索
	backupVal := readBackup(key)
	if backupVal != "" {
		if db != nil {
			_ = db.Put([]byte(key), []byte(backupVal), nil)
		}
		return backupVal, nil
	}

	return "", err
}

// SetValue 设置 leveldb 值（同步写入 LevelDB 与持久化文件）
func SetValue(key string, value string) error {
	writeBackup(key, value)
	db, err := getDB()
	if err != nil {
		return err
	}
	return db.Put([]byte(key), []byte(value), nil)
}

// DelValue 删除值
func DelValue(key string) error {
	writeBackup(key, "")
	db, err := getDB()
	if err != nil {
		return err
	}
	return db.Delete([]byte(key), nil)
}
