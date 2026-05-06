package api

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"proxy-subscription/models"
)

func TestGenerateVmessURLRoundTrip(t *testing.T) {
	original := models.Proxy{
		Type:    "vmess",
		Name:    "vm-ws-aliyun-jp-03",
		Server:  "8.209.254.248",
		Port:    2082,
		UUID:    "10e25f65-d4a3-4e5a-98eb-e459f1899e55",
		Network: "ws",
		Path:    "10e25f65-d4a3-4e5a-98eb-e459f1899e55-vm",
		Host:    "www.bing.com",
		TLS:     false,
	}

	generated := generateVmessURL(original)
	roundTripped := decodeVmessURLForTest(t, generated)

	assertProxyFieldsEqual(t, roundTripped, original)
}

func TestGenerateVlessURLRoundTrip(t *testing.T) {
	rawConfig := map[string]string{
		"encryption": "none",
		"flow":       "xtls-rprx-vision",
		"security":   "reality",
		"sni":        "apple.com",
		"fp":         "chrome",
		"pbk":        "PR8JkbArJstRJb8y584SqRkjpMqbyHoZupc2L5sT_Gs",
		"sid":        "5f7aaec5",
		"type":       "tcp",
		"headerType": "none",
	}
	rawData, err := json.Marshal(rawConfig)
	if err != nil {
		t.Fatalf("json.Marshal(rawConfig) error = %v", err)
	}

	original := models.Proxy{
		Type:      "vless",
		Name:      "vl-reality-aliyun-jp-03",
		Server:    "8.209.254.248",
		Port:      18543,
		UUID:      "10e25f65-d4a3-4e5a-98eb-e459f1899e55",
		Network:   "tcp",
		SNI:       "apple.com",
		TLS:       true,
		RawConfig: string(rawData),
	}

	generated := generateVlessURL(original)
	roundTripped, roundTripConfig := decodeVlessURLForTest(t, generated)

	assertProxyFieldsEqual(t, roundTripped, original)
	for key, want := range rawConfig {
		if got := roundTripConfig[key]; got != want {
			t.Fatalf("round-trip raw config %q = %q, want %q", key, got, want)
		}
	}
}

func TestGenerateSubscriptionContentBase64RoundTrip(t *testing.T) {
	rawConfig := `{"encryption":"none","flow":"xtls-rprx-vision","security":"reality","sni":"apple.com","fp":"chrome","pbk":"PR8JkbArJstRJb8y584SqRkjpMqbyHoZupc2L5sT_Gs","sid":"5f7aaec5","type":"tcp","headerType":"none"}`
	proxies := []models.Proxy{
		{
			Type:    "vmess",
			Name:    "vm-ws-aliyun-jp-03",
			Server:  "8.209.254.248",
			Port:    2082,
			UUID:    "10e25f65-d4a3-4e5a-98eb-e459f1899e55",
			Network: "ws",
			Path:    "10e25f65-d4a3-4e5a-98eb-e459f1899e55-vm",
			Host:    "www.bing.com",
		},
		{
			Type:      "vless",
			Name:      "vl-reality-aliyun-jp-03",
			Server:    "8.209.254.248",
			Port:      18543,
			UUID:      "10e25f65-d4a3-4e5a-98eb-e459f1899e55",
			Network:   "tcp",
			SNI:       "apple.com",
			TLS:       true,
			RawConfig: rawConfig,
		},
	}

	content, _, err := generateSubscriptionContent(proxies, "base64")
	if err != nil {
		t.Fatalf("generateSubscriptionContent() error = %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		t.Fatalf("base64 decode generated content error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(decoded)), "\n")
	if len(lines) != 2 {
		t.Fatalf("generated subscription contains %d links, want 2", len(lines))
	}

	assertProxyFieldsEqual(t, decodeVmessURLForTest(t, lines[0]), proxies[0])
	roundTrippedVless, _ := decodeVlessURLForTest(t, lines[1])
	assertProxyFieldsEqual(t, roundTrippedVless, proxies[1])
}

func decodeVmessURLForTest(t *testing.T, link string) models.Proxy {
	t.Helper()
	if !strings.HasPrefix(link, "vmess://") {
		t.Fatalf("vmess link = %q, want vmess:// prefix", link)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "vmess://"))
	if err != nil {
		t.Fatalf("decode vmess payload error = %v", err)
	}

	var config struct {
		PS   string `json:"ps"`
		Add  string `json:"add"`
		Port int    `json:"port"`
		ID   string `json:"id"`
		Net  string `json:"net"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
	}
	if err := json.Unmarshal(decoded, &config); err != nil {
		t.Fatalf("unmarshal vmess payload error = %v", err)
	}

	return models.Proxy{
		Type:    "vmess",
		Name:    config.PS,
		Server:  config.Add,
		Port:    config.Port,
		UUID:    config.ID,
		Network: config.Net,
		Path:    config.Path,
		Host:    config.Host,
		TLS:     config.TLS == "tls",
	}
}

func decodeVlessURLForTest(t *testing.T, link string) (models.Proxy, map[string]string) {
	t.Helper()

	parsedURL, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse vless url error = %v", err)
	}
	if parsedURL.Scheme != "vless" {
		t.Fatalf("scheme = %q, want vless", parsedURL.Scheme)
	}

	host, portStr, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		t.Fatalf("split vless host/port error = %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse vless port error = %v", err)
	}

	query := parsedURL.Query()
	rawConfig := make(map[string]string, len(query))
	for key := range query {
		rawConfig[key] = query.Get(key)
	}

	return models.Proxy{
		Type:    "vless",
		Name:    parsedURL.Fragment,
		Server:  host,
		Port:    port,
		UUID:    parsedURL.User.Username(),
		Network: query.Get("type"),
		Path:    query.Get("path"),
		Host:    query.Get("host"),
		SNI:     query.Get("sni"),
		ALPN:    query.Get("alpn"),
		TLS:     query.Get("security") == "tls" || query.Get("security") == "reality",
	}, rawConfig
}

func assertProxyFieldsEqual(t *testing.T, got, want models.Proxy) {
	t.Helper()

	assertEqual(t, got.Type, want.Type, "Type")
	assertEqual(t, got.Name, want.Name, "Name")
	assertEqual(t, got.Server, want.Server, "Server")
	assertEqual(t, got.Port, want.Port, "Port")
	assertEqual(t, got.UUID, want.UUID, "UUID")
	assertEqual(t, got.Network, want.Network, "Network")
	assertEqual(t, got.Path, want.Path, "Path")
	assertEqual(t, got.Host, want.Host, "Host")
	assertEqual(t, got.TLS, want.TLS, "TLS")
	assertEqual(t, got.SNI, want.SNI, "SNI")
	assertEqual(t, got.ALPN, want.ALPN, "ALPN")
}

func assertEqual[T comparable](t *testing.T, got, want T, field string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", field, got, want)
	}
}
