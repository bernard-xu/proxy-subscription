package services

import (
	"encoding/json"
	"testing"
)

const sampleVmessLink = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJ2bS13cy1hbGl5dW4tanAtMDMiLAogICJhZGQiOiAiOC4yMDkuMjU0LjI0OCIsCiAgInBvcnQiOiAiMjA4MiIsCiAgImlkIjogIjEwZTI1ZjY1LWQ0YTMtNGU1YS05OGViLWU0NTlmMTg5OWU1NSIsCiAgImFpZCI6ICIwIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAid3MiLAogICJ0eXBlIjogIm5vbmUiLAogICJob3N0IjogInd3dy5iaW5nLmNvbSIsCiAgInBhdGgiOiAiMTBlMjVmNjUtZDRhMy00ZTVhLTk4ZWItZTQ1OWYxODk5ZTU1LXZtIiwKICAidGxzIjogIiIsCiAgInNuaSI6ICIiLAogICJhbHBuIjogIiIsCiAgImZwIjogIiIsCiAgImluc2VjdXJlIjogIjAiCn0="

const sampleVlessLink = "vless://10e25f65-d4a3-4e5a-98eb-e459f1899e55@8.209.254.248:18543?encryption=none&flow=xtls-rprx-vision&security=reality&sni=apple.com&fp=chrome&pbk=PR8JkbArJstRJb8y584SqRkjpMqbyHoZupc2L5sT_Gs&sid=5f7aaec5&type=tcp&headerType=none#vl-reality-aliyun-jp-03"

func TestParseVmessLinkSample(t *testing.T) {
	proxy, err := parseVmessLink(sampleVmessLink)
	if err != nil {
		t.Fatalf("parseVmessLink() error = %v", err)
	}

	assertEqual(t, proxy.Type, "vmess", "Type")
	assertEqual(t, proxy.Name, "vm-ws-aliyun-jp-03", "Name")
	assertEqual(t, proxy.Server, "8.209.254.248", "Server")
	assertEqual(t, proxy.Port, 2082, "Port")
	assertEqual(t, proxy.UUID, "10e25f65-d4a3-4e5a-98eb-e459f1899e55", "UUID")
	assertEqual(t, proxy.Network, "ws", "Network")
	assertEqual(t, proxy.Host, "www.bing.com", "Host")
	assertEqual(t, proxy.Path, "10e25f65-d4a3-4e5a-98eb-e459f1899e55-vm", "Path")
	assertEqual(t, proxy.TLS, false, "TLS")

	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
		t.Fatalf("RawConfig is not valid JSON: %v", err)
	}
	assertEqual(t, rawConfig["scy"], "auto", "RawConfig.scy")
	assertEqual(t, rawConfig["tls"], "", "RawConfig.tls")
	assertEqual(t, rawConfig["insecure"], "0", "RawConfig.insecure")
}

func TestParseVlessLinkSample(t *testing.T) {
	proxy, err := parseVlessLink(sampleVlessLink)
	if err != nil {
		t.Fatalf("parseVlessLink() error = %v", err)
	}

	assertEqual(t, proxy.Type, "vless", "Type")
	assertEqual(t, proxy.Name, "vl-reality-aliyun-jp-03", "Name")
	assertEqual(t, proxy.Server, "8.209.254.248", "Server")
	assertEqual(t, proxy.Port, 18543, "Port")
	assertEqual(t, proxy.UUID, "10e25f65-d4a3-4e5a-98eb-e459f1899e55", "UUID")
	assertEqual(t, proxy.Network, "tcp", "Network")
	assertEqual(t, proxy.SNI, "apple.com", "SNI")
	assertEqual(t, proxy.TLS, true, "TLS")

	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(proxy.RawConfig), &rawConfig); err != nil {
		t.Fatalf("RawConfig is not valid JSON: %v", err)
	}
	assertEqual(t, rawConfig["encryption"], "none", "RawConfig.encryption")
	assertEqual(t, rawConfig["flow"], "xtls-rprx-vision", "RawConfig.flow")
	assertEqual(t, rawConfig["security"], "reality", "RawConfig.security")
	assertEqual(t, rawConfig["fp"], "chrome", "RawConfig.fp")
	assertEqual(t, rawConfig["pbk"], "PR8JkbArJstRJb8y584SqRkjpMqbyHoZupc2L5sT_Gs", "RawConfig.pbk")
	assertEqual(t, rawConfig["sid"], "5f7aaec5", "RawConfig.sid")
	assertEqual(t, rawConfig["headerType"], "none", "RawConfig.headerType")
}

func TestParseSubscriptionContentMixedSamples(t *testing.T) {
	proxies, err := parseSubscriptionContent(sampleVmessLink+"\n"+sampleVlessLink, "mixed")
	if err != nil {
		t.Fatalf("parseSubscriptionContent() error = %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("parseSubscriptionContent() returned %d proxies, want 2", len(proxies))
	}
	assertEqual(t, proxies[0].Type, "vmess", "first proxy type")
	assertEqual(t, proxies[1].Type, "vless", "second proxy type")
}

func assertEqual[T comparable](t *testing.T, got, want T, field string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", field, got, want)
	}
}
