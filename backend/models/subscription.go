package models

import (
	"time"
)

// Subscription 订阅模型
type Subscription struct {
	BaseModel
	Name        string    `json:"name" gorm:"not null"`
	URL         string    `json:"url" gorm:"not null"`
	Type        string    `json:"type" gorm:"not null"` // 支持的订阅类型，如v2ray, trojan, ss等
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	LastUpdated time.Time `json:"lastUpdated"`
	Proxies     []Proxy   `json:"proxies,omitempty" gorm:"foreignKey:SubscriptionID"`
}

// Proxy 代理节点模型
type Proxy struct {
	BaseModel
	SubscriptionID uint   `json:"subscription_id" gorm:"not null"`
	Name           string `json:"name" gorm:"not null"`
	Type           string `json:"type" gorm:"not null"` // v2ray, ss, trojan等
	Server         string `json:"server" gorm:"not null"`
	Port           int    `json:"port" gorm:"not null"`
	UUID           string `json:"uuid"`
	Password       string `json:"password"`
	Method         string `json:"method"`
	Network        string `json:"network"`
	Path           string `json:"path"`
	Host           string `json:"host"`
	TLS            bool   `json:"tls"`
	SNI            string `json:"sni"`
	ALPN           string `json:"alpn"`
	Plugin         string `json:"plugin"`                     // Shadowsocks插件名称
	PluginOpts     string `json:"plugin_opts"`                // Shadowsocks插件选项
	AllowInsecure  bool   `json:"allow_insecure"`             // 是否允许不安全连接（跳过证书验证）
	RawConfig      string `json:"rawConfig" gorm:"type:text"` // 存储原始配置
}
