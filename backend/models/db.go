package models

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite" // 替换为纯 Go 实现的 SQLite 驱动
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() error {
	// 确保数据目录存在
	dbDir := os.Getenv("DATA_DIR")
	if dbDir == "" {
		dbDir = "./data"
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
	if err := DB.AutoMigrate(&Subscription{}, &Proxy{}, &Setting{}); err != nil {
		return err
	}

	log.Println("数据库初始化成功")
	return nil
}