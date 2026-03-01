package build

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/logging"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func TestBuildSnapshot(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "integration")
	profilePath := filepath.Join(dir, "profile.yaml")
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile read: %v", err)
	}
	res, err := Run(context.Background(), p, profilePath, logging.New(os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("build run: %v", err)
	}
	got, err := configfile.MarshalYAML(res.Config, true, true)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "expected.yaml"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("snapshot mismatch\n--- got ---\n%s\n--- want ---\n%s", string(got), string(want))
	}
}

func TestBuildSnapshotWithTemplateAndHook(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "integration")
	profilePath := filepath.Join(dir, "profile-with-template-hook.yaml")
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile read: %v", err)
	}
	res, err := Run(context.Background(), p, profilePath, logging.New(os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("build run: %v", err)
	}
	got, err := configfile.MarshalYAML(res.Config, true, true)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "expected-with-template-hook.yaml"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("snapshot mismatch\n--- got ---\n%s\n--- want ---\n%s", string(got), string(want))
	}
}

func TestBuildFromNodesFileSnapshot(t *testing.T) {
	dir := t.TempDir()

	vmessJSON := `{"v":"2","ps":"vmess-node","add":"vm.example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"ws","host":"ws.example.com","path":"/ws","tls":"tls"}`
	vmessLine := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessJSON))
	ssLine := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password@ss.example.com:8388")) + "#ss-node"
	socksLine := "socks5://user:pass@socks.example.com:1080#socks-node"
	sock5Line := "sock5://sock5.example.com:1081#sock5-node"
	httpLine := "http://user:pass@http.example.com:8080#http-node"
	httpsLine := "https://https.example.com:8443#https-node"
	nodesContent := strings.Join([]string{ssLine, vmessLine, socksLine, sock5Line, httpLine, httpsLine}, "\n") + "\n"

	nodesPath := filepath.Join(dir, "nodes.txt")
	if err := os.WriteFile(nodesPath, []byte(nodesContent), 0o644); err != nil {
		t.Fatalf("write nodes file: %v", err)
	}

	profileBody := "subscriptions:\n  - nodesFile: ./nodes.txt\noutput:\n  deterministic: true\n  sortKeys: true\n  keepComments: false\nfetch:\n  timeoutSeconds: 10\n  retries: 1\n  concurrency: 2\n  ignoreFailed: false\n"
	profilePath := filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(profilePath, []byte(profileBody), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile read: %v", err)
	}

	res, err := Run(context.Background(), p, profilePath, logging.New(os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("build run: %v", err)
	}

	got, err := configfile.MarshalYAML(res.Config, true, true)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(got)

	checks := []string{
		"proxies:",
		"type: ss",
		"type: vmess",
		"type: socks5",
		"type: http",
		"tls: true",
		"name: sock5-node",
		"proxy-groups:",
		"name: PROXY",
		"MATCH,PROXY",
	}
	for _, needle := range checks {
		if !strings.Contains(s, needle) {
			t.Fatalf("expected output to contain %q, got:\n%s", needle, s)
		}
	}
}

func TestBuildFromMultipleNodesFilesMergesProxyGroup(t *testing.T) {
	dir := t.TempDir()

	nodesA := "socks5://a.example.com:1080#a-node\n"
	nodesB := "http://b.example.com:8080#b-node\n"
	if err := os.WriteFile(filepath.Join(dir, "nodes-a.txt"), []byte(nodesA), 0o644); err != nil {
		t.Fatalf("write nodes-a file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nodes-b.txt"), []byte(nodesB), 0o644); err != nil {
		t.Fatalf("write nodes-b file: %v", err)
	}

	profileBody := `
subscriptions:
  - nodesFile: ./nodes-a.txt
  - nodesFile: ./nodes-b.txt
output:
  deterministic: true
  sortKeys: true
  keepComments: false
fetch:
  timeoutSeconds: 10
  retries: 1
  concurrency: 2
  ignoreFailed: false
`
	profilePath := filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(profilePath, []byte(profileBody), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile read: %v", err)
	}
	res, err := Run(context.Background(), p, profilePath, logging.New(os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("build run: %v", err)
	}

	groups, ok := res.Config["proxy-groups"].([]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("missing proxy-groups in output")
	}
	var proxyGroup map[string]any
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := group["name"].(string); name == "PROXY" {
			proxyGroup = group
			break
		}
	}
	if proxyGroup == nil {
		t.Fatalf("missing PROXY group in output")
	}
	groupMembers, ok := proxyGroup["proxies"].([]any)
	if !ok {
		t.Fatalf("PROXY group proxies should be a sequence")
	}
	joined := strings.Join(toStringSlice(groupMembers), ",")
	for _, expected := range []string{"a-node", "b-node", "DIRECT"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected PROXY group to include %q, got %v", expected, groupMembers)
		}
	}
}

func toStringSlice(items []any) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
