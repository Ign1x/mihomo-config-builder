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
	httpLine := "http://user:pass@http.example.com:8080#http-node"
	httpsLine := "https://https.example.com:8443#https-node"
	sock5Line := "sock5://sock5.example.com:1081#sock5-node"
	upperCaseScheme := "SOCKS5://upper.example.com:1082#upper-socks"
	vlessNode := "vless://22222222-2222-2222-2222-222222222222@vless.example.com:443?type=ws&security=tls&sni=vless.example.com&path=%2Fws#vless-node"
	trojanNode := "trojan://secret@trojan.example.com:443?sni=trojan.example.com#trojan-node"
	hy2Line := "hysteria2://secret@hy.example.com:8443?alpn=h3&insecure=1#hy2-node"
	nodesContent := strings.Join([]string{ssLine, ssSIP002UserInfo, ssNoName, ssNoName2, vmessLine, socksLine, sock5Line, httpLine, httpsLine, upperCaseScheme, vlessNode, trojanNode, hy2Line}, "\n") + "\n"

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
