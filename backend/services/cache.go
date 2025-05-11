package services

import (
	"crypto/md5"
	"encoding/hex"
	"sync"
	"time"
)

// 缓存结构体
type CacheItem struct {
	Content     string    // 缓存的内容
	ContentType string    // 内容类型
	Timestamp   time.Time // 缓存时间
}

// 全局缓存
var (
	subscriptionCache sync.Map
	cacheDuration     = 5 * time.Minute // 缓存有效期
)

// GetSubscriptionCache 从缓存获取订阅内容
func GetSubscriptionCache(format string) (string, string, bool) {
	key := getCacheKey(format)
	if item, exists := subscriptionCache.Load(key); exists {
		cacheItem := item.(CacheItem)
		// 检查缓存是否过期
		if time.Since(cacheItem.Timestamp) < cacheDuration {
			return cacheItem.Content, cacheItem.ContentType, true
		}
	}
	return "", "", false
}

// SetSubscriptionCache 设置订阅缓存
func SetSubscriptionCache(format, content, contentType string) {
	key := getCacheKey(format)
	cacheItem := CacheItem{
		Content:     content,
		ContentType: contentType,
		Timestamp:   time.Now(),
	}
	subscriptionCache.Store(key, cacheItem)
}

// getCacheKey 生成缓存键
func getCacheKey(format string) string {
	hash := md5.Sum([]byte(format))
	return "subscription_" + hex.EncodeToString(hash[:])
}

// InvalidateCache 使缓存失效
func InvalidateCache() {
	subscriptionCache = sync.Map{}
}
