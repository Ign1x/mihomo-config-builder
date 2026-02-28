package validate

import "testing"

func TestConfigValidationAndWarnings(t *testing.T) {
	cfg := map[string]any{
		"proxies":      []any{map[string]any{"name": "A", "type": "ss"}},
		"proxy-groups": []any{map[string]any{"name": "AUTO", "type": "select", "proxies": []any{"A"}}},
		"rules":        []any{"MATCH,AUTO"},
		"dns": map[string]any{
			"enhanced-mode":  "fake-ip",
			"fake-ip-filter": []any{"+.lan"},
		},
	}
	warnings, err := Config(cfg)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected fake-ip warning")
	}
}
