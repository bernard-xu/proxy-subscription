package models

// Setting 系统设置模型
type Setting struct {
	BaseModel
	Key   string `json:"key" gorm:"uniqueIndex"`
	Value string `json:"value"`
}

// 设置键名常量
const (
	SettingAutoRefresh     = "auto_refresh"
	SettingRefreshInterval = "refresh_interval"
	SettingDefaultFormat   = "default_format"
)
