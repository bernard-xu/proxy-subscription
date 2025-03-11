package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"proxy-subscription/models"

	"github.com/gin-gonic/gin"
)

// GetProxies 获取所有代理节点
func GetProxies(c *gin.Context) {
	var proxies []models.Proxy
	query := models.DB.Model(&models.Proxy{})

	// 支持按订阅ID过滤
	if subID := c.Query("subscription_id"); subID != "" {
		query = query.Where("subscription_id = ?", subID)
	}

	if err := query.Find(&proxies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, proxies)
}

// GetProxy 获取单个代理节点
func GetProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var proxy models.Proxy
	if err := models.DB.First(&proxy, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "代理节点不存在"})
		return
	}

	c.JSON(http.StatusOK, proxy)
}

// GetMergedSubscription 获取合并后的订阅
func GetMergedSubscription(c *gin.Context) {
	format := c.DefaultQuery("format", "base64")

	// 获取所有启用的订阅
	var subscriptions []models.Subscription
	if err := models.DB.Where("enabled = ?", true).Find(&subscriptions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取所有代理节点
	var proxies []models.Proxy
	if err := models.DB.Joins("JOIN subscriptions ON proxies.subscription_id = subscriptions.id").
		Where("subscriptions.enabled = ?", true).
		Find(&proxies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 根据请求的格式生成订阅内容
	content, contentType, err := generateSubscriptionContent(proxies, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 设置响应头
	c.Header("Content-Type", contentType)
	c.String(http.StatusOK, content)
}

// 生成订阅内容
func generateSubscriptionContent(proxies []models.Proxy, format string) (string, string, error) {
	var content strings.Builder
	contentType := "text/plain;charset=utf-8"

	// 根据不同格式生成内容
	switch format {
	case "base64":
		// 生成每个代理的URL
		for _, proxy := range proxies {
			var proxyURL string
			switch proxy.Type {
			case "vmess":
				// 生成vmess链接
				proxyURL = generateVmessURL(proxy)
			case "ss":
				// 生成ss链接
				proxyURL = generateSSURL(proxy)
			case "trojan":
				// 生成trojan链接
				proxyURL = generateTrojanURL(proxy)
			default:
				continue
			}

			if proxyURL != "" {
				content.WriteString(proxyURL + "\n")
			}
		}

		// Base64编码
		encodedContent := base64.StdEncoding.EncodeToString([]byte(content.String()))
		return encodedContent, contentType, nil

	case "clash":
		// 生成Clash配置
		yamlContent := generateClashConfig(proxies)
		return yamlContent, "text/yaml;charset=utf-8", nil

	case "json":
		// 生成JSON格式
		jsonContent, err := generateJSONConfig(proxies)
		if err != nil {
			return "", "", err
		}
		return jsonContent, "application/json;charset=utf-8", nil

	default:
		return "", "", errors.New("不支持的格式: " + format)
	}
}

// 生成Vmess URL
func generateVmessURL(proxy models.Proxy) string {
	// 创建vmess配置JSON
	config := map[string]interface{}{
		"v":    "2",
		"ps":   proxy.Name,
		"add":  proxy.Server,
		"port": proxy.Port,
		"id":   proxy.UUID,
		"aid":  0,
		"net":  proxy.Network,
		"type": "none",
		"host": proxy.Host,
		"path": proxy.Path,
		"tls":  "",
	}

	// 设置TLS
	if proxy.TLS {
		config["tls"] = "tls"
	}

	// 如果Network为空，设置默认值
	if proxy.Network == "" {
		config["net"] = "tcp"
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(config)
	if err != nil {
		return ""
	}

	// Base64编码
	return "vmess://" + base64.StdEncoding.EncodeToString(jsonData)
}

// 生成Shadowsocks URL
func generateSSURL(proxy models.Proxy) string {
	// 格式：ss://base64(method:password)@server:port
	if proxy.Method == "" || proxy.Password == "" {
		return ""
	}

	// 编码method:password部分
	auth := proxy.Method + ":" + proxy.Password
	encoded := base64.RawURLEncoding.EncodeToString([]byte(auth))

	// 构建完整URL
	return "ss://" + encoded + "@" + proxy.Server + ":" + strconv.Itoa(proxy.Port)
}

// 生成Trojan URL
func generateTrojanURL(proxy models.Proxy) string {
	// 格式：trojan://password@server:port?sni=xxx&alpn=xxx
	if proxy.Password == "" {
		return ""
	}

	// 构建基本URL
	result := "trojan://" + proxy.Password + "@" + proxy.Server + ":" + strconv.Itoa(proxy.Port)

	// 添加查询参数
	params := make([]string, 0)
	if proxy.SNI != "" {
		params = append(params, "sni="+proxy.SNI)
	}
	if proxy.ALPN != "" {
		params = append(params, "alpn="+proxy.ALPN)
	}

	// 如果有参数，添加到URL
	if len(params) > 0 {
		result += "?" + strings.Join(params, "&")
	}

	return result
}

// 生成Clash配置
func generateClashConfig(proxies []models.Proxy) string {
	// 实现Clash配置生成逻辑
	var yaml strings.Builder

	yaml.WriteString("proxies:\n")
	for _, proxy := range proxies {
		// 根据代理类型生成对应的Clash配置
		// 这里只是一个简化的示例
		yaml.WriteString("  - name: " + proxy.Name + "\n")
		yaml.WriteString("    type: " + proxy.Type + "\n")
		yaml.WriteString("    server: " + proxy.Server + "\n")
		yaml.WriteString("    port: " + strconv.Itoa(proxy.Port) + "\n")
		// 添加其他配置...
		yaml.WriteString("\n")
	}

	return yaml.String()
}

// 生成JSON配置
func generateJSONConfig(proxies []models.Proxy) (string, error) {
	// 实现JSON配置生成逻辑
	type jsonProxy struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		Server string `json:"server"`
		Port   int    `json:"port"`
		// 其他字段...
	}

	var jsonProxies []jsonProxy
	for _, proxy := range proxies {
		jsonProxies = append(jsonProxies, jsonProxy{
			Name:   proxy.Name,
			Type:   proxy.Type,
			Server: proxy.Server,
			Port:   proxy.Port,
			// 设置其他字段...
		})
	}

	jsonData, err := json.Marshal(jsonProxies)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}
