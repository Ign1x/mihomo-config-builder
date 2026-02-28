package render

import "testing"

func TestApplyGamePlatformDirect(t *testing.T) {
	cfg := map[string]any{
		"dns": map[string]any{
			"enhanced-mode":  "fake-ip",
			"fake-ip-filter": []any{"+.existing.com"},
		},
		"rules": []any{"MATCH,PROXY"},
	}
	ApplyGamePlatformDirect(cfg, []string{"steam", "example.com"})
	rules := cfg["rules"].([]any)
	if len(rules) < 2 {
		t.Fatalf("expected prepended rules")
	}
	dns := cfg["dns"].(map[string]any)
	filters := dns["fake-ip-filter"].([]any)
	if len(filters) < 2 {
		t.Fatalf("expected fake-ip filters appended")
	}
}
