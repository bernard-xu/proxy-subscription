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

	var proxies []models.Proxy
	query := models.DB.Model(&models.Proxy{})

	// 支持按订阅ID过滤
	if subID := c.Query("subscription_id"); subID != "" {
		if subID == "-1" || strings.EqualFold(subID, "custom") {
			query = query.Where("proxies.is_custom = ?", true)
		} else {
			query = query.Where("proxies.subscription_id = ?", subID)
		}
	}

	if err := query.Find(&proxies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	subscriptionIDs := make([]uint, 0)
	seenSubscriptionIDs := make(map[uint]struct{})
	for _, proxy := range proxies {
		if proxy.IsCustom || proxy.SubscriptionID == 0 {
			continue
		}
		if _, exists := seenSubscriptionIDs[proxy.SubscriptionID]; exists {
			continue
		}
		seenSubscriptionIDs[proxy.SubscriptionID] = struct{}{}
		subscriptionIDs = append(subscriptionIDs, proxy.SubscriptionID)
	}

	subscriptionNames := make(map[uint]string, len(subscriptionIDs))
	if len(subscriptionIDs) > 0 {
		var subscriptions []models.Subscription
		if err := models.DB.Select("id", "name").Find(&subscriptions, subscriptionIDs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, subscription := range subscriptions {
			subscriptionNames[subscription.ID] = subscription.Name
		}
	}

	results := make([]ProxyWithSubscription, 0, len(proxies))
	// 为每个代理设置显示名称
	for _, proxy := range proxies {
		proxy.DisplayName = proxy.GetDisplayName()
		result := ProxyWithSubscription{Proxy: proxy}
		if proxy.IsCustom {
			result.SubscriptionName = "自定义节点"
		} else {
			result.SubscriptionName = subscriptionNames[proxy.SubscriptionID]
		}
		results = append(results, result)
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

	// 设置显示名称
	proxy.DisplayName = proxy.GetDisplayName()

	c.JSON(http.StatusOK, proxy)
}

// AddCustomProxy 添加手动自定义代理节点
func AddCustomProxy(c *gin.Context) {
	var proxy models.Proxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点参数"})
		return
	}

	if err := normalizeProxyFields(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	proxy.SubscriptionID = 0
	proxy.IsCustom = true
	proxy.ManualOverride = true
	proxy.SourceKey = proxy.BuildSourceKey()

	if err := models.DB.Create(&proxy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.InvalidateCache()
	proxy.DisplayName = proxy.GetDisplayName()
	c.JSON(http.StatusCreated, proxy)
}

// UpdateProxy 手动更新代理节点
func UpdateProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var existing models.Proxy
	if err := models.DB.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "代理节点不存在"})
		return
	}

	var proxy models.Proxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点参数"})
		return
	}
	if err := normalizeProxyFields(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	proxy.ID = existing.ID
	proxy.CreatedAt = existing.CreatedAt
	proxy.SubscriptionID = existing.SubscriptionID
	proxy.IsCustom = existing.IsCustom
	proxy.ManualOverride = true
	if existing.SourceKey != "" {
		proxy.SourceKey = existing.SourceKey
	} else {
		proxy.SourceKey = existing.BuildSourceKey()
	}
	if err := models.DB.Save(&proxy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.InvalidateCache()
	proxy.DisplayName = proxy.GetDisplayName()
	c.JSON(http.StatusOK, proxy)
}

// DeleteCustomProxy 删除手动自定义代理节点
func DeleteCustomProxy(c *gin.Context) {
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
	if !proxy.IsCustom {
		c.JSON(http.StatusForbidden, gin.H{"error": "订阅节点不能手动删除"})
		return
	}

	if err := models.DB.Delete(&proxy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.InvalidateCache()
	c.JSON(http.StatusOK, gin.H{"message": "节点已删除"})
}

func normalizeProxyFields(proxy *models.Proxy) error {
	proxy.Name = strings.TrimSpace(proxy.Name)
	proxy.Type = strings.ToLower(strings.TrimSpace(proxy.Type))
	proxy.Server = strings.TrimSpace(proxy.Server)
	proxy.UUID = strings.TrimSpace(proxy.UUID)
	proxy.Password = strings.TrimSpace(proxy.Password)
	proxy.Method = strings.TrimSpace(proxy.Method)
	proxy.Network = strings.TrimSpace(proxy.Network)
	proxy.Path = strings.TrimSpace(proxy.Path)
	proxy.Host = strings.TrimSpace(proxy.Host)
	proxy.SNI = strings.TrimSpace(proxy.SNI)
	proxy.ALPN = strings.TrimSpace(proxy.ALPN)
	proxy.Plugin = strings.TrimSpace(proxy.Plugin)
	proxy.PluginOpts = strings.TrimSpace(proxy.PluginOpts)
	proxy.RawConfig = strings.TrimSpace(proxy.RawConfig)

	if proxy.Name == "" {
		return errors.New("节点名称不能为空")
	}
	if proxy.Server == "" {
		return errors.New("服务器不能为空")
	}
	if proxy.Port <= 0 || proxy.Port > 65535 {
		return errors.New("端口必须在 1-65535 之间")
	}

	switch proxy.Type {
	case "vmess", "vless":
		if proxy.UUID == "" {
			return errors.New("VMess 节点必须填写 UUID")
		}
		if proxy.Network == "" {
			proxy.Network = "tcp"
		}
	case "ss":
		if proxy.Method == "" || proxy.Password == "" {
			return errors.New("Shadowsocks 节点必须填写加密方式和密码")
		}
	case "trojan":
		if proxy.Password == "" {
			return errors.New("Trojan 节点必须填写密码")
		}
	case "tuic":
		if proxy.UUID == "" || proxy.Password == "" {
			return errors.New("TUIC 节点必须填写 UUID 和密码")
		}
		proxy.TLS = true
	case "anytls", "hysteria2":
		if proxy.Password == "" {
			return errors.New("AnyTLS/Hysteria2 节点必须填写密码")
		}
		proxy.TLS = true
	default:
		return errors.New("仅支持 vmess、vless、ss、trojan、tuic、anytls、hysteria2 类型")
	}
	return nil
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
	if err := models.DB.Joins("LEFT JOIN subscriptions ON proxies.subscription_id = subscriptions.id").
		Where("proxies.is_custom = ? OR subscriptions.enabled = ?", true, true).
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
			case "vless":
				proxyURL = generateVlessURL(proxy)
			case "ss":
				// 生成ss链接
				proxyURL = generateSSURL(proxy)
			case "trojan":
				// 生成trojan链接
				proxyURL = generateTrojanURL(proxy)
			case "tuic", "anytls", "hysteria2":
				proxyURL = generateCredentialProxyURL(proxy)
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
func generateVlessURL(proxy models.Proxy) string {
	if proxy.UUID == "" {
		return ""
	}

	result := "vless://" + url.QueryEscape(proxy.UUID) + "@" + proxy.Server + ":" + strconv.Itoa(proxy.Port)
	params := url.Values{}
	params.Set("encryption", "none")

	rawConfig := map[string]interface{}{}
	if proxy.RawConfig != "" {
		_ = json.Unmarshal([]byte(proxy.RawConfig), &rawConfig)
	}

	if security, ok := rawConfig["security"].(string); ok && security != "" {
		params.Set("security", security)
	} else if proxy.TLS {
		params.Set("security", "tls")
	}
	if proxy.Network != "" {
		params.Set("type", proxy.Network)
	}
	if proxy.SNI != "" {
		params.Set("sni", proxy.SNI)
	}
	if proxy.Host != "" {
		params.Set("host", proxy.Host)
	}
	if proxy.Path != "" {
		params.Set("path", proxy.Path)
	}
	if proxy.ALPN != "" {
		params.Set("alpn", proxy.ALPN)
	}
	if proxy.AllowInsecure {
		params.Set("allowInsecure", "1")
	}

	for key, value := range rawConfig {
		if _, exists := params[key]; exists {
			continue
		}
		if strValue, ok := value.(string); ok && strValue != "" {
			params.Set(key, strValue)
		}
	}

	if encoded := params.Encode(); encoded != "" {
		result += "?" + encoded
	}
	if proxy.Name != "" {
		result += "#" + url.QueryEscape(proxy.Name)
	}
	return result
}

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

func generateCredentialProxyURL(proxy models.Proxy) string {
	if proxy.Type == "tuic" && (proxy.UUID == "" || proxy.Password == "") {
		return ""
	}
	if (proxy.Type == "anytls" || proxy.Type == "hysteria2") && proxy.Password == "" {
		return ""
	}

	user := url.QueryEscape(proxy.Password)
	if proxy.Type == "tuic" {
		user = url.QueryEscape(proxy.UUID) + ":" + url.QueryEscape(proxy.Password)
	}
	result := proxy.Type + "://" + user + "@" + proxy.Server + ":" + strconv.Itoa(proxy.Port)
	params := url.Values{}
	if proxy.SNI != "" {
		params.Set("sni", proxy.SNI)
	}
	if proxy.ALPN != "" {
		params.Set("alpn", proxy.ALPN)
	}
	if proxy.AllowInsecure {
		params.Set("allowInsecure", "1")
	}

	if proxy.RawConfig != "" {
		var rawConfig map[string]interface{}
		if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err == nil {
			for key, value := range rawConfig {
				if key == "uuid" || key == "password" || key == "server" || key == "port" ||
					key == "sni" || key == "servername" || key == "alpn" || key == "allowInsecure" {
					continue
				}
				if strValue, ok := value.(string); ok && strValue != "" {
					params.Set(key, strValue)
				}
			}
		}
	}

	if encoded := params.Encode(); encoded != "" {
		result += "?" + encoded
	}
	if proxy.Name != "" {
		result += "#" + url.QueryEscape(proxy.Name)
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

		case "vless":
			yaml.WriteString("    uuid: " + proxy.UUID + "\n")
			if proxy.Network != "" {
				yaml.WriteString("    network: " + proxy.Network + "\n")
			}
			if proxy.TLS {
				yaml.WriteString("    tls: true\n")
			}
			if proxy.SNI != "" {
				yaml.WriteString("    servername: " + proxy.SNI + "\n")
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
		case "tuic":
			yaml.WriteString("    uuid: ")
			yaml.WriteString(proxy.UUID)
			yaml.WriteString("\n")
			yaml.WriteString("    password: " + proxy.Password + "\n")
			if proxy.SNI != "" {
				yaml.WriteString("    sni: " + proxy.SNI + "\n")
			}
			if proxy.ALPN != "" {
				yaml.WriteString("    alpn:\n")
				for _, alpn := range strings.Split(proxy.ALPN, ",") {
					yaml.WriteString("      - " + strings.TrimSpace(alpn) + "\n")
				}
			}
			if proxy.AllowInsecure {
				yaml.WriteString("    skip-cert-verify: true\n")
			}
		case "anytls", "hysteria2":
			yaml.WriteString("    password: " + proxy.Password + "\n")
			if proxy.SNI != "" {
				yaml.WriteString("    sni: " + proxy.SNI + "\n")
			}
			if proxy.ALPN != "" {
				yaml.WriteString("    alpn:\n")
				for _, alpn := range strings.Split(proxy.ALPN, ",") {
					yaml.WriteString("      - " + strings.TrimSpace(alpn) + "\n")
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
