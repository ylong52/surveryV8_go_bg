package main

import (
	"fmt"
	"os"
	"path/filepath"
	"survey.cli/core/database"
)

func main() {
	// 确保 data 目录存在
	dir := "./data"
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("创建目录失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化数据库
	dbPath := filepath.Join(dir, "survey.db")
	if err := database.Init(dbPath); err != nil {
		fmt.Printf("数据库初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	fmt.Printf("数据库已创建: %s\n", dbPath)
	fmt.Println("表结构已创建完成。")
}
