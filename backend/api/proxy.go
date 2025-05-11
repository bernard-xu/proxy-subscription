package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"proxy-subscription/models"
	"proxy-subscription/services"

	"github.com/gin-gonic/gin"
)

// GetProxies 获取所有代理节点
func GetProxies(c *gin.Context) {
	type ProxyWithSubscription struct {
		models.Proxy
		SubscriptionName string `json:"subscription_name"`
	}

	var results []ProxyWithSubscription
	query := models.DB.Model(&models.Proxy{}).Select("proxies.*, subscriptions.name as subscription_name").Joins("left join subscriptions on proxies.subscription_id = subscriptions.id")

	// 支持按订阅ID过滤
	if subID := c.Query("subscription_id"); subID != "" {
		query = query.Where("proxies.subscription_id = ?", subID)
	}

	if err := query.Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
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

	// 尝试从缓存获取
	if content, contentType, found := services.GetSubscriptionCache(format); found {
		c.Header("Content-Type", contentType)
		c.Header("X-Cache", "HIT")
		c.String(http.StatusOK, content)
		return
	}

	// 缓存未命中，生成新的内容
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

	// 存入缓存
	services.SetSubscriptionCache(format, content, contentType)

	// 设置响应头
	c.Header("Content-Type", contentType)
	c.Header("X-Cache", "MISS")
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
	// 格式：ss://base64(method:password)@server:port?plugin=...#name
	if proxy.Method == "" || proxy.Password == "" {
		return ""
	}

	// 编码method:password部分
	auth := proxy.Method + ":" + proxy.Password
	encoded := base64.RawURLEncoding.EncodeToString([]byte(auth))

	// 构建基本URL
	result := "ss://" + encoded + "@" + proxy.Server + ":" + strconv.Itoa(proxy.Port)

	// 添加查询参数
	params := make([]string, 0)

	// 添加插件信息
	if proxy.Plugin != "" {
		pluginStr := proxy.Plugin
		if proxy.PluginOpts != "" {
			pluginStr += ";" + proxy.PluginOpts
		}
		params = append(params, "plugin="+url.QueryEscape(pluginStr))
	}

	// 如果有参数，添加到URL
	if len(params) > 0 {
		result += "?" + strings.Join(params, "&")
	}

	// 添加节点名称作为fragment
	if proxy.Name != "" {
		// URL编码节点名称
		encodedName := url.QueryEscape(proxy.Name)
		result += "#" + encodedName
	}

	return result
}

// 生成Trojan URL
func generateTrojanURL(proxy models.Proxy) string {
	// 格式：trojan://password@server:port?sni=xxx&alpn=xxx#name
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
	if proxy.AllowInsecure {
		params = append(params, "allowInsecure=1")
	}

	// 添加其他可能的参数（从RawConfig中提取）
	if proxy.RawConfig != "" {
		var rawConfig map[string]interface{}
		if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err == nil {
			for key, value := range rawConfig {
				// 跳过已处理的字段和非字符串值
				if key == "server" || key == "port" || key == "password" ||
					key == "sni" || key == "alpn" || key == "allowInsecure" {
					continue
				}

				// 只处理字符串值
				if strValue, ok := value.(string); ok && strValue != "" {
					params = append(params, key+"="+url.QueryEscape(strValue))
				}
			}
		}
	}

	// 如果有参数，添加到URL
	if len(params) > 0 {
		result += "?" + strings.Join(params, "&")
	}

	// 添加节点名称作为fragment
	if proxy.Name != "" {
		// URL编码节点名称
		encodedName := url.QueryEscape(proxy.Name)
		result += "#" + encodedName
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
		yaml.WriteString("  - name: " + proxy.Name + "\n")
		yaml.WriteString("    type: " + proxy.Type + "\n")
		yaml.WriteString("    server: " + proxy.Server + "\n")
		yaml.WriteString("    port: " + strconv.Itoa(proxy.Port) + "\n")

		// 根据代理类型添加特定配置
		switch proxy.Type {
		case "ss":
			yaml.WriteString("    cipher: " + proxy.Method + "\n")
			yaml.WriteString("    password: " + proxy.Password + "\n")

			// 添加插件配置
			if proxy.Plugin != "" {
				yaml.WriteString("    plugin: " + proxy.Plugin + "\n")
				if proxy.PluginOpts != "" {
					yaml.WriteString("    plugin-opts:\n")
					// 解析插件选项
					opts := strings.Split(proxy.PluginOpts, ";")
					for _, opt := range opts {
						if kv := strings.SplitN(opt, "=", 2); len(kv) == 2 {
							yaml.WriteString("      " + kv[0] + ": " + kv[1] + "\n")
						}
					}
				}
			}

		case "vmess":
			yaml.WriteString("    uuid: " + proxy.UUID + "\n")
			if proxy.Network != "" {
				yaml.WriteString("    network: " + proxy.Network + "\n")
			}
			if proxy.TLS {
				yaml.WriteString("    tls: true\n")
			}
			if proxy.Path != "" {
				yaml.WriteString("    ws-path: " + proxy.Path + "\n")
			}
			if proxy.Host != "" {
				yaml.WriteString("    ws-headers:\n")
				yaml.WriteString("      Host: " + proxy.Host + "\n")
			}

		case "trojan":
			yaml.WriteString("    password: " + proxy.Password + "\n")
			if proxy.SNI != "" {
				yaml.WriteString("    sni: " + proxy.SNI + "\n")
			}
			if proxy.ALPN != "" {
				yaml.WriteString("    alpn:\n")
				for _, alpn := range strings.Split(proxy.ALPN, ",") {
					yaml.WriteString("      - " + alpn + "\n")
				}
			}
			if proxy.AllowInsecure {
				yaml.WriteString("    skip-cert-verify: true\n")
			}
		}

		yaml.WriteString("\n")
	}

	return yaml.String()
}

// 生成JSON配置
func generateJSONConfig(proxies []models.Proxy) (string, error) {
	// 实现JSON配置生成逻辑
	type jsonProxy struct {
		Name          string `json:"name"`
		Type          string `json:"type"`
		Server        string `json:"server"`
		Port          int    `json:"port"`
		UUID          string `json:"uuid,omitempty"`
		Password      string `json:"password,omitempty"`
		Method        string `json:"method,omitempty"`
		Network       string `json:"network,omitempty"`
		Path          string `json:"path,omitempty"`
		Host          string `json:"host,omitempty"`
		TLS           bool   `json:"tls,omitempty"`
		SNI           string `json:"sni,omitempty"`
		ALPN          string `json:"alpn,omitempty"`
		Plugin        string `json:"plugin,omitempty"`
		PluginOpts    string `json:"plugin_opts,omitempty"`
		AllowInsecure bool   `json:"allow_insecure,omitempty"`
	}

	var jsonProxies []jsonProxy
	for _, proxy := range proxies {
		jp := jsonProxy{
			Name:          proxy.Name,
			Type:          proxy.Type,
			Server:        proxy.Server,
			Port:          proxy.Port,
			UUID:          proxy.UUID,
			Password:      proxy.Password,
			Method:        proxy.Method,
			Network:       proxy.Network,
			Path:          proxy.Path,
			Host:          proxy.Host,
			TLS:           proxy.TLS,
			SNI:           proxy.SNI,
			ALPN:          proxy.ALPN,
			Plugin:        proxy.Plugin,
			PluginOpts:    proxy.PluginOpts,
			AllowInsecure: proxy.AllowInsecure,
		}
		jsonProxies = append(jsonProxies, jp)
	}

	jsonData, err := json.Marshal(jsonProxies)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}
