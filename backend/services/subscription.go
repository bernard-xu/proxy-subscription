package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if utils.IsBase64(content) {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, err
		}
		content = string(decoded)
	}

	// 根据订阅类型解析内容
	switch subType {
	case "v2ray":
		return parseV2raySubscription(content)
	case "ss":
		return parseSSSubscription(content)
	case "trojan":
		return parseTrojanSubscription(content)
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
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("base64 decode error: %v", err)
	}

	// 解析JSON配置
	var config struct {
		Add  string `json:"add"`
		Port int    `json:"port"` // 注意：有些配置可能是字符串格式的端口
		ID   string `json:"id"`
		Aid  int    `json:"aid"`
		Net  string `json:"net"`
		Type string `json:"type"`
		TLS  string `json:"tls"`
		Host string `json:"host"`
		Path string `json:"path"`
		PS   string `json:"ps"` // 节点名称
		V    int    `json:"v"`  // 版本
	}

	if err := json.Unmarshal(decoded, &config); err != nil {
		// 尝试处理端口为字符串的情况
		var configWithStringPort struct {
			Add  string `json:"add"`
			Port string `json:"port"`
			ID   string `json:"id"`
			Aid  int    `json:"aid"`
			Net  string `json:"net"`
			Type string `json:"type"`
			TLS  string `json:"tls"`
			Host string `json:"host"`
			Path string `json:"path"`
			PS   string `json:"ps"`
			V    int    `json:"v"`
		}

		if jsonErr := json.Unmarshal(decoded, &configWithStringPort); jsonErr != nil {
			return models.Proxy{}, fmt.Errorf("json unmarshal error: %v", err)
		}

		// 转换端口字符串为整数
		portInt, portErr := strconv.Atoi(configWithStringPort.Port)
		if portErr != nil {
			return models.Proxy{}, fmt.Errorf("invalid port format: %v", portErr)
		}

		// 将字符串端口的配置复制到原始配置
		config.Add = configWithStringPort.Add
		config.Port = portInt
		config.ID = configWithStringPort.ID
		config.Aid = configWithStringPort.Aid
		config.Net = configWithStringPort.Net
		config.Type = configWithStringPort.Type
		config.TLS = configWithStringPort.TLS
		config.Host = configWithStringPort.Host
		config.Path = configWithStringPort.Path
		config.PS = configWithStringPort.PS
		config.V = configWithStringPort.V
	}

	// 验证必要字段
	if config.Add == "" || config.Port == 0 || config.ID == "" {
		return models.Proxy{}, errors.New("missing required vmess parameters")
	}

	// 设置节点名称，如果PS为空则使用默认名称
	nodeName := "VMess Node"
	if config.PS != "" {
		nodeName = config.PS
	}

	// 处理路径，确保有默认值
	path := "/"
	if config.Path != "" {
		path = config.Path
	}

	return models.Proxy{
		Type:      "vmess",
		Name:      nodeName,
		Server:    config.Add,
		Port:      config.Port,
		UUID:      config.ID,
		Network:   config.Net,
		Path:      path,
		Host:      config.Host,
		TLS:       config.TLS == "tls",
		RawConfig: string(decoded),
	}, nil
}

// parseSSLink 解析Shadowsocks链接
func parseSSLink(link string) (models.Proxy, error) {
	// 解析格式：ss://method:password@host:port
	parts := strings.SplitN(link[5:], "@", 2)
	if len(parts) != 2 {
		return models.Proxy{}, errors.New("invalid ss format")
	}

	// 解析认证信息
	auth, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return models.Proxy{}, fmt.Errorf("base64 decode error: %v", err)
	}

	authParts := strings.SplitN(string(auth), ":", 2)
	if len(authParts) != 2 {
		return models.Proxy{}, errors.New("invalid auth format")
	}

	// 解析服务器信息
	serverParts := strings.SplitN(parts[1], ":", 2)
	if len(serverParts) != 2 {
		return models.Proxy{}, errors.New("invalid server format")
	}

	port, err := strconv.Atoi(serverParts[1])
	if err != nil {
		return models.Proxy{}, fmt.Errorf("invalid port: %v", err)
	}

	return models.Proxy{
		Type:     "ss",
		Server:   serverParts[0],
		Port:     port,
		Method:   authParts[0],
		Password: authParts[1],
	}, nil
}

// parseTrojanLink 解析Trojan链接
func parseTrojanLink(link string) (models.Proxy, error) {
	// 解析格式：trojan://password@host:port?query
	u, err := url.Parse(link)
	if err != nil {
		return models.Proxy{}, fmt.Errorf("url parse error: %v", err)
	}

	if u.Scheme != "trojan" {
		return models.Proxy{}, errors.New("invalid trojan scheme")
	}

	// 解析端口
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return models.Proxy{}, fmt.Errorf("invalid port: %v", err)
	}

	// 获取查询参数
	query := u.Query()

	return models.Proxy{
		Type:     "trojan",
		Server:   u.Hostname(),
		Port:     port,
		Password: u.User.Username(),
		SNI:      query.Get("sni"),
		ALPN:     query.Get("alpn"),
		TLS:      true,
	}, nil
}
