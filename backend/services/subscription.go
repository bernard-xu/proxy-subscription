package services

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
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
	utils.Info("开始获取订阅内容 ID=%d, URL=%s", subscription.ID, subscription.URL)

	// 获取订阅内容
	content, err := fetchSubscriptionContent(subscription.URL)
	if err != nil {
		utils.Error("获取订阅内容失败 ID=%d, URL=%s, 错误: %v", subscription.ID, subscription.URL, err)
		return fmt.Errorf("获取订阅内容失败: %w", err)
	}

	utils.Info("订阅内容获取成功 ID=%d, 内容长度=%d", subscription.ID, len(content))

	// 解析订阅内容
	utils.Info("开始解析订阅内容 ID=%d, Type=%s", subscription.ID, subscription.Type)
	proxies, err := parseSubscriptionContent(content, subscription.Type)
	if err != nil {
		utils.Error("解析订阅内容失败 ID=%d, Type=%s, 错误: %v", subscription.ID, subscription.Type, err)
		return fmt.Errorf("解析订阅内容失败: %w", err)
	}

	utils.Info("订阅内容解析成功 ID=%d, 解析出 %d 个代理节点", subscription.ID, len(proxies))

	// 开始事务
	tx := models.DB.Begin()

	var manualProxies []models.Proxy
	if err := tx.Where("subscription_id = ? AND manual_override = ?", subscription.ID, true).Find(&manualProxies).Error; err != nil {
		tx.Rollback()
		utils.Error("读取手动修改节点失败 ID=%d, 错误: %v", subscription.ID, err)
		return fmt.Errorf("读取手动修改节点失败: %w", err)
	}

	manualSourceKeys := make(map[string]struct{}, len(manualProxies))
	for i := range manualProxies {
		sourceKey := manualProxies[i].SourceKey
		if sourceKey == "" {
			sourceKey = manualProxies[i].BuildSourceKey()
			manualProxies[i].SourceKey = sourceKey
			if err := tx.Model(&manualProxies[i]).Update("source_key", sourceKey).Error; err != nil {
				tx.Rollback()
				utils.Error("更新手动修改节点标识失败 ID=%d, 错误: %v", subscription.ID, err)
				return fmt.Errorf("更新手动修改节点标识失败: %w", err)
			}
		}
		manualSourceKeys[sourceKey] = struct{}{}
	}

	// 删除旧的代理节点
	utils.Info("删除旧的代理节点 ID=%d", subscription.ID)
	if err := tx.Where("subscription_id = ? AND (manual_override = ? OR manual_override IS NULL)", subscription.ID, false).Delete(&models.Proxy{}).Error; err != nil {
		tx.Rollback()
		utils.Error("删除旧代理节点失败 ID=%d, 错误: %v", subscription.ID, err)
		return fmt.Errorf("删除旧代理节点失败: %w", err)
	}

	// 添加新的代理节点
	utils.Info("开始添加新的代理节点 ID=%d, 数量=%d", subscription.ID, len(proxies))

	for i, proxy := range proxies {
		proxy.SubscriptionID = subscription.ID
		proxy.IsCustom = false
		proxy.ManualOverride = false
		proxy.SourceKey = proxy.BuildSourceKey()
		if _, exists := manualSourceKeys[proxy.SourceKey]; exists {
			continue
		}
		if err := tx.Create(&proxy).Error; err != nil {
			tx.Rollback()
			utils.Error("添加代理节点失败 ID=%d, 节点索引=%d, 节点名称=%s, 错误: %v", subscription.ID, i, proxy.Name, err)
			return fmt.Errorf("添加代理节点失败: %w", err)
		}
	}

	utils.Info("代理节点添加成功 ID=%d, 成功添加 %d 个节点", subscription.ID, len(proxies))

	// 更新订阅的最后更新时间
	subscription.LastUpdated = time.Now()
	if err := tx.Save(subscription).Error; err != nil {
		tx.Rollback()
		utils.Error("更新订阅最后更新时间失败 ID=%d, 错误: %v", subscription.ID, err)
		return fmt.Errorf("更新订阅最后更新时间失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		utils.Error("提交事务失败 ID=%d, 错误: %v", subscription.ID, err)
		return fmt.Errorf("提交事务失败: %w", err)
	}

	utils.Info("订阅刷新完成 ID=%d, 成功刷新 %d 个代理节点", subscription.ID, len(proxies))
	return nil
}

// fetchSubscriptionContent 获取订阅内容
func fetchSubscriptionContent(subscriptionURL string) (string, error) {
	utils.Info("开始HTTP请求获取订阅内容 URL=%s", subscriptionURL)

	// 创建HTTP请求
	req, err := http.NewRequest("GET", subscriptionURL, nil)
	if err != nil {
		utils.Error("创建HTTP请求失败 URL=%s, 错误: %v", subscriptionURL, err)
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头，模拟真实浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// 如果URL包含域名，设置Referer
	if parsedURL, err := url.Parse(subscriptionURL); err == nil {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", parsedURL.Scheme, parsedURL.Host))
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		// 不自动跟随重定向，避免丢失请求头
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 在重定向时保持请求头
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			// 复制原始请求的请求头
			maps.Copy(req.Header, via[0].Header)
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		utils.Error("HTTP请求失败 URL=%s, 错误: %v", subscriptionURL, err)
		return "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	utils.Info("HTTP请求成功 URL=%s, 状态码=%d", subscriptionURL, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		// 读取响应体以获取更多错误信息
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}

		// 根据不同的状态码提供更友好的错误信息
		var errorMsg string
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			errorMsg = "订阅认证失败，token可能已过期或无效，请检查订阅URL中的token参数"
		case http.StatusForbidden:
			errorMsg = "订阅访问被拒绝，服务器可能检测到非浏览器请求，请检查订阅URL是否正确"
		case http.StatusNotFound:
			errorMsg = "订阅不存在，请检查订阅URL是否正确"
		case http.StatusTooManyRequests:
			errorMsg = "请求过于频繁，请稍后再试"
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			errorMsg = fmt.Sprintf("订阅服务器错误 (%d)，请稍后重试", resp.StatusCode)
		default:
			errorMsg = fmt.Sprintf("获取订阅内容失败，HTTP状态码: %d %s", resp.StatusCode, resp.Status)
		}

		utils.Error("HTTP状态码异常 URL=%s, 状态码=%d, 状态=%s, 响应体: %s", subscriptionURL, resp.StatusCode, resp.Status, bodyStr)
		return "", fmt.Errorf(errorMsg)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		utils.Error("读取响应体失败 URL=%s, 错误: %v", subscriptionURL, err)
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	// 检查响应是否被压缩，如果是则解压
	contentEncoding := resp.Header.Get("Content-Encoding")
	if contentEncoding == "gzip" || contentEncoding == "x-gzip" {
		utils.Info("检测到gzip压缩响应，开始解压，压缩数据长度=%d", len(body))
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			utils.Warn("创建gzip读取器失败: %v，使用原始内容", err)
		} else {
			decompressed, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				utils.Warn("gzip解压失败: %v，使用原始内容", err)
			} else {
				utils.Info("gzip解压成功，解压后长度=%d", len(decompressed))
				body = decompressed
			}
		}
	}

	utils.Info("订阅内容获取成功 URL=%s, 内容长度=%d", subscriptionURL, len(body))
	return string(body), nil
}

// parseSubscriptionContent 解析订阅内容
func parseSubscriptionContent(content string, subType string) ([]models.Proxy, error) {
	// 尝试解码Base64内容
	decoded, err := utils.DecodeBase64(content)
	if err == nil && len(decoded) > 0 {
		// Base64解码成功，检查是否需要解压
		var finalContent []byte = decoded

		// 检查是否是gzip压缩的数据（gzip文件头：0x1f 0x8b）
		if len(decoded) >= 2 && decoded[0] == 0x1f && decoded[1] == 0x8b {
			utils.Info("检测到gzip压缩数据，开始解压，压缩数据长度=%d", len(decoded))
			reader, err := gzip.NewReader(bytes.NewReader(decoded))
			if err == nil {
				decompressed, err := io.ReadAll(reader)
				reader.Close()
				if err == nil && len(decompressed) > 0 {
					utils.Info("gzip解压成功，解压后长度=%d", len(decompressed))
					finalContent = decompressed
				} else {
					utils.Warn("gzip解压失败: %v，使用原始解码内容", err)
				}
			} else {
				utils.Warn("创建gzip读取器失败: %v，使用原始解码内容", err)
			}
		}

		// 尝试将解码后的内容转换为字符串
		decodedStr := string(finalContent)

		// 检查解码后的内容是否看起来像有效的代理链接或配置
		// 检查是否包含可打印字符（至少70%是可打印字符，降低阈值以处理更多情况）
		printableCount := 0
		for _, b := range finalContent {
			if b >= 32 && b < 127 || b == 9 || b == 10 || b == 13 {
				printableCount++
			}
		}
		printableRatio := float64(printableCount) / float64(len(finalContent))

		utils.Info("Base64解码后内容分析: 长度=%d, 可打印字符比例=%.2f%%, 前100字符: %s",
			len(decodedStr), printableRatio*100, getPreview(decodedStr, 100))

		// 如果可打印字符比例足够高，或者包含明显的代理链接标识，使用解码后的内容
		if printableRatio > 0.7 || strings.Contains(decodedStr, "://") ||
			strings.HasPrefix(strings.TrimSpace(decodedStr), "{") ||
			strings.HasPrefix(strings.TrimSpace(decodedStr), "[") {
			utils.Info("使用Base64解码后的内容进行解析")
			content = decodedStr
		} else {
			// 解码后的内容看起来不像有效格式
			utils.Warn("Base64解码后的内容格式异常（可打印字符比例=%.2f%%），尝试其他解析方式", printableRatio*100)
			// 如果解压后的内容可打印字符比例很低，可能是每行都是Base64编码的链接
			// 尝试逐行Base64解码（使用字节数组，因为可能是二进制数据）
			utils.Info("尝试对解压后的内容进行逐行Base64解码（字节模式）")
			if proxies, err := parseLineByLineBase64FromBytes(finalContent, subType); err == nil && len(proxies) > 0 {
				return proxies, nil
			}
			// 尝试字符串模式的逐行Base64解码
			utils.Info("尝试对解压后的内容进行逐行Base64解码（字符串模式）")
			if proxies, err := parseLineByLineBase64(decodedStr, subType); err == nil && len(proxies) > 0 {
				return proxies, nil
			}
			// 如果原始内容看起来像Base64，尝试逐行Base64解码
			if isBase64Like(content) {
				utils.Info("原始内容看起来像Base64，尝试逐行Base64解码")
				return parseLineByLineBase64(content, subType)
			}
			// 否则尝试直接解析原始内容（可能是纯文本格式）
			utils.Info("尝试使用原始内容进行解析")
		}
	} else {
		// Base64解码失败，可能是纯文本格式，使用原始内容
		utils.Info("订阅内容Base64解码失败，使用原始内容，前100字符: %s", getPreview(content, 100))
	}

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

// isBase64Like 检查字符串是否看起来像Base64编码
func isBase64Like(s string) bool {
	if len(s) < 4 {
		return false
	}
	// Base64只包含A-Z, a-z, 0-9, +, /, = 和可能的空格/换行
	validChars := 0
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' ||
			r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			validChars++
		}
	}
	return float64(validChars)/float64(len(s)) > 0.9
}

// parseLineByLineBase64FromBytes 从字节数组逐行Base64解码并解析
func parseLineByLineBase64FromBytes(data []byte, subType string) ([]models.Proxy, error) {
	utils.Info("尝试从字节数组逐行Base64解码，数据长度=%d", len(data))
	var allProxies []models.Proxy

	// 按换行符分割（支持\n和\r\n）
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 1 {
		// 如果没有\n，尝试\r\n
		lines = bytes.Split(data, []byte("\r\n"))
	}

	utils.Info("分割后共 %d 行", len(lines))

	for i, lineBytes := range lines {
		// 去除首尾空白字符
		lineBytes = bytes.TrimSpace(lineBytes)
		if len(lineBytes) == 0 {
			continue
		}

		// 尝试将字节数组转换为字符串（可能是Base64编码的字符串）
		lineStr := string(lineBytes)

		// 检查这一行是否看起来像Base64（至少包含Base64字符）
		base64Like := false
		base64CharCount := 0
		for _, b := range lineBytes {
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') ||
				(b >= '0' && b <= '9') || b == '+' || b == '/' || b == '=' {
				base64CharCount++
			}
		}
		base64Ratio := 0.0
		if len(lineBytes) > 0 {
			base64Ratio = float64(base64CharCount) / float64(len(lineBytes))
			if base64Ratio > 0.8 {
				base64Like = true
			}
		}

		if !base64Like {
			// 如果这一行不像Base64，跳过
			if i < 3 {
				previewLen := 50
				if len(lineBytes) < previewLen {
					previewLen = len(lineBytes)
				}
				utils.Info("行 %d 不像Base64编码，跳过，前%d字节(hex): %x", i+1, previewLen, lineBytes[:previewLen])
			}
			continue
		}

		// 尝试Base64解码这一行
		decoded, err := utils.DecodeBase64(lineStr)
		if err != nil {
			// 如果这一行不是Base64，跳过
			if i < 3 {
				utils.Info("行 %d Base64解码失败: %v", i+1, err)
			}
			continue
		}

		// 检查解码后是否是gzip压缩
		if len(decoded) >= 2 && decoded[0] == 0x1f && decoded[1] == 0x8b {
			reader, err := gzip.NewReader(bytes.NewReader(decoded))
			if err == nil {
				decompressed, err := io.ReadAll(reader)
				reader.Close()
				if err == nil {
					decoded = decompressed
				}
			}
		}

		decodedStr := string(decoded)
		utils.Info("行 %d Base64解码成功，解码后长度=%d, 前100字符: %s", i+1, len(decodedStr), getPreview(decodedStr, 100))

		// 尝试解析解码后的内容
		proxies, err := autoDetectAndParse(decodedStr)
		if err == nil && len(proxies) > 0 {
			allProxies = append(allProxies, proxies...)
		} else if err != nil {
			utils.Warn("行 %d Base64解码后解析失败: %v", i+1, err)
		}
	}

	if len(allProxies) > 0 {
		utils.Info("从字节数组逐行Base64解码成功，解析出 %d 个代理节点", len(allProxies))
		return allProxies, nil
	}

	return nil, fmt.Errorf("从字节数组逐行Base64解码未找到代理")
}

// parseLineByLineBase64 逐行Base64解码并解析
func parseLineByLineBase64(content string, subType string) ([]models.Proxy, error) {
	utils.Info("尝试逐行Base64解码（字符串模式）")
	var allProxies []models.Proxy
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 尝试Base64解码这一行
		decoded, err := utils.DecodeBase64(line)
		if err != nil {
			// 如果这一行不是Base64，可能是纯文本链接，跳过
			continue
		}

		// 检查解码后是否是gzip压缩
		if len(decoded) >= 2 && decoded[0] == 0x1f && decoded[1] == 0x8b {
			reader, err := gzip.NewReader(bytes.NewReader(decoded))
			if err == nil {
				decompressed, err := io.ReadAll(reader)
				reader.Close()
				if err == nil {
					decoded = decompressed
				}
			}
		}

		decodedStr := string(decoded)
		utils.Info("行 %d Base64解码成功，解码后长度=%d, 前100字符: %s", i+1, len(decodedStr), getPreview(decodedStr, 100))

		// 尝试解析解码后的内容
		proxies, err := autoDetectAndParse(decodedStr)
		if err == nil && len(proxies) > 0 {
			allProxies = append(allProxies, proxies...)
		} else if err != nil {
			utils.Warn("行 %d Base64解码后解析失败: %v", i+1, err)
		}
	}

	if len(allProxies) > 0 {
		utils.Info("逐行Base64解码成功，解析出 %d 个代理节点", len(allProxies))
		return allProxies, nil
	}

	// 如果逐行解码失败，尝试整体解码后按换行分割
	utils.Info("逐行Base64解码未找到代理，尝试整体解码")
	return autoDetectAndParse(content)
}

// getPreview 获取内容预览，用于日志
func getPreview(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// getPreviewBytes 获取字节数组的十六进制预览，用于日志
func getPreviewBytes(data []byte, maxLen int) string {
	if len(data) <= maxLen {
		return fmt.Sprintf("%x", data)
	}
	return fmt.Sprintf("%x...", data[:maxLen])
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
	utils.Info("开始自动检测订阅类型，内容长度=%d", len(content))

	// 检查是否为JSON格式
	trimmed := strings.TrimSpace(content)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		utils.Info("检测到JSON格式订阅")
		return parseJSONSubscription(content)
	}

	// 检查是否为Clash配置
	if strings.Contains(content, "proxies:") && (strings.Contains(content, "yaml") || strings.Contains(content, "rules:")) {
		utils.Info("检测到Clash格式订阅")
		return parseClashSubscription(content)
	}

	// 检查是否为Surge配置
	if strings.Contains(content, "[Proxy]") || strings.Contains(content, "[Proxy Group]") {
		utils.Info("检测到Surge格式订阅")
		return parseSurgeSubscription(content)
	}

	// 检查是否为Quantumult配置
	if strings.Contains(content, "shadowsocks=") || strings.Contains(content, "vmess=") || strings.Contains(content, "SERVER,") {
		utils.Info("检测到Quantumult格式订阅")
		return parseQuantumultSubscription(content)
	}

	// 检查内容是否是二进制数据（包含大量不可打印字符）
	contentBytes := []byte(content)
	printableCount := 0
	for _, b := range contentBytes {
		if b >= 32 && b < 127 || b == 9 || b == 10 || b == 13 {
			printableCount++
		}
	}
	printableRatio := float64(printableCount) / float64(len(contentBytes))

	// 如果可打印字符比例很低，可能是二进制数据，尝试逐行Base64解码
	if len(contentBytes) > 0 && printableRatio < 0.5 {
		utils.Info("检测到内容可能是二进制数据（可打印字符比例=%.2f%%），尝试逐行Base64解码", printableRatio*100)
		if proxies, err := parseLineByLineBase64FromBytes(contentBytes, ""); err == nil && len(proxies) > 0 {
			return proxies, nil
		}
		utils.Warn("逐行Base64解码未找到代理，继续尝试其他解析方式")
	}

	// 逐行解析URI
	utils.Info("尝试逐行解析URI格式订阅")
	var proxies []models.Proxy
	lines := strings.Split(content, "\n")

	utils.Info("内容共 %d 行，开始解析", len(lines))

	// 打印前5行的实际内容用于调试
	nonEmptyLines := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmptyLines++
			if nonEmptyLines <= 5 {
				utils.Info("第 %d 行内容预览（前100字符）: %s", i+1, getPreview(trimmed, 100))
			}
		}
	}
	utils.Info("非空行总数: %d", nonEmptyLines)

	vmessCount := 0
	ssCount := 0
	trojanCount := 0
	otherCount := 0
	errorCount := 0

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var proxy models.Proxy
		var err error

		switch {
		case strings.HasPrefix(line, "vmess://"):
			vmessCount++
			proxy, err = parseVmessLink(line)
			if err != nil {
				utils.Warn("解析vmess链接失败 行=%d, 错误: %v, 链接前50字符: %s", i+1, err, getPreview(line, 50))
				errorCount++
			}
		case strings.HasPrefix(line, "ss://"):
			ssCount++
			proxy, err = parseSSLink(line)
			if err != nil {
				utils.Warn("解析ss链接失败 行=%d, 错误: %v, 链接前50字符: %s", i+1, err, getPreview(line, 50))
				errorCount++
			}
		case strings.HasPrefix(line, "trojan://"):
			trojanCount++
			proxy, err = parseTrojanLink(line)
			if err != nil {
				utils.Warn("解析trojan链接失败 行=%d, 错误: %v, 链接前50字符: %s", i+1, err, getPreview(line, 50))
				errorCount++
			}
		case strings.HasPrefix(line, "ssr://"):
			proxy, err = parseSSRLink(line)
			if err != nil {
				utils.Warn("解析ssr链接失败 行=%d, 错误: %v", i+1, err)
				errorCount++
			}
		case strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://"):
			proxy, err = parseHTTPLink(line)
			if err != nil {
				utils.Warn("解析http链接失败 行=%d, 错误: %v", i+1, err)
				errorCount++
			}
		case strings.HasPrefix(line, "socks://") || strings.HasPrefix(line, "socks5://"):
			proxy, err = parseSOCKSLink(line)
			if err != nil {
				utils.Warn("解析socks链接失败 行=%d, 错误: %v", i+1, err)
				errorCount++
			}
		default:
			otherCount++
			// 记录前10行不支持的内容，用于调试
			if otherCount <= 10 {
				utils.Info("跳过不支持的行 %d, 内容前100字符: %s", i+1, getPreview(line, 100))
				// 尝试检查是否是Base64编码的链接
				if decoded, err := utils.DecodeBase64(line); err == nil && len(decoded) > 0 {
					decodedStr := string(decoded)
					utils.Info("  该行可能是Base64编码，解码后前100字符: %s", getPreview(decodedStr, 100))
					// 如果解码后看起来像代理链接，尝试解析
					if strings.HasPrefix(decodedStr, "vmess://") ||
						strings.HasPrefix(decodedStr, "ss://") ||
						strings.HasPrefix(decodedStr, "trojan://") {
						utils.Info("  检测到解码后是代理链接，尝试解析")
						var decodedProxy models.Proxy
						var decodedErr error
						switch {
						case strings.HasPrefix(decodedStr, "vmess://"):
							vmessCount++
							decodedProxy, decodedErr = parseVmessLink(decodedStr)
						case strings.HasPrefix(decodedStr, "ss://"):
							ssCount++
							decodedProxy, decodedErr = parseSSLink(decodedStr)
						case strings.HasPrefix(decodedStr, "trojan://"):
							trojanCount++
							decodedProxy, decodedErr = parseTrojanLink(decodedStr)
						}
						if decodedErr == nil {
							proxies = append(proxies, decodedProxy)
							otherCount-- // 修正计数
							utils.Info("  成功解析Base64编码的代理链接")
							continue
						} else {
							utils.Warn("  解析Base64解码后的链接失败: %v", decodedErr)
							errorCount++
						}
					}
				}
			}
			continue // 跳过不支持的链接
		}

		if err == nil {
			proxies = append(proxies, proxy)
		}
	}

	utils.Info("解析完成: vmess=%d, ss=%d, trojan=%d, 其他=%d, 错误=%d, 成功解析=%d",
		vmessCount, ssCount, trojanCount, otherCount, errorCount, len(proxies))

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
