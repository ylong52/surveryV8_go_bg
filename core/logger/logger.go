package logger

// 提供 Logs.Print() 风格的全局日志调用

import (
	"fmt"
	"time"
)

// Logger 轻量级日志器
type Logger struct {
	prefix string
}

// Logs 全局单例，由 Init() 设置
var Logs *Logger

// Print 输出带时间戳的日志行
func (l *Logger) Print(msg string) {
	fmt.Printf("[%s][%s] %s\n", l.prefix, time.Now().Format("15:04:05"), msg)
}

// Info 格式化输出普通日志
func (l *Logger) Info(format string, args ...any) {
	l.Print(fmt.Sprintf(format, args...))
}

// Error 格式化输出错误日志
func (l *Logger) Error(format string, args ...any) {
	l.Print("[ERR] " + fmt.Sprintf(format, args...))
}
