package models

import (
	"time"

	"gorm.io/gorm"
)

// Subscription 订阅模型
type Subscription struct {
	gorm.Model
	Name        string    `json:"name" gorm:"not null"`
	URL         string    `json:"url" gorm:"not null"`
	Type        string    `json:"type" gorm:"not null"` // 支持的订阅类型，如v2ray, trojan, ss等
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	LastUpdated time.Time `json:"lastUpdated"`
	Proxies     []Proxy   `json:"proxies,omitempty" gorm:"foreignKey:SubscriptionID"`
}

// Proxy 代理节点模型
type Proxy struct {
	gorm.Model
	SubscriptionID uint   `json:"subscriptionId" gorm:"not null"`
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
	RawConfig      string `json:"rawConfig" gorm:"type:text"` // 存储原始配置
}