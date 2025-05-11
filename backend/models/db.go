package models

import (
	"fmt"
	"os"
	"path/filepath"
	"proxy-subscription/utils"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite" // 替换为纯 Go 实现的 SQLite 驱动
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormLogWriter 自定义的GORM日志写入器
type GormLogWriter struct{}

// Printf 实现io.Writer接口，自定义日志格式
func (w GormLogWriter) Printf(format string, args ...interface{}) {
	// 使用自定义日志系统记录
	message := fmt.Sprintf(format, args...)
	if len(message) > 0 {
		// 根据消息内容和类型判断日志级别
		if message[0] == '[' {
			switch message[1] {
			case 'e', 'E': // 错误
				utils.Error("SQL: %s", message)
			case 'w', 'W': // 警告
				utils.Warn("SQL: %s", message)
			case 's', 'S': // 慢查询
				utils.Warn("SQL(慢查询): %s", message)
			default:
				if containsSelectKeyword(message) {
					utils.Debug("SQL(查询): %s", message)
				} else if containsUpdateKeyword(message) {
					utils.Info("SQL(更新): %s", message)
				} else if containsInsertKeyword(message) {
					utils.Info("SQL(插入): %s", message)
				} else if containsDeleteKeyword(message) {
					utils.Info("SQL(删除): %s", message)
				} else {
					utils.Debug("SQL: %s", message)
				}
			}
		} else {
			utils.Debug("SQL: %s", message)
		}
	}
}

// 辅助函数检查SQL语句类型
func containsSelectKeyword(sql string) bool {
	return strings.Contains(strings.ToUpper(sql), "SELECT")
}

func containsUpdateKeyword(sql string) bool {
	return strings.Contains(strings.ToUpper(sql), "UPDATE")
}

func containsInsertKeyword(sql string) bool {
	return strings.Contains(strings.ToUpper(sql), "INSERT")
}

func containsDeleteKeyword(sql string) bool {
	return strings.Contains(strings.ToUpper(sql), "DELETE")
}

var DB *gorm.DB

// 默认管理员账户
const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin0505"
)

// InitDB 初始化数据库连接
func InitDB() error {
	// 确保数据目录存在
	dbDir := os.Getenv("DATA_DIR")
	if dbDir == "" {
		// 获取可执行文件所在目录
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		execDir := filepath.Dir(execPath)
		dbDir = filepath.Join(execDir, "data")
	}

	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(dbDir, "nekoray-config.db")
	var err error

	// 配置GORM
	logLevel := logger.Info
	if os.Getenv("GIN_MODE") == "release" || os.Getenv("LOG_LEVEL") == "ERROR" || os.Getenv("LOG_LEVEL") == "FATAL" {
		logLevel = logger.Error // 只记录错误
	} else if os.Getenv("LOG_LEVEL") == "WARN" {
		logLevel = logger.Warn // 记录警告和错误
	} else if os.Getenv("LOG_LEVEL") == "INFO" {
		logLevel = logger.Info // 记录信息、警告和错误
	} else if os.Getenv("LOG_LEVEL") == "DEBUG" {
		logLevel = logger.Info // GORM不支持DEBUG级别，使用Info代替
	}

	// 获取慢查询阈值配置
	slowThreshold := 100 * time.Millisecond // 默认100ms
	if thresholdStr := os.Getenv("SQL_SLOW_THRESHOLD"); thresholdStr != "" {
		if threshold, err := strconv.Atoi(thresholdStr); err == nil && threshold > 0 {
			slowThreshold = time.Duration(threshold) * time.Millisecond
			utils.Info("慢查询阈值设置为: %d ms", threshold)
		}
	}

	// 创建自定义的日志配置
	gormLogger := logger.New(
		GormLogWriter{}, // 使用自定义的日志写入器
		logger.Config{
			SlowThreshold:             slowThreshold, // 慢查询阈值
			LogLevel:                  logLevel,      // 日志级别
			IgnoreRecordNotFoundError: true,          // 忽略记录未找到错误
			Colorful:                  false,         // 禁用彩色输出
		},
	)

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})

	if err != nil {
		return err
	}

	// 自动迁移表结构
	if err := DB.AutoMigrate(&Subscription{}, &Proxy{}, &Setting{}, &User{}); err != nil {
		return err
	}

	// 创建默认管理员账户
	if err := createDefaultAdmin(); err != nil {
		return err
	}

	utils.Info("数据库初始化成功，位置: %s", dbPath)
	utils.Info("默认管理员账号: %s, 密码: %s", DefaultAdminUsername, DefaultAdminPassword)
	return nil
}

// createDefaultAdmin 创建默认管理员账户
func createDefaultAdmin() error {
	var count int64
	DB.Model(&User{}).Count(&count)

	// 只有当没有任何用户时才创建默认管理员
	if count == 0 {
		admin := User{
			Username:     DefaultAdminUsername,
			PasswordHash: HashPassword(DefaultAdminPassword),
			IsAdmin:      true,
		}

		result := DB.Create(&admin)
		if result.Error != nil {
			return fmt.Errorf("创建默认管理员账户失败: %v", result.Error)
		}
		utils.Info("创建默认管理员账户成功")
	}

	return nil
}
