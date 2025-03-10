package utils

import (
	"encoding/base64"
	"regexp"
	"strings"
)

// IsBase64 检查字符串是否为Base64编码
func IsBase64(s string) bool {
	s = strings.TrimSpace(s)
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil && regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`).MatchString(s)
}

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
func DecodeBase64(s string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}