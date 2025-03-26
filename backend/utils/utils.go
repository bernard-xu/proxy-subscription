package utils

import (
	"encoding/base64"
	"fmt"
	"strconv"
)

// GenerateProxyURL 根据代理信息生成URL
func GenerateProxyURL(proxyType, server, port, uuid, password, method string) string {
	// 根据不同代理类型生成对应的URL
	// 这里只是一个简化的示例
	return "proxy_url"
}

// EncodeBase64 Base64编码
func EncodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// DecodeBase64 Base64解码
func DecodeBase64(s string) ([]byte, error) {
	bytes, _ := base64.StdEncoding.DecodeString(s)
	return bytes, nil
}

// GetString 从map中获取字符串值
func GetString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			return strconv.Itoa(v)
		case bool:
			return strconv.FormatBool(v)
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

// GetInt 从map中获取整数值
func GetInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return 0
}
