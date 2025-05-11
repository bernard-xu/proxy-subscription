package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

var (
	// 当前日志级别
	currentLogLevel = LogLevelInfo

	// 标准日志
	Logger = log.New(os.Stdout, "", log.LstdFlags)
)

// SetLogLevel 设置日志级别
func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}

// InitLogger 初始化日志配置
func InitLogger() {
	// 从环境变量读取日志级别
	logLevel := os.Getenv("LOG_LEVEL")
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		SetLogLevel(LogLevelDebug)
	case "INFO":
		SetLogLevel(LogLevelInfo)
	case "WARN":
		SetLogLevel(LogLevelWarn)
	case "ERROR":
		SetLogLevel(LogLevelError)
	case "FATAL":
		SetLogLevel(LogLevelFatal)
	default:
		// 默认使用Info级别
		SetLogLevel(LogLevelInfo)
	}
}

// Debug 输出调试日志
func Debug(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelDebug {
		Logger.Output(2, fmt.Sprintf("[DEBUG] "+format, v...))
	}
}

// Info 输出信息日志
func Info(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelInfo {
		Logger.Output(2, fmt.Sprintf("[INFO] "+format, v...))
	}
}

// Warn 输出警告日志
func Warn(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelWarn {
		Logger.Output(2, fmt.Sprintf("[WARN] "+format, v...))
	}
}

// Error 输出错误日志
func Error(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelError {
		Logger.Output(2, fmt.Sprintf("[ERROR] "+format, v...))
	}
}

// Fatal 输出致命错误日志并退出程序
func Fatal(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelFatal {
		Logger.Output(2, fmt.Sprintf("[FATAL] "+format, v...))
		os.Exit(1)
	}
}
