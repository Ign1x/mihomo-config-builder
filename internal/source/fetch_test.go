package source

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func TestLoadSubscriptionsFromFileAndURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "proxies: []\nproxy-groups: []\nrules: []\n")
	}))
	defer ts.Close()

	dir := t.TempDir()
	local := filepath.Join(dir, "sub.yaml")
	if err := os.WriteFile(local, []byte("proxies: []\nproxy-groups: []\nrules: []\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := New(5*time.Second, 1)
	p := profile.DefaultProfile()
	p.Subscriptions = []profile.SourceRef{
		{File: "sub.yaml"},
		{URL: ts.URL},
	}
	res := f.LoadSubscriptions(context.Background(), p, filepath.Join(dir, "profile.yaml"))
	if len(res) != 2 {
		t.Fatalf("unexpected result size")
	}
	for _, r := range res {
		if r.Err != nil {
			t.Fatalf("unexpected fetch error: %v", r.Err)
		}
	}
}

func TestLoadURLRetry(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 2 {
			http.Error(w, "bad", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	f := New(3*time.Second, 2)
	_, err := f.loadURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("expected retry success, got: %v", err)
	}
}

func TestLoadSubscriptionsFromNodesFile(t *testing.T) {
	vmessJSON := `{"v":"2","ps":"vmess-node","add":"vm.example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"ws","host":"ws.example.com","path":"/ws","tls":"tls"}`
	vmessLine := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessJSON))
	ssLine := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password@ss.example.com:8388")) + "#ss-node"
	ssSIP002UserInfo := "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:password")) + "@ss4.example.com:8388#ss-sip002-userinfo"
	ssNoName := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password@ss2.example.com:8388"))
	ssNoName2 := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password@ss3.example.com:8388"))
	socksLine := "socks5://user:pass@socks.example.com:1080#socks-node"
	socksEncodedName := "socks5://user:pass@socks-encoded.example.com:1080#%F0%9F%8F%AB%20%E6%A0%A1%E5%86%85%20FRP%20%E4%BB%A3%E7%90%86"
	httpLine := "http://user:pass@http.example.com:8080#http-node"
	httpsLine := "https://https.example.com:8443#https-node"
	sock5Line := "sock5://sock5.example.com:1081#sock5-node"
	upperCaseScheme := "SOCKS5://upper.example.com:1082#upper-socks"
	vlessNode := "vless://22222222-2222-2222-2222-222222222222@vless.example.com:443?type=ws&security=tls&sni=vless.example.com&path=%2Fws#vless-node"
	trojanNode := "trojan://secret@trojan.example.com:443?sni=trojan.example.com#trojan-node"
	hy2Line := "hysteria2://secret@hy.example.com:8443?alpn=h3&insecure=1#hy2-node"
	nodesContent := strings.Join([]string{ssLine, ssSIP002UserInfo, ssNoName, ssNoName2, vmessLine, socksLine, socksEncodedName, sock5Line, httpLine, httpsLine, upperCaseScheme, vlessNode, trojanNode, hy2Line}, "\n") + "\n"

	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.txt")
	if err := os.WriteFile(nodesPath, []byte(nodesContent), 0o644); err != nil {
		t.Fatalf("write nodes file: %v", err)
	}

	f := New(5*time.Second, 1)
	p := profile.DefaultProfile()
	p.Subscriptions = []profile.SourceRef{{NodesFile: "nodes.txt"}}

	res := f.LoadSubscriptions(context.Background(), p, filepath.Join(dir, "profile.yaml"))
	if len(res) != 1 {
		t.Fatalf("unexpected result size: %d", len(res))
	}
	if res[0].Err != nil {
		t.Fatalf("unexpected nodes load error: %v", res[0].Err)
	}
	if !strings.Contains(string(res[0].Data), "proxies:") {
		t.Fatalf("converted subscription missing proxies")
	}
	if !strings.Contains(string(res[0].Data), "type: ss") {
		t.Fatalf("converted subscription missing ss node")
	}
	if !strings.Contains(string(res[0].Data), "type: vmess") {
		t.Fatalf("converted subscription missing vmess node")
	}
	if !strings.Contains(string(res[0].Data), "type: vless") {
		t.Fatalf("converted subscription missing vless node")
	}
	if !strings.Contains(string(res[0].Data), "type: trojan") {
		t.Fatalf("converted subscription missing trojan node")
	}
	if !strings.Contains(string(res[0].Data), "type: socks5") {
		t.Fatalf("converted subscription missing socks5 node")
	}
	if !strings.Contains(string(res[0].Data), "type: http") {
		t.Fatalf("converted subscription missing http node")
	}
	if !strings.Contains(string(res[0].Data), "name: SS") || !strings.Contains(string(res[0].Data), "SS-2") {
		t.Fatalf("converted subscription missing unique ss fallback names")
	}
	if !strings.Contains(string(res[0].Data), "name: ss-sip002-userinfo") {
		t.Fatalf("converted subscription missing ss sip002 userinfo node")
	}
	if !strings.Contains(string(res[0].Data), "socks-encoded.example.com") {
		t.Fatalf("converted subscription missing encoded-name socks node")
	}
	if strings.Contains(string(res[0].Data), "%20") {
		t.Fatalf("converted subscription should not keep percent-encoded node name fragment")
	}
	if !strings.Contains(string(res[0].Data), "type: hysteria2") {
		t.Fatalf("converted subscription missing hysteria2 node")
	}
	if !strings.Contains(string(res[0].Data), "proxy-groups:") {
		t.Fatalf("converted subscription missing proxy-groups")
	}
}

func TestLoadSubscriptionsFromNodesFileMissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.txt")

	if err := os.WriteFile(nodesPath, []byte("trojan://@trojan.example.com:443#bad\n"), 0o644); err != nil {
		t.Fatalf("write nodes file: %v", err)
	}

	f := New(5*time.Second, 1)
	p := profile.DefaultProfile()
	p.Subscriptions = []profile.SourceRef{{NodesFile: "nodes.txt"}}

	res := f.LoadSubscriptions(context.Background(), p, filepath.Join(dir, "profile.yaml"))
	if len(res) != 1 {
		t.Fatalf("unexpected result size: %d", len(res))
	}
	if res[0].Err == nil {
		t.Fatalf("expected nodes parse error")
	}
	if !strings.Contains(res[0].Err.Error(), "trojan password is required") {
		t.Fatalf("unexpected nodes error: %v", res[0].Err)
	}

	if err := os.WriteFile(nodesPath, []byte("vless://@vless.example.com:443?security=tls#bad\n"), 0o644); err != nil {
		t.Fatalf("rewrite nodes file: %v", err)
	}
	res = f.LoadSubscriptions(context.Background(), p, filepath.Join(dir, "profile.yaml"))
	if len(res) != 1 {
		t.Fatalf("unexpected result size: %d", len(res))
	}
	if res[0].Err == nil {
		t.Fatalf("expected nodes parse error")
	}
	if !strings.Contains(res[0].Err.Error(), "vless uuid is required") {
		t.Fatalf("unexpected nodes error: %v", res[0].Err)
	}

	if err := os.WriteFile(nodesPath, []byte("vmess://eyJ2IjoiMiIsInBvcnQiOiI0NDMifQ==\n"), 0o644); err != nil {
		t.Fatalf("rewrite nodes file: %v", err)
	}
	res = f.LoadSubscriptions(context.Background(), p, filepath.Join(dir, "profile.yaml"))
	if len(res) != 1 {
		t.Fatalf("unexpected result size: %d", len(res))
	}
	if res[0].Err == nil {
		t.Fatalf("expected nodes parse error")
	}
	if !strings.Contains(res[0].Err.Error(), "vmess server is required") {
		t.Fatalf("unexpected nodes error: %v", res[0].Err)
	}
}

func TestLoadSubscriptionsFromNodesFileWithSubscriptionLinks(t *testing.T) {
	subBase64Node := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password@sub-ss.example.com:8388")) + "#from-sub-base64"
	subBase64Payload := base64.StdEncoding.EncodeToString([]byte(subBase64Node + "\n"))
	yamlPayload := "proxies:\n  - name: yaml-sub-node\n    type: socks5\n    server: yaml-socks.example.com\n    port: 1080\nproxy-groups: []\nrules: []\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sub-base64":
			fmt.Fprint(w, subBase64Payload)
		case "/sub-yaml":
			fmt.Fprint(w, yamlPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	directNode := "socks5://user:pass@direct.example.com:1080#direct-node"
	nodesContent := strings.Join([]string{
		directNode,
		ts.URL + "/sub-base64?token=abc#HK",
		ts.URL + "/sub-yaml#JP",
	}, "\n") + "\n"

	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.txt")
	if err := os.WriteFile(nodesPath, []byte(nodesContent), 0o644); err != nil {
		t.Fatalf("write nodes file: %v", err)
	}

	f := New(5*time.Second, 1)
	p := profile.DefaultProfile()
	p.Subscriptions = []profile.SourceRef{{NodesFile: "nodes.txt"}}

	res := f.LoadSubscriptions(context.Background(), p, filepath.Join(dir, "profile.yaml"))
	if len(res) != 1 {
		t.Fatalf("unexpected result size: %d", len(res))
	}
	if res[0].Err != nil {
		t.Fatalf("unexpected nodes load error: %v", res[0].Err)
	}

	out := string(res[0].Data)
	if !strings.Contains(out, "name: direct-node") {
		t.Fatalf("missing direct node")
	}
	if !strings.Contains(out, "name: from-sub-base64 HK") {
		t.Fatalf("missing node from base64 subscription link")
	}
	if !strings.Contains(out, "name: yaml-sub-node JP") {
		t.Fatalf("missing node from yaml subscription link")
	}
	if strings.Contains(out, "type: http") {
		t.Fatalf("subscription links should not be treated as http proxy nodes")
	}
}

func TestLoadSubscriptionsFromURLNormalizesNodePayloads(t *testing.T) {
	subBase64Node := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password@sub-ss.example.com:8388")) + "#from-sub-base64"
	subBase64Payload := base64.StdEncoding.EncodeToString([]byte(subBase64Node + "\n"))
	yamlPayload := "proxies:\n  - name: yaml-sub-node\n    type: socks5\n    server: yaml-socks.example.com\n    port: 1080\nproxy-groups: []\nrules: []\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sub-base64":
			fmt.Fprint(w, subBase64Payload)
		case "/sub-yaml":
			fmt.Fprint(w, yamlPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	f := New(5*time.Second, 1)
	p := profile.DefaultProfile()
	p.Subscriptions = []profile.SourceRef{{URL: ts.URL + "/sub-base64"}, {URL: ts.URL + "/sub-yaml"}}

	res := f.LoadSubscriptions(context.Background(), p, filepath.Join(t.TempDir(), "profile.yaml"))
	if len(res) != 2 {
		t.Fatalf("unexpected result size: %d", len(res))
	}
	for i, result := range res {
		if result.Err != nil {
			t.Fatalf("subscription %d failed: %v", i, result.Err)
		}
		if !strings.Contains(string(result.Data), "proxy-groups:") {
			t.Fatalf("subscription %d missing normalized proxy-groups", i)
		}
		if !strings.Contains(string(result.Data), "rules:") {
			t.Fatalf("subscription %d missing normalized rules", i)
		}
	}
	if !strings.Contains(string(res[0].Data), "name: from-sub-base64") {
		t.Fatalf("missing normalized node from base64 URL subscription")
	}
	if !strings.Contains(string(res[1].Data), "name: yaml-sub-node") {
		t.Fatalf("missing normalized node from yaml URL subscription")
	}
}
