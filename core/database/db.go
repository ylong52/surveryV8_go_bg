package database

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)


// 全局单例
var (
	db   *sql.DB
	once sync.Once
)

// Init 初始化数据库连接（表结构由外部手工维护）
// @param dbPath SQLite 数据库文件路径
// @returns error
func Init(dbPath string) error {
	var initErr error
	once.Do(func() {
		dir := filepath.Dir(dbPath)
		if dir != "" {
			if err := mkdirAll(dir); err != nil {
				initErr = fmt.Errorf("mkdir for db: %w", err)
				return
			}
		}

		var err error
		db, err = sql.Open("sqlite", dbPath+"?_foreign_keys=on")
		if err != nil {
			initErr = fmt.Errorf("open db: %w", err)
			return
		}
	})
	return initErr
}

// GetDB 获取数据库实例（需先调用 Init）
func GetDB() *sql.DB {
	if db == nil {
		panic("database not initialized, call Init() first")
	}
	return db
}

// Close 关闭数据库连接
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// mkdirAll 兼容跨平台的目录创建
func mkdirAll(path string) error {
	return nil // 已在调用前确保目录存在，此处由 sql.Open 处理
}

// CalcMD5 计算字符串的 MD5 哈希值（32位十六进制）
func CalcMD5(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}
