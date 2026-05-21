package logger

import "survey.cli/configs"

// Init 初始化全局日志器并返回实例
// @param cfg  全局配置（未来可扩展日志级别、文件输出）
// @returns    已初始化的 Logger 实例
func Init(cfg *configs.Config) *Logger {
	l := &Logger{prefix: "survey"}
	Logs = l
	return l
}
