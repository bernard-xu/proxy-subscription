package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"proxy-subscription/models"
	"proxy-subscription/utils"
)

// RefreshSubscription 刷新订阅内容
func RefreshSubscription(subscription *models.Subscription) error {
	// 获取订阅内容
	content, err := fetchSubscriptionContent(subscription.URL)
	if err != nil {
		return err
	}

	// 解析订阅内容
	proxies, err := parseSubscriptionContent(content, subscription.Type)
	if err != nil {
		return err
	}

	// 开始事务
	tx := models.DB.Begin()

	// 删除旧的代理节点
	if err := tx.Where("subscription_id = ?", subscription.ID).Delete(&models.Proxy{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 添加新的代理节点
	for _, proxy := range proxies {
		proxy.SubscriptionID = subscription.ID
		if err := tx.Create(&proxy).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// 更新订阅的最后更新时间
	subscription.LastUpdated = time.Now()
	if err := tx.Save(subscription).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	return tx.Commit().Error
}

// fetchSubscriptionContent 获取订阅内容
func fetchSubscriptionContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("获取订阅内容失败，HTTP状态码: " + resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// parseSubscriptionContent 解析订阅内容
func parseSubscriptionContent(content string, subType string) ([]models.Proxy, error) {
	// 解码Base64内容
	decoded, _ := utils.DecodeBase64(content)
	content = string(decoded)

	// 根据订阅类型解析内容
	switch subType {
	case "v2ray":
		return parseV2raySubscription(content)
	case "ss":
		return parseSSSubscription(content)
	case "trojan":
		return parseTrojanSubscription(content)
	case "mixed":
		return parseMixedSubscription(content)
	case "sip002":
		return parseSSSubscription(content) // SIP002是SS的一种标准格式
	case "sip008":
		return parseSIP008Subscription(content)
	case "clash":
		return parseClashSubscription(content)
	case "surge":
		return parseSurgeSubscription(content)
	case "quantumult":
		return parseQuantumultSubscription(content)
	case "json":
		return parseJSONSubscription(content)
	default:
		// 尝试自动检测类型
		return autoDetectAndParse(content)
	}
}

// parseV2raySubscription 解析V2Ray订阅
func parseV2raySubscription(content string) ([]models.Proxy, error) {
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 处理vmess://开头的链接
		if strings.HasPrefix(line, "vmess://") {
			proxy, err := parseVmessLink(line)
			if err != nil {
				continue // 跳过解析失败的链接
			}
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// parseSSSubscription 解析Shadowsocks订阅
func parseSSSubscription(content string) ([]models.Proxy, error) {
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 处理ss://开头的链接
		if strings.HasPrefix(line, "ss://") {
			proxy, err := parseSSLink(line)
			if err != nil {
				continue // 跳过解析失败的链接
			}
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// parseTrojanSubscription 解析Trojan订阅
func parseTrojanSubscription(content string) ([]models.Proxy, error) {
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 处理trojan://开头的链接
		if strings.HasPrefix(line, "trojan://") {
			proxy, err := parseTrojanLink(line)
			if err != nil {
				continue // 跳过解析失败的链接
			}
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// autoDetectAndParse 自动检测并解析订阅类型
func autoDetectAndParse(content string) ([]models.Proxy, error) {
	// 检查是否为JSON格式
	if len(content) > 0 && (strings.TrimSpace(content)[0] == '{' || strings.TrimSpace(content)[0] == '[') {
		return parseJSONSubscription(content)
	}

	// 检查是否为Clash配置
	if strings.Contains(content, "proxies:") && (strings.Contains(content, "yaml") || strings.Contains(content, "rules:")) {
		return parseClashSubscription(content)
	}

	// 检查是否为Surge配置
	if strings.Contains(content, "[Proxy]") || strings.Contains(content, "[Proxy Group]") {
		return parseSurgeSubscription(content)
	}

	// 检查是否为Quantumult配置
	if strings.Contains(content, "shadowsocks=") || strings.Contains(content, "vmess=") || strings.Contains(content, "SERVER,") {
		return parseQuantumultSubscription(content)
	}

	// 逐行解析URI
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var proxy models.Proxy
		var err error

		switch {
		case strings.HasPrefix(line, "vmess://"):
			proxy, err = parseVmessLink(line)
		case strings.HasPrefix(line, "ss://"):
			proxy, err = parseSSLink(line)
		case strings.HasPrefix(line, "trojan://"):
			proxy, err = parseTrojanLink(line)
		case strings.HasPrefix(line, "ssr://"):
			proxy, err = parseSSRLink(line)
		case strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://"):
			proxy, err = parseHTTPLink(line)
		case strings.HasPrefix(line, "socks://") || strings.HasPrefix(line, "socks5://"):
			proxy, err = parseSOCKSLink(line)
		default:
			continue // 跳过不支持的链接
		}

		if err == nil {
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// parseVmessLink 解析Vmess链接
func parseVmessLink(link string) (models.Proxy, error) {
	// 解析vmess链接格式：vmess://base64(JSON配置)
	if len(link) < 9 {
		return models.Proxy{}, errors.New("invalid vmess link")
	}

	// 解码base64内容
	encoded := link[8:]
	decoded, _ := utils.DecodeBase64(encoded)

	// 使用map[string]interface{}解析JSON，以处理字段类型的多样性
	var configMap map[string]interface{}
	if err := json.Unmarshal(decoded, &configMap); err != nil {
		return models.Proxy{}, fmt.Errorf("json unmarshal error: %v", err)
	}

	// 从map中提取字段，处理不同类型的情况
	server := utils.GetString(configMap, "add")
	port := utils.GetInt(configMap, "port")
	uuid := utils.GetString(configMap, "id")
	network := utils.GetString(configMap, "net")
	tls := utils.GetString(configMap, "tls")
	host := utils.GetString(configMap, "host")
	path := utils.GetString(configMap, "path")
	nodeName := utils.GetString(configMap, "ps")

	// 验证必要字段
	if server == "" || port == 0 || uuid == "" {
		return models.Proxy{}, errors.New("missing required vmess parameters")
	}

	// 设置节点名称，如果PS为空则使用默认名称
	if nodeName == "" {
		nodeName = "VMess Node"
	}

	// 处理路径，确保有默认值
	if path == "" {
		path = "/"
	}

	return models.Proxy{
		Type:      "vmess",
		Name:      nodeName,
		Server:    server,
		Port:      port,
		UUID:      uuid,
		Network:   network,
		Path:      path,
		Host:      host,
		TLS:       tls == "tls",
		RawConfig: string(decoded),
	}, nil
}

// parseSSLink 解析Shadowsocks链接
func parseSSLink(link string) (models.Proxy, error) {
	// 解析格式：ss://base64(method:password)@host:port?plugin=...#name
	// 或者 SIP002格式：ss://base64(method:password@host:port)#name
	if len(link) < 6 {
		return models.Proxy{}, errors.New("SS链接格式错误：链接太短")
	}

	// 移除ss://前缀
	ssURL := link[5:]

	// 分离fragment（节点名称）
	var fragment string
	if idx := strings.Index(ssURL, "#"); idx >= 0 {
		fragment = ssURL[idx+1:]
		ssURL = ssURL[:idx]
		// URL解码节点名称
		if decodedFragment, err := url.QueryUnescape(fragment); err == nil {
			fragment = decodedFragment
		}
	}

	// 分离查询参数
	var pluginStr string
	if idx := strings.Index(ssURL, "?"); idx >= 0 {
		query := ssURL[idx+1:]
		ssURL = ssURL[:idx]

		// 解析查询参数
		params, err := url.ParseQuery(query)
		if err == nil {
			pluginStr = params.Get("plugin")
		}
	}

	// 尝试解析SIP002格式
	if !strings.Contains(ssURL, "@") {
		// 可能是整个URL都被base64编码的情况
		decodedBytes, err := base64.RawURLEncoding.DecodeString(ssURL)
		if err != nil {
			// 尝试标准base64解码
			decodedBytes, _ = utils.DecodeBase64(ssURL)
		}

		// 解码后应该是 method:password@host:port 格式
		decoded := string(decodedBytes)
		if !strings.Contains(decoded, "@") {
			return models.Proxy{}, errors.New("SS链接格式错误：缺少@分隔符")
		}

		// 重新设置ssURL为解码后的内容
		ssURL = decoded
	}

	// 分离用户信息和服务器信息
	parts := strings.SplitN(ssURL, "@", 2)
	if len(parts) != 2 {
		return models.Proxy{}, errors.New("SS链接格式错误：无法分离用户信息和服务器信息")
	}

	// 解析认证信息
	var method, password string
	authPart := parts[0]

	// 检查认证部分是否需要base64解码
	if !strings.Contains(authPart, ":") {
		// 尝试base64解码
		decodedAuth, err := base64.RawURLEncoding.DecodeString(authPart)
		if err != nil {
			// 尝试标准base64解码
			decodedAuth, _ = utils.DecodeBase64(authPart)
		}
		authPart = string(decodedAuth)
	}

	// 分离加密方法和密码
	authParts := strings.SplitN(authPart, ":", 2)
	if len(authParts) != 2 {
		return models.Proxy{}, errors.New("SS链接格式错误：无法分离加密方法和密码")
	}
	method = authParts[0]
	password = authParts[1]

	// 解析服务器信息
	serverPart := parts[1]
	serverParts := strings.SplitN(serverPart, ":", 2)
	if len(serverParts) != 2 {
		return models.Proxy{}, errors.New("SS链接格式错误：无法分离服务器地址和端口")
	}

	server := serverParts[0]
	portStr := strings.TrimRight(serverParts[1], "/")

	// 解析端口
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("端口解析失败: %v", err)
	}

	// 解析插件信息
	var plugin, pluginOpts string
	if pluginStr != "" {
		pluginParts := strings.SplitN(pluginStr, ";", 2)
		plugin = pluginParts[0]
		if len(pluginParts) > 1 {
			pluginOpts = pluginParts[1]
		}
	}

	// 构建代理对象
	proxy := models.Proxy{
		Type:       "ss",
		Server:     server,
		Port:       port,
		Method:     method,
		Password:   password,
		Plugin:     plugin,
		PluginOpts: pluginOpts,
	}

	// 设置节点名称
	if fragment != "" {
		proxy.Name = fragment
	} else {
		// 如果没有节点名称，使用服务器地址作为默认名称
		proxy.Name = server
	}

	// 验证必要字段
	if proxy.Method == "" || proxy.Password == "" || proxy.Server == "" || proxy.Port == 0 {
		return models.Proxy{}, errors.New("SS链接缺少必要字段")
	}

	return proxy, nil
}

// parseTrojanLink 解析Trojan链接
func parseTrojanLink(link string) (models.Proxy, error) {
	// 解析格式：trojan://password@host:port?sni=xxx&alpn=xxx&allowInsecure=1#name
	if len(link) < 10 {
		return models.Proxy{}, errors.New("Trojan链接格式错误：链接太短")
	}

	// 尝试使用标准URL解析
	u, err := url.Parse(link)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("URL解析错误: %v", err)
	}

	if u.Scheme != "trojan" {
		return models.Proxy{}, errors.New("Trojan链接格式错误：协议不是trojan")
	}

	// 检查必要参数
	if u.User == nil || u.User.Username() == "" {
		return models.Proxy{}, errors.New("Trojan链接格式错误：缺少密码")
	}

	if u.Host == "" {
		return models.Proxy{}, errors.New("Trojan链接格式错误：缺少主机地址")
	}

	// 解析端口
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		// 如果没有指定端口，使用默认端口443
		host = u.Host
		portStr = "443"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("端口解析失败: %v", err)
	}

	// 获取查询参数
	query := u.Query()

	// 获取节点名称（从fragment中）
	nodeName := u.Fragment
	if nodeName != "" {
		// URL解码节点名称
		if decodedName, err := url.QueryUnescape(nodeName); err == nil {
			nodeName = decodedName
		}
	} else {
		// 如果没有节点名称，使用服务器地址作为默认名称
		nodeName = host
	}

	// 创建代理对象
	proxy := models.Proxy{
		Type:     "trojan",
		Name:     nodeName,
		Server:   host,
		Port:     port,
		Password: u.User.Username(),
		SNI:      query.Get("sni"),
		TLS:      true,
	}

	// 处理ALPN参数
	if alpn := query.Get("alpn"); alpn != "" {
		proxy.ALPN = alpn
	} else if alpns := query.Get("alpns"); alpns != "" {
		// 有些实现使用alpns而不是alpn
		proxy.ALPN = alpns
	}

	// 如果SNI为空，但服务器地址不为IP，则使用服务器地址作为SNI
	if proxy.SNI == "" && !isIP(proxy.Server) {
		proxy.SNI = proxy.Server
	}

	// 处理allowInsecure参数
	allowInsecure := query.Get("allowInsecure")
	if allowInsecure == "1" || allowInsecure == "true" {
		proxy.AllowInsecure = true
	} else if skipVerify := query.Get("skip-cert-verify"); skipVerify == "1" || skipVerify == "true" {
		// 有些实现使用skip-cert-verify而不是allowInsecure
		proxy.AllowInsecure = true
	}

	// 处理其他可能的参数
	// 1. 处理流控参数
	if flow := query.Get("flow"); flow != "" {
		// 存储flow参数到RawConfig
		rawConfig := make(map[string]interface{})
		if proxy.RawConfig != "" {
			// 如果已有RawConfig，先解析
			if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
				rawConfig = make(map[string]interface{})
			}
		}
		rawConfig["flow"] = flow
		if jsonData, err := json.Marshal(rawConfig); err == nil {
			proxy.RawConfig = string(jsonData)
		}
	}

	// 2. 处理ws参数（某些Trojan实现支持WebSocket）
	if ws := query.Get("ws"); ws == "1" || ws == "true" {
		proxy.Network = "ws"
		if path := query.Get("path"); path != "" {
			proxy.Path = path
		}
		if host := query.Get("host"); host != "" {
			proxy.Host = host
		}
	}

	// 存储所有查询参数到RawConfig
	rawConfig := map[string]interface{}{
		"server":        proxy.Server,
		"port":          proxy.Port,
		"password":      proxy.Password,
		"sni":           proxy.SNI,
		"alpn":          proxy.ALPN,
		"allowInsecure": proxy.AllowInsecure,
	}

	// 添加其他可能的参数
	for key, values := range query {
		if len(values) > 0 && key != "sni" && key != "alpn" && key != "allowInsecure" &&
			key != "skip-cert-verify" && key != "ws" && key != "path" && key != "host" {
			rawConfig[key] = values[0]
		}
	}

	// 序列化为JSON
	if jsonData, err := json.Marshal(rawConfig); err == nil {
		proxy.RawConfig = string(jsonData)
	}

	return proxy, nil
}

// isIP 判断字符串是否为IP地址
func isIP(host string) bool {
	// 简单判断是否为IPv4地址
	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if num, err := strconv.Atoi(part); err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

// parseMixedSubscription 解析混合类型订阅
func parseMixedSubscription(content string) ([]models.Proxy, error) {
	// 混合类型订阅实际上就是自动检测并解析
	return autoDetectAndParse(content)
}

// parseSIP008Subscription 解析SIP008格式的Shadowsocks订阅
// SIP008格式是一种JSON格式的SS订阅标准
// 参考: https://shadowsocks.org/en/wiki/SIP008-Online-Configuration-Delivery.html
func parseSIP008Subscription(content string) ([]models.Proxy, error) {
	var sip008Config struct {
		Version int `json:"version"`
		Servers []struct {
			ID         string `json:"id"`
			Remarks    string `json:"remarks"`
			Server     string `json:"server"`
			ServerPort int    `json:"server_port"`
			Password   string `json:"password"`
			Method     string `json:"method"`
			Plugin     string `json:"plugin,omitempty"`
			PluginOpts string `json:"plugin_opts,omitempty"`
		} `json:"servers"`
	}

	if err := json.Unmarshal([]byte(content), &sip008Config); err != nil {
		return nil, fmt.Errorf("解析SIP008格式失败: %v", err)
	}

	var proxies []models.Proxy
	for _, server := range sip008Config.Servers {
		proxy := models.Proxy{
			Type:       "ss",
			Name:       server.Remarks,
			Server:     server.Server,
			Port:       server.ServerPort,
			Password:   server.Password,
			Method:     server.Method,
			Plugin:     server.Plugin,
			PluginOpts: server.PluginOpts,
		}

		// 如果没有设置名称，使用服务器地址作为默认名称
		if proxy.Name == "" {
			proxy.Name = server.Server
		}

		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// parseClashSubscription 解析Clash格式的订阅
func parseClashSubscription(content string) ([]models.Proxy, error) {
	// 使用简单的字符串处理方式解析YAML
	// 注意：这是一个简化的实现，实际应该使用YAML解析库
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	inProxies := false
	var currentProxy *models.Proxy

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 检测是否进入proxies部分
		if line == "proxies:" {
			inProxies = true
			continue
		}

		if !inProxies {
			continue
		}

		// 检测新的代理节点开始
		if strings.HasPrefix(line, "- name:") {
			// 保存之前的代理节点（如果有）
			if currentProxy != nil && currentProxy.Server != "" && currentProxy.Port != 0 {
				proxies = append(proxies, *currentProxy)
			}

			// 创建新的代理节点
			currentProxy = &models.Proxy{
				Name: strings.TrimSpace(strings.TrimPrefix(line, "- name:")),
			}
			continue
		}

		// 如果没有当前代理节点，跳过
		if currentProxy == nil {
			continue
		}

		// 解析代理节点的各个属性
		switch {
		case strings.HasPrefix(line, "type:"):
			currentProxy.Type = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
		case strings.HasPrefix(line, "server:"):
			currentProxy.Server = strings.TrimSpace(strings.TrimPrefix(line, "server:"))
		case strings.HasPrefix(line, "port:"):
			portStr := strings.TrimSpace(strings.TrimPrefix(line, "port:"))
			port, err := strconv.Atoi(portStr)
			if err == nil {
				currentProxy.Port = port
			}
		case strings.HasPrefix(line, "password:"):
			currentProxy.Password = strings.TrimSpace(strings.TrimPrefix(line, "password:"))
		case strings.HasPrefix(line, "cipher:") || strings.HasPrefix(line, "method:"):
			currentProxy.Method = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "cipher:"), "method:"))
		case strings.HasPrefix(line, "uuid:"):
			currentProxy.UUID = strings.TrimSpace(strings.TrimPrefix(line, "uuid:"))
		case strings.HasPrefix(line, "network:"):
			currentProxy.Network = strings.TrimSpace(strings.TrimPrefix(line, "network:"))
		case strings.HasPrefix(line, "tls:"):
			tlsStr := strings.TrimSpace(strings.TrimPrefix(line, "tls:"))
			currentProxy.TLS = tlsStr == "true"
		case strings.HasPrefix(line, "sni:"):
			currentProxy.SNI = strings.TrimSpace(strings.TrimPrefix(line, "sni:"))
		case strings.HasPrefix(line, "ws-path:") || strings.HasPrefix(line, "path:"):
			currentProxy.Path = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "ws-path:"), "path:"))
		case strings.HasPrefix(line, "plugin:"):
			currentProxy.Plugin = strings.TrimSpace(strings.TrimPrefix(line, "plugin:"))
		case strings.HasPrefix(line, "skip-cert-verify:"):
			skipStr := strings.TrimSpace(strings.TrimPrefix(line, "skip-cert-verify:"))
			currentProxy.AllowInsecure = skipStr == "true"
		case strings.HasPrefix(line, "username:"):
			// 存储用户名到RawConfig
			username := strings.TrimSpace(strings.TrimPrefix(line, "username:"))
			rawConfig := make(map[string]interface{})
			if currentProxy.RawConfig != "" {
				// 如果已有RawConfig，先解析
				if err := json.Unmarshal([]byte(currentProxy.RawConfig), &rawConfig); err != nil {
					rawConfig = make(map[string]interface{})
				}
			}
			rawConfig["username"] = username
			if jsonData, err := json.Marshal(rawConfig); err == nil {
				currentProxy.RawConfig = string(jsonData)
			}
		}
	}

	// 保存最后一个代理节点（如果有）
	if currentProxy != nil && currentProxy.Server != "" && currentProxy.Port != 0 {
		proxies = append(proxies, *currentProxy)
	}

	return proxies, nil
}

// parseSurgeSubscription 解析Surge格式的订阅
func parseSurgeSubscription(content string) ([]models.Proxy, error) {
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行、注释和节段标记
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}

		// 解析代理行
		// 格式: ProxyName = ProxyType, Server, Port, Param1=Value1, Param2=Value2, ...
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		params := strings.Split(parts[1], ",")
		if len(params) < 3 {
			continue
		}

		proxyType := strings.TrimSpace(params[0])
		server := strings.TrimSpace(params[1])
		portStr := strings.TrimSpace(params[2])
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		// 创建基本代理对象
		proxy := models.Proxy{
			Name:   name,
			Server: server,
			Port:   port,
		}

		// 根据代理类型设置不同的参数
		switch strings.ToLower(proxyType) {
		case "ss", "shadowsocks":
			proxy.Type = "ss"
			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "method", "encrypt-method":
					proxy.Method = value
				case "password":
					proxy.Password = value
				case "plugin":
					proxy.Plugin = value
				case "plugin-opts":
					proxy.PluginOpts = value
				}
			}

		case "vmess":
			proxy.Type = "vmess"
			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "username", "uuid":
					proxy.UUID = value
				case "ws", "network":
					proxy.Network = value
				case "tls":
					proxy.TLS = value == "true" || value == "1"
				case "ws-path", "path":
					proxy.Path = value
				case "ws-headers", "host":
					proxy.Host = value
				case "skip-cert-verify":
					proxy.AllowInsecure = value == "true" || value == "1"
				}
			}

		case "trojan":
			proxy.Type = "trojan"
			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "password":
					proxy.Password = value
				case "sni":
					proxy.SNI = value
				case "alpn":
					proxy.ALPN = value
				case "skip-cert-verify":
					proxy.AllowInsecure = value == "true" || value == "1"
				}
			}

		case "http", "https":
			// 处理HTTP/HTTPS代理
			proxy.Type = "http"
			// 如果是HTTPS，设置TLS为true
			if strings.ToLower(proxyType) == "https" {
				proxy.TLS = true
			}

			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "username":
					// 存储用户名到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["username"] = value
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				case "password":
					proxy.Password = value
				case "tls":
					proxy.TLS = value == "true" || value == "1"
				case "sni":
					proxy.SNI = value
				case "skip-cert-verify":
					proxy.AllowInsecure = value == "true" || value == "1"
				}
			}
		case "socks", "socks5":
			// 处理SOCKS代理
			proxy.Type = "socks"

			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "username":
					// 存储用户名到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["username"] = value
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				case "password":
					proxy.Password = value
				case "tls":
					proxy.TLS = value == "true" || value == "1"
				case "skip-cert-verify":
					proxy.AllowInsecure = value == "true" || value == "1"
				case "udp":
					// 存储UDP支持到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["udp"] = value == "true" || value == "1"
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				}
			}
		}

		// 只添加有效的代理
		if (proxy.Type == "ss" && proxy.Method != "" && proxy.Password != "") ||
			(proxy.Type == "vmess" && proxy.UUID != "") ||
			(proxy.Type == "trojan" && proxy.Password != "") ||
			(proxy.Type == "http") ||
			(proxy.Type == "socks") {
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// parseQuantumultSubscription 解析Quantumult格式的订阅
func parseQuantumultSubscription(content string) ([]models.Proxy, error) {
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 检查是否为URI格式
		if strings.HasPrefix(line, "vmess://") ||
			strings.HasPrefix(line, "ss://") ||
			strings.HasPrefix(line, "trojan://") {
			// 使用已有的解析函数
			proxy, err := parseProxyURI(line)
			if err == nil {
				proxies = append(proxies, proxy)
			}
			continue
		}

		// 解析Quantumult特有格式
		// 格式: ProxyType = ProxyName, Server, Port, Param1=Value1, Param2=Value2, tag=ProxyName
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		proxyType := strings.TrimSpace(parts[0])
		params := strings.Split(parts[1], ",")
		if len(params) < 3 {
			continue
		}

		name := strings.TrimSpace(params[0])
		server := strings.TrimSpace(params[1])
		portStr := strings.TrimSpace(params[2])
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		// 创建基本代理对象
		proxy := models.Proxy{
			Name:   name,
			Server: server,
			Port:   port,
		}

		// 根据代理类型设置不同的参数
		switch strings.ToLower(proxyType) {
		case "shadowsocks":
			proxy.Type = "ss"
			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "method":
					proxy.Method = value
				case "password":
					proxy.Password = value
				case "obfs":
					proxy.Plugin = "obfs"
					proxy.PluginOpts = "obfs=" + value
				case "obfs-host":
					if proxy.PluginOpts != "" {
						proxy.PluginOpts += ";obfs-host=" + value
					} else {
						proxy.PluginOpts = "obfs-host=" + value
					}
				}
			}

		case "vmess":
			proxy.Type = "vmess"
			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "method":
					// Quantumult中的method对应VMess的加密方式，但我们的模型中没有对应字段
				case "password":
					proxy.UUID = value
				case "obfs":
					proxy.Network = value
				case "obfs-host":
					proxy.Host = value
				case "obfs-path":
					proxy.Path = value
				case "over-tls":
					proxy.TLS = value == "true"
				case "tls-host":
					proxy.SNI = value
				case "skip-cert-verify":
					proxy.AllowInsecure = value == "true"
				}
			}

		case "http", "https":
			// 处理HTTP/HTTPS代理
			proxy.Type = "http"
			// 如果是HTTPS，设置TLS为true
			if strings.ToLower(proxyType) == "https" {
				proxy.TLS = true
			}

			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "username":
					// 存储用户名到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["username"] = value
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				case "password":
					proxy.Password = value
				case "over-tls":
					proxy.TLS = value == "true"
				case "tls-host":
					proxy.SNI = value
				case "skip-cert-verify":
					proxy.AllowInsecure = value == "true"
				}
			}
		case "socks", "socks5":
			// 处理SOCKS代理
			proxy.Type = "socks"

			// 解析其他参数
			for i := 3; i < len(params); i++ {
				kv := strings.SplitN(strings.TrimSpace(params[i]), "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])

				switch key {
				case "username":
					// 存储用户名到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["username"] = value
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				case "password":
					proxy.Password = value
				case "tls":
					proxy.TLS = value == "true" || value == "1"
				case "skip-cert-verify":
					proxy.AllowInsecure = value == "true" || value == "1"
				case "udp":
					// 存储UDP支持到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["udp"] = value == "true" || value == "1"
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				}
			}
		}

		// 只添加有效的代理
		if (proxy.Type == "ss" && proxy.Method != "" && proxy.Password != "") ||
			(proxy.Type == "vmess" && proxy.UUID != "") ||
			(proxy.Type == "trojan" && proxy.Password != "") ||
			(proxy.Type == "http") ||
			(proxy.Type == "socks") {
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// parseJSONSubscription 解析JSON格式的订阅
func parseJSONSubscription(content string) ([]models.Proxy, error) {
	// 尝试解析为SIP008格式
	sip008Proxies, err := parseSIP008Subscription(content)
	if err == nil && len(sip008Proxies) > 0 {
		return sip008Proxies, nil
	}

	// 尝试解析为通用JSON数组格式
	var jsonProxies []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &jsonProxies); err == nil {
		var proxies []models.Proxy

		for _, item := range jsonProxies {
			proxy := models.Proxy{}

			// 提取基本字段
			if name, ok := item["name"].(string); ok {
				proxy.Name = name
			} else if remarks, ok := item["remarks"].(string); ok {
				proxy.Name = remarks
			}

			if server, ok := item["server"].(string); ok {
				proxy.Server = server
			}

			// 提取端口（可能是数字或字符串）
			if port, ok := item["port"].(float64); ok {
				proxy.Port = int(port)
			} else if portStr, ok := item["port"].(string); ok {
				if port, err := strconv.Atoi(portStr); err == nil {
					proxy.Port = port
				}
			} else if serverPort, ok := item["server_port"].(float64); ok {
				proxy.Port = int(serverPort)
			}

			// 提取类型
			if proxyType, ok := item["type"].(string); ok {
				proxy.Type = proxyType
			}

			// 根据类型提取特定字段
			switch proxy.Type {
			case "ss", "shadowsocks":
				proxy.Type = "ss"
				if method, ok := item["method"].(string); ok {
					proxy.Method = method
				} else if cipher, ok := item["cipher"].(string); ok {
					proxy.Method = cipher
				}

				if password, ok := item["password"].(string); ok {
					proxy.Password = password
				}

				if plugin, ok := item["plugin"].(string); ok {
					proxy.Plugin = plugin
				}

				if pluginOpts, ok := item["plugin_opts"].(string); ok {
					proxy.PluginOpts = pluginOpts
				} else if pluginOpts, ok := item["plugin-opts"].(map[string]interface{}); ok {
					// 将插件选项转换为字符串
					var opts []string
					for k, v := range pluginOpts {
						opts = append(opts, k+"="+fmt.Sprint(v))
					}
					proxy.PluginOpts = strings.Join(opts, ";")
				}

			case "vmess":
				if uuid, ok := item["uuid"].(string); ok {
					proxy.UUID = uuid
				} else if id, ok := item["id"].(string); ok {
					proxy.UUID = id
				}

				if network, ok := item["network"].(string); ok {
					proxy.Network = network
				} else if net, ok := item["net"].(string); ok {
					proxy.Network = net
				}

				if tls, ok := item["tls"].(bool); ok {
					proxy.TLS = tls
				} else if tlsStr, ok := item["tls"].(string); ok {
					proxy.TLS = tlsStr == "tls" || tlsStr == "true"
				}

				if path, ok := item["path"].(string); ok {
					proxy.Path = path
				} else if wsPath, ok := item["ws-path"].(string); ok {
					proxy.Path = wsPath
				}

				if host, ok := item["host"].(string); ok {
					proxy.Host = host
				} else if wsHeaders, ok := item["ws-headers"].(map[string]interface{}); ok {
					if host, ok := wsHeaders["Host"].(string); ok {
						proxy.Host = host
					}
				}

			case "trojan":
				if password, ok := item["password"].(string); ok {
					proxy.Password = password
				}

				if sni, ok := item["sni"].(string); ok {
					proxy.SNI = sni
				}

				if alpn, ok := item["alpn"].(string); ok {
					proxy.ALPN = alpn
				} else if alpnList, ok := item["alpn"].([]interface{}); ok {
					var alpns []string
					for _, a := range alpnList {
						if alpnStr, ok := a.(string); ok {
							alpns = append(alpns, alpnStr)
						}
					}
					proxy.ALPN = strings.Join(alpns, ",")
				}

				if skipVerify, ok := item["skip-cert-verify"].(bool); ok {
					proxy.AllowInsecure = skipVerify
				} else if allowInsecure, ok := item["allowInsecure"].(bool); ok {
					proxy.AllowInsecure = allowInsecure
				}
			case "http", "https":
				proxy.Type = "http"

				// 如果是HTTPS，设置TLS为true
				if proxy.Type == "https" {
					proxy.TLS = true
				}

				// 处理TLS参数
				if tls, ok := item["tls"].(bool); ok {
					proxy.TLS = tls
				} else if tlsStr, ok := item["tls"].(string); ok {
					proxy.TLS = tlsStr == "true" || tlsStr == "1"
				}

				// 处理用户名和密码
				if username, ok := item["username"].(string); ok {
					// 存储用户名到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["username"] = username
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				}

				if password, ok := item["password"].(string); ok {
					proxy.Password = password
				}

				// 处理SNI参数
				if sni, ok := item["sni"].(string); ok {
					proxy.SNI = sni
				}

				// 处理skip-cert-verify参数
				if skipVerify, ok := item["skip-cert-verify"].(bool); ok {
					proxy.AllowInsecure = skipVerify
				} else if allowInsecure, ok := item["allowInsecure"].(bool); ok {
					proxy.AllowInsecure = allowInsecure
				}
			case "socks", "socks5":
				// 处理SOCKS代理
				proxy.Type = "socks"

				// 处理用户名和密码
				if username, ok := item["username"].(string); ok {
					// 存储用户名到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["username"] = username
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				}

				if password, ok := item["password"].(string); ok {
					proxy.Password = password
				}

				// 处理TLS参数
				if tls, ok := item["tls"].(bool); ok {
					proxy.TLS = tls
				} else if tlsStr, ok := item["tls"].(string); ok {
					proxy.TLS = tlsStr == "true" || tlsStr == "1"
				}

				// 处理skip-cert-verify参数
				if skipVerify, ok := item["skip-cert-verify"].(bool); ok {
					proxy.AllowInsecure = skipVerify
				} else if allowInsecure, ok := item["allowInsecure"].(bool); ok {
					proxy.AllowInsecure = allowInsecure
				}

				// 处理UDP支持
				if udp, ok := item["udp"].(bool); ok {
					// 存储到RawConfig
					rawConfig := make(map[string]interface{})
					if proxy.RawConfig != "" {
						// 如果已有RawConfig，先解析
						if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
							rawConfig = make(map[string]interface{})
						}
					}
					rawConfig["udp"] = udp
					if jsonData, err := json.Marshal(rawConfig); err == nil {
						proxy.RawConfig = string(jsonData)
					}
				}
			}

			// 只添加有效的代理
			if proxy.Server != "" && proxy.Port != 0 &&
				((proxy.Type == "ss" && proxy.Method != "" && proxy.Password != "") ||
					(proxy.Type == "vmess" && proxy.UUID != "") ||
					(proxy.Type == "trojan" && proxy.Password != "") ||
					(proxy.Type == "http") ||
					(proxy.Type == "socks")) {

				// 如果没有名称，使用服务器地址作为默认名称
				if proxy.Name == "" {
					proxy.Name = proxy.Server
				}

				proxies = append(proxies, proxy)
			}
		}

		if len(proxies) > 0 {
			return proxies, nil
		}
	}

	// 尝试解析为单个JSON对象
	var singleProxy map[string]interface{}
	if err := json.Unmarshal([]byte(content), &singleProxy); err == nil {
		// 检查是否包含代理数组
		if proxyList, ok := singleProxy["proxies"].([]interface{}); ok {
			var proxies []models.Proxy

			for _, item := range proxyList {
				if proxyMap, ok := item.(map[string]interface{}); ok {
					proxy := parseJSONProxy(proxyMap)
					if isValidProxy(proxy) {
						proxies = append(proxies, proxy)
					}
				}
			}

			if len(proxies) > 0 {
				return proxies, nil
			}
		}
	}

	return nil, errors.New("无法解析JSON格式的订阅")
}

// parseProxyURI 解析代理URI
func parseProxyURI(uri string) (models.Proxy, error) {
	switch {
	case strings.HasPrefix(uri, "vmess://"):
		return parseVmessLink(uri)
	case strings.HasPrefix(uri, "ss://"):
		return parseSSLink(uri)
	case strings.HasPrefix(uri, "trojan://"):
		return parseTrojanLink(uri)
	case strings.HasPrefix(uri, "ssr://"):
		return parseSSRLink(uri)
	case strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://"):
		return parseHTTPLink(uri)
	case strings.HasPrefix(uri, "socks://") || strings.HasPrefix(uri, "socks5://"):
		return parseSOCKSLink(uri)
	default:
		return models.Proxy{}, errors.New("不支持的代理URI格式")
	}
}

// parseJSONProxy 从JSON对象解析代理
func parseJSONProxy(item map[string]interface{}) models.Proxy {
	proxy := models.Proxy{}

	// 提取基本字段
	if name, ok := item["name"].(string); ok {
		proxy.Name = name
	} else if remarks, ok := item["remarks"].(string); ok {
		proxy.Name = remarks
	}

	if server, ok := item["server"].(string); ok {
		proxy.Server = server
	}

	// 提取端口（可能是数字或字符串）
	if port, ok := item["port"].(float64); ok {
		proxy.Port = int(port)
	} else if portStr, ok := item["port"].(string); ok {
		if port, err := strconv.Atoi(portStr); err == nil {
			proxy.Port = port
		}
	} else if serverPort, ok := item["server_port"].(float64); ok {
		proxy.Port = int(serverPort)
	}

	// 提取类型
	if proxyType, ok := item["type"].(string); ok {
		proxy.Type = proxyType
	}

	// 根据类型提取特定字段
	switch proxy.Type {
	case "ss", "shadowsocks":
		proxy.Type = "ss"
		if method, ok := item["method"].(string); ok {
			proxy.Method = method
		} else if cipher, ok := item["cipher"].(string); ok {
			proxy.Method = cipher
		}

		if password, ok := item["password"].(string); ok {
			proxy.Password = password
		}

		if plugin, ok := item["plugin"].(string); ok {
			proxy.Plugin = plugin
		}

		if pluginOpts, ok := item["plugin_opts"].(string); ok {
			proxy.PluginOpts = pluginOpts
		} else if pluginOpts, ok := item["plugin-opts"].(map[string]interface{}); ok {
			// 将插件选项转换为字符串
			var opts []string
			for k, v := range pluginOpts {
				opts = append(opts, k+"="+fmt.Sprint(v))
			}
			proxy.PluginOpts = strings.Join(opts, ";")
		}

	case "vmess":
		if uuid, ok := item["uuid"].(string); ok {
			proxy.UUID = uuid
		} else if id, ok := item["id"].(string); ok {
			proxy.UUID = id
		}

		if network, ok := item["network"].(string); ok {
			proxy.Network = network
		} else if net, ok := item["net"].(string); ok {
			proxy.Network = net
		}

		if tls, ok := item["tls"].(bool); ok {
			proxy.TLS = tls
		} else if tlsStr, ok := item["tls"].(string); ok {
			proxy.TLS = tlsStr == "tls" || tlsStr == "true"
		}

		if path, ok := item["path"].(string); ok {
			proxy.Path = path
		} else if wsPath, ok := item["ws-path"].(string); ok {
			proxy.Path = wsPath
		}

		if host, ok := item["host"].(string); ok {
			proxy.Host = host
		} else if wsHeaders, ok := item["ws-headers"].(map[string]interface{}); ok {
			if host, ok := wsHeaders["Host"].(string); ok {
				proxy.Host = host
			}
		}

	case "trojan":
		if password, ok := item["password"].(string); ok {
			proxy.Password = password
		}

		if sni, ok := item["sni"].(string); ok {
			proxy.SNI = sni
		}

		if alpn, ok := item["alpn"].(string); ok {
			proxy.ALPN = alpn
		} else if alpnList, ok := item["alpn"].([]interface{}); ok {
			var alpns []string
			for _, a := range alpnList {
				if alpnStr, ok := a.(string); ok {
					alpns = append(alpns, alpnStr)
				}
			}
			proxy.ALPN = strings.Join(alpns, ",")
		}

		if skipVerify, ok := item["skip-cert-verify"].(bool); ok {
			proxy.AllowInsecure = skipVerify
		} else if allowInsecure, ok := item["allowInsecure"].(bool); ok {
			proxy.AllowInsecure = allowInsecure
		}
	case "http", "https":
		proxy.Type = "http"

		// 如果是HTTPS，设置TLS为true
		if proxy.Type == "https" {
			proxy.TLS = true
		}

		// 处理TLS参数
		if tls, ok := item["tls"].(bool); ok {
			proxy.TLS = tls
		} else if tlsStr, ok := item["tls"].(string); ok {
			proxy.TLS = tlsStr == "true" || tlsStr == "1"
		}

		// 处理用户名和密码
		if username, ok := item["username"].(string); ok {
			// 存储用户名到RawConfig
			rawConfig := make(map[string]interface{})
			if proxy.RawConfig != "" {
				// 如果已有RawConfig，先解析
				if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
					rawConfig = make(map[string]interface{})
				}
			}
			rawConfig["username"] = username
			if jsonData, err := json.Marshal(rawConfig); err == nil {
				proxy.RawConfig = string(jsonData)
			}
		}

		if password, ok := item["password"].(string); ok {
			proxy.Password = password
		}

		// 处理SNI参数
		if sni, ok := item["sni"].(string); ok {
			proxy.SNI = sni
		}

		// 处理skip-cert-verify参数
		if skipVerify, ok := item["skip-cert-verify"].(bool); ok {
			proxy.AllowInsecure = skipVerify
		} else if allowInsecure, ok := item["allowInsecure"].(bool); ok {
			proxy.AllowInsecure = allowInsecure
		}
	case "socks", "socks5":
		// 处理SOCKS代理
		proxy.Type = "socks"

		// 处理用户名和密码
		if username, ok := item["username"].(string); ok {
			// 存储用户名到RawConfig
			rawConfig := make(map[string]interface{})
			if proxy.RawConfig != "" {
				// 如果已有RawConfig，先解析
				if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
					rawConfig = make(map[string]interface{})
				}
			}
			rawConfig["username"] = username
			if jsonData, err := json.Marshal(rawConfig); err == nil {
				proxy.RawConfig = string(jsonData)
			}
		}

		if password, ok := item["password"].(string); ok {
			proxy.Password = password
		}

		// 处理TLS参数
		if tls, ok := item["tls"].(bool); ok {
			proxy.TLS = tls
		} else if tlsStr, ok := item["tls"].(string); ok {
			proxy.TLS = tlsStr == "true" || tlsStr == "1"
		}

		// 处理skip-cert-verify参数
		if skipVerify, ok := item["skip-cert-verify"].(bool); ok {
			proxy.AllowInsecure = skipVerify
		} else if allowInsecure, ok := item["allowInsecure"].(bool); ok {
			proxy.AllowInsecure = allowInsecure
		}

		// 处理UDP支持
		if udp, ok := item["udp"].(bool); ok {
			// 存储到RawConfig
			rawConfig := make(map[string]interface{})
			if proxy.RawConfig != "" {
				// 如果已有RawConfig，先解析
				if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
					rawConfig = make(map[string]interface{})
				}
			}
			rawConfig["udp"] = udp
			if jsonData, err := json.Marshal(rawConfig); err == nil {
				proxy.RawConfig = string(jsonData)
			}
		}
	}

	// 如果没有名称，使用服务器地址作为默认名称
	if proxy.Name == "" {
		proxy.Name = proxy.Server
	}

	return proxy
}

// isValidProxy 检查代理是否有效
func isValidProxy(proxy models.Proxy) bool {
	if proxy.Server == "" || proxy.Port == 0 {
		return false
	}

	switch proxy.Type {
	case "ss":
		return proxy.Method != "" && proxy.Password != ""
	case "vmess":
		return proxy.UUID != ""
	case "trojan":
		return proxy.Password != ""
	case "http":
		return true
	case "socks":
		return true
	default:
		return false
	}
}

// parseSSRLink 解析SSR链接
func parseSSRLink(link string) (models.Proxy, error) {
	// 解析格式：ssr://base64(server:port:protocol:method:obfs:base64(password)/?obfsparam=base64(obfsparam)&protoparam=base64(protoparam)&remarks=base64(remarks))
	if len(link) < 7 {
		return models.Proxy{}, errors.New("SSR链接格式错误：链接太短")
	}

	// 移除ssr://前缀
	ssrURL := link[6:]

	// Base64解码
	decodedBytes, err := base64.RawURLEncoding.DecodeString(ssrURL)
	if err != nil {
		// 尝试标准base64解码
		decodedBytes, _ = utils.DecodeBase64(ssrURL)
	}

	decoded := string(decodedBytes)

	// 分离参数部分
	var mainPart string
	var paramsPart string

	if idx := strings.Index(decoded, "/?"); idx >= 0 {
		mainPart = decoded[:idx]
		paramsPart = decoded[idx+2:]
	} else if idx := strings.Index(decoded, "?"); idx >= 0 {
		mainPart = decoded[:idx]
		paramsPart = decoded[idx+1:]
	} else {
		mainPart = decoded
	}

	// 解析主要部分
	parts := strings.Split(mainPart, ":")
	if len(parts) < 6 {
		return models.Proxy{}, errors.New("SSR链接格式错误：主要部分不完整")
	}

	server := parts[0]
	portStr := parts[1]
	protocol := parts[2]
	method := parts[3]
	obfs := parts[4]

	// 解析密码（Base64编码）
	passwordBase64 := parts[5]
	passwordBytes, err := base64.RawURLEncoding.DecodeString(passwordBase64)
	if err != nil {
		// 尝试标准base64解码
		passwordBytes, _ = utils.DecodeBase64(passwordBase64)
	}
	password := string(passwordBytes)

	// 解析端口
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("端口解析失败: %v", err)
	}

	// 创建代理对象
	proxy := models.Proxy{
		Type:     "ssr",
		Server:   server,
		Port:     port,
		Method:   method,
		Password: password,
	}

	// 解析参数部分
	if paramsPart != "" {
		params := strings.Split(paramsPart, "&")
		for _, param := range params {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) != 2 {
				continue
			}

			key := kv[0]
			value := kv[1]

			// 解码参数值（Base64编码）
			valueBytes, err := base64.RawURLEncoding.DecodeString(value)
			if err != nil {
				// 尝试标准base64解码
				valueBytes, _ = utils.DecodeBase64(value)
			}
			decodedValue := string(valueBytes)

			switch key {
			case "remarks":
				proxy.Name = decodedValue
			case "obfsparam":
				// 存储到RawConfig
				rawConfig := make(map[string]interface{})
				if proxy.RawConfig != "" {
					// 如果已有RawConfig，先解析
					if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
						rawConfig = make(map[string]interface{})
					}
				}
				rawConfig["obfsparam"] = decodedValue
				if jsonData, err := json.Marshal(rawConfig); err == nil {
					proxy.RawConfig = string(jsonData)
				}
			case "protoparam":
				// 存储到RawConfig
				rawConfig := make(map[string]interface{})
				if proxy.RawConfig != "" {
					// 如果已有RawConfig，先解析
					if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
						rawConfig = make(map[string]interface{})
					}
				}
				rawConfig["protoparam"] = decodedValue
				if jsonData, err := json.Marshal(rawConfig); err == nil {
					proxy.RawConfig = string(jsonData)
				}
			}
		}
	}

	// 如果没有设置名称，使用服务器地址作为默认名称
	if proxy.Name == "" {
		proxy.Name = server
	}

	// 存储SSR特有参数到RawConfig
	rawConfig := make(map[string]interface{})
	if proxy.RawConfig != "" {
		// 如果已有RawConfig，先解析
		if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
			rawConfig = make(map[string]interface{})
		}
	}

	rawConfig["protocol"] = protocol
	rawConfig["obfs"] = obfs

	if jsonData, err := json.Marshal(rawConfig); err == nil {
		proxy.RawConfig = string(jsonData)
	}

	return proxy, nil
}

// parseHTTPLink 解析HTTP/HTTPS代理链接
func parseHTTPLink(link string) (models.Proxy, error) {
	// 解析格式：http(s)://[username:password@]host:port[/?[tls=true][&skip-cert-verify=true]]#name

	// 尝试使用标准URL解析
	u, err := url.Parse(link)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("URL解析错误: %v", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return models.Proxy{}, errors.New("HTTP链接格式错误：协议不是http或https")
	}

	if u.Host == "" {
		return models.Proxy{}, errors.New("HTTP链接格式错误：缺少主机地址")
	}

	// 解析端口
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		// 如果没有指定端口，根据协议使用默认端口
		host = u.Host
		if u.Scheme == "http" {
			portStr = "80"
		} else {
			portStr = "443"
		}
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("端口解析失败: %v", err)
	}

	// 获取查询参数
	query := u.Query()

	// 获取节点名称（从fragment中）
	nodeName := u.Fragment
	if nodeName != "" {
		// URL解码节点名称
		if decodedName, err := url.QueryUnescape(nodeName); err == nil {
			nodeName = decodedName
		}
	} else {
		// 如果没有节点名称，使用服务器地址作为默认名称
		nodeName = host
	}

	// 创建代理对象
	proxy := models.Proxy{
		Type:   "http",
		Name:   nodeName,
		Server: host,
		Port:   port,
	}

	// 如果是HTTPS，设置TLS为true
	if u.Scheme == "https" {
		proxy.TLS = true
	}

	// 处理用户名和密码
	if u.User != nil {
		username := u.User.Username()
		password, hasPassword := u.User.Password()

		// 将用户名存储到RawConfig
		rawConfig := make(map[string]interface{})
		if proxy.RawConfig != "" {
			// 如果已有RawConfig，先解析
			if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
				rawConfig = make(map[string]interface{})
			}
		}
		rawConfig["username"] = username

		// 设置密码
		if hasPassword {
			proxy.Password = password
		} else {
			// 如果URL中只有用户名没有密码，则用户名可能是编码后的用户名:密码
			parts := strings.SplitN(username, ":", 2)
			if len(parts) == 2 {
				rawConfig["username"] = parts[0]
				proxy.Password = parts[1]
			} else {
				// 如果无法分离，则将整个用户部分作为密码
				proxy.Password = username
			}
		}

		// 更新RawConfig
		if jsonData, err := json.Marshal(rawConfig); err == nil {
			proxy.RawConfig = string(jsonData)
		}
	}

	// 处理TLS参数
	if tlsParam := query.Get("tls"); tlsParam == "true" || tlsParam == "1" {
		proxy.TLS = true
	}

	// 处理skip-cert-verify参数
	if skipVerify := query.Get("skip-cert-verify"); skipVerify == "true" || skipVerify == "1" {
		proxy.AllowInsecure = true
	} else if insecure := query.Get("allowInsecure"); insecure == "true" || insecure == "1" {
		// 有些实现使用allowInsecure而不是skip-cert-verify
		proxy.AllowInsecure = true
	}

	// 处理SNI参数
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	} else if host := query.Get("host"); host != "" {
		// 有些实现使用host作为SNI
		proxy.SNI = host
	} else if serverName := query.Get("servername"); serverName != "" {
		// 有些实现使用servername作为SNI
		proxy.SNI = serverName
	}

	// 如果SNI为空，但服务器地址不为IP，则使用服务器地址作为SNI
	if proxy.TLS && proxy.SNI == "" && !isIP(proxy.Server) {
		proxy.SNI = proxy.Server
	}

	// 处理path参数
	if path := query.Get("path"); path != "" {
		proxy.Path = path
	}

	// 处理ws参数（某些HTTP实现支持WebSocket）
	if ws := query.Get("ws"); ws == "1" || ws == "true" {
		proxy.Network = "ws"
		// 如果已经设置了path，则不覆盖
		if proxy.Path == "" {
			if wsPath := query.Get("ws-path"); wsPath != "" {
				proxy.Path = wsPath
			}
		}
		// 如果已经设置了host，则不覆盖
		if proxy.Host == "" {
			if wsHost := query.Get("ws-host"); wsHost != "" {
				proxy.Host = wsHost
			}
		}
	}

	// 存储所有查询参数到RawConfig
	rawConfig := map[string]interface{}{
		"server":        proxy.Server,
		"port":          proxy.Port,
		"tls":           proxy.TLS,
		"allowInsecure": proxy.AllowInsecure,
		"sni":           proxy.SNI,
	}

	// 添加用户名密码（如果有）
	if u.User != nil {
		username := u.User.Username()
		password, _ := u.User.Password()
		rawConfig["username"] = username
		rawConfig["password"] = password
	}

	// 添加其他可能的参数
	for key, values := range query {
		if len(values) > 0 && key != "tls" && key != "skip-cert-verify" &&
			key != "sni" && key != "host" && key != "servername" &&
			key != "ws" && key != "ws-path" && key != "ws-host" &&
			key != "path" && key != "allowInsecure" {
			rawConfig[key] = values[0]
		}
	}

	// 序列化为JSON
	if jsonData, err := json.Marshal(rawConfig); err == nil {
		proxy.RawConfig = string(jsonData)
	}

	// 验证必要字段
	if proxy.Server == "" || proxy.Port == 0 {
		return models.Proxy{}, errors.New("HTTP链接缺少必要字段")
	}

	return proxy, nil
}

// parseSOCKSLink 解析SOCKS代理链接
func parseSOCKSLink(link string) (models.Proxy, error) {
	// 解析格式：socks(5)://[username:password@]host:port[/?skip-cert-verify=true]#name

	// 尝试使用标准URL解析
	u, err := url.Parse(link)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("URL解析错误: %v", err)
	}

	if u.Scheme != "socks" && u.Scheme != "socks5" {
		return models.Proxy{}, errors.New("SOCKS链接格式错误：协议不是socks或socks5")
	}

	if u.Host == "" {
		return models.Proxy{}, errors.New("SOCKS链接格式错误：缺少主机地址")
	}

	// 解析端口
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		// 如果没有指定端口，使用默认端口1080
		host = u.Host
		portStr = "1080"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("端口解析失败: %v", err)
	}

	// 获取查询参数
	query := u.Query()

	// 获取节点名称（从fragment中）
	nodeName := u.Fragment
	if nodeName != "" {
		// URL解码节点名称
		if decodedName, err := url.QueryUnescape(nodeName); err == nil {
			nodeName = decodedName
		}
	} else {
		// 如果没有节点名称，使用服务器地址作为默认名称
		nodeName = host
	}

	// 创建代理对象
	proxy := models.Proxy{
		Type:   "socks",
		Name:   nodeName,
		Server: host,
		Port:   port,
	}

	// 处理用户名和密码
	if u.User != nil {
		username := u.User.Username()
		password, hasPassword := u.User.Password()

		// 将用户名存储到RawConfig
		rawConfig := make(map[string]interface{})
		if proxy.RawConfig != "" {
			// 如果已有RawConfig，先解析
			if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
				rawConfig = make(map[string]interface{})
			}
		}
		rawConfig["username"] = username

		// 设置密码
		if hasPassword {
			proxy.Password = password
		} else {
			// 如果URL中只有用户名没有密码，则用户名可能是编码后的用户名:密码
			parts := strings.SplitN(username, ":", 2)
			if len(parts) == 2 {
				rawConfig["username"] = parts[0]
				proxy.Password = parts[1]
			} else {
				// 如果无法分离，则将整个用户部分作为密码
				proxy.Password = username
			}
		}

		// 更新RawConfig
		if jsonData, err := json.Marshal(rawConfig); err == nil {
			proxy.RawConfig = string(jsonData)
		}
	}

	// 处理TLS参数
	if tlsParam := query.Get("tls"); tlsParam == "true" || tlsParam == "1" {
		proxy.TLS = true
	}

	// 处理skip-cert-verify参数
	if skipVerify := query.Get("skip-cert-verify"); skipVerify == "true" || skipVerify == "1" {
		proxy.AllowInsecure = true
	} else if insecure := query.Get("allowInsecure"); insecure == "true" || insecure == "1" {
		// 有些实现使用allowInsecure而不是skip-cert-verify
		proxy.AllowInsecure = true
	}

	// 处理SNI参数
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	}

	// 处理UDP参数
	udpParam := query.Get("udp")

	// 存储所有查询参数到RawConfig
	rawConfig := map[string]interface{}{
		"server":        proxy.Server,
		"port":          proxy.Port,
		"tls":           proxy.TLS,
		"allowInsecure": proxy.AllowInsecure,
		"sni":           proxy.SNI,
	}

	// 添加UDP支持参数
	if udpParam == "true" || udpParam == "1" {
		rawConfig["udp"] = true
	}

	// 添加用户名密码（如果有）
	if u.User != nil {
		username := u.User.Username()
		password, _ := u.User.Password()
		rawConfig["username"] = username
		rawConfig["password"] = password
	}

	// 添加其他可能的参数
	for key, values := range query {
		if len(values) > 0 && key != "tls" && key != "skip-cert-verify" &&
			key != "sni" && key != "allowInsecure" && key != "udp" {
			rawConfig[key] = values[0]
		}
	}

	// 序列化为JSON
	if jsonData, err := json.Marshal(rawConfig); err == nil {
		proxy.RawConfig = string(jsonData)
	}

	// 验证必要字段
	if proxy.Server == "" || proxy.Port == 0 {
		return models.Proxy{}, errors.New("SOCKS链接缺少必要字段")
	}

	return proxy, nil
}
