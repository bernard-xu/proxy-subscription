package models

import (
	"gorm.io/gorm"
)

// Setting 系统设置模型
type Setting struct {
	gorm.Model
	Key   string `json:"key" gorm:"uniqueIndex"`
	Value string `json:"value"`
}

// 设置键名常量
const (
	SettingAutoRefresh      = "auto_refresh"
	SettingRefreshInterval  = "refresh_interval"
	SettingDefaultFormat    = "default_format"
)