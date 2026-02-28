package override

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func TestApplyYAMLMergeAndJSONPatch(t *testing.T) {
	base := map[string]any{
		"dns": map[string]any{
			"enhanced-mode": "fake-ip",
		},
		"rules": []any{"MATCH,PROXY"},
	}
	p := profile.DefaultProfile()
	p.Overrides.Patches = []profile.PatchSpec{
		{
			Type: "yaml-merge",
			Patch: map[string]any{
				"dns": map[string]any{"enable": true},
			},
		},
		{
			Type: "json-patch",
			Patch: []any{
				map[string]any{"op": "replace", "path": "/rules/0", "value": "MATCH,DIRECT"},
			},
		},
	}
	if err := ApplyAll(base, p, "/tmp/profile.yaml"); err != nil {
		t.Fatalf("apply all failed: %v", err)
	}
	if base["rules"].([]any)[0].(string) != "MATCH,DIRECT" {
		t.Fatalf("unexpected rule")
	}
	dns := base["dns"].(map[string]any)
	if dns["enable"].(bool) != true {
		t.Fatalf("dns merge not applied")
	}
}

func TestApplyOverrideFile(t *testing.T) {
	dir := t.TempDir()
	overridePath := filepath.Join(dir, "override.yaml")
	body := "dns:\n  enhanced-mode: fake-ip\n"
	if err := os.WriteFile(overridePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	base := map[string]any{}
	p := profile.DefaultProfile()
	p.Overrides.Files = []string{"override.yaml"}
	if err := ApplyAll(base, p, filepath.Join(dir, "profile.yaml")); err != nil {
		t.Fatalf("apply all: %v", err)
	}
	dns, ok := base["dns"].(map[string]any)
	if !ok {
		t.Fatalf("dns not merged")
	}
	if dns["enhanced-mode"] != "fake-ip" {
		t.Fatalf("unexpected dns value")
	}
}
