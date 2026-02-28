package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func TestApplyJSHook(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hook.js")
	script := `
function mcbTransform(config, ctx) {
  config["log-level"] = "debug";
  return config;
}
`
	if err := os.WriteFile(hookPath, []byte(script), 0o644); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	cfg := map[string]any{"proxies": []any{}, "proxy-groups": []any{}, "rules": []any{}}
	p := profile.DefaultProfile()
	p.Hooks.JS.Files = []string{"hook.js"}
	p.Hooks.JS.TimeoutMs = 1000

	if err := Apply(cfg, p, filepath.Join(dir, "profile.yaml")); err != nil {
		t.Fatalf("apply hook: %v", err)
	}
	if cfg["log-level"] != "debug" {
		t.Fatalf("hook not applied")
	}
}

func TestApplyJSHookInvalid(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hook.js")
	if err := os.WriteFile(hookPath, []byte(`function nope() {}`), 0o644); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	cfg := map[string]any{"proxies": []any{}, "proxy-groups": []any{}, "rules": []any{}}
	p := profile.DefaultProfile()
	p.Hooks.JS.Files = []string{"hook.js"}
	if err := Apply(cfg, p, filepath.Join(dir, "profile.yaml")); err == nil {
		t.Fatalf("expected error")
	}
}
