package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite" // 替换为纯 Go 实现的 SQLite 驱动
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
	if os.Getenv("GIN_MODE") == "release" {
		logLevel = logger.Error
	}

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
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

	log.Println("数据库初始化成功，位置:", dbPath)
	log.Printf("默认管理员账号: %s, 密码: %s\n", DefaultAdminUsername, DefaultAdminPassword)
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
		log.Println("创建默认管理员账户成功")
	}

	return nil
}
