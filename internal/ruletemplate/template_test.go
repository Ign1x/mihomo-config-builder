package ruletemplate

import "testing"

func TestApplyCNDirectTemplate(t *testing.T) {
	cfg := map[string]any{
		"rules": []any{"MATCH,PROXY"},
	}
	if err := Apply(cfg, []string{"cn-direct"}); err != nil {
		t.Fatalf("apply template: %v", err)
	}
	rules := cfg["rules"].([]any)
	if len(rules) < 2 {
		t.Fatalf("expected prepended rules")
	}
}

func TestApplySteamDirectEnhancedTemplate(t *testing.T) {
	cfg := map[string]any{
		"rules": []any{"MATCH,PROXY"},
		"dns": map[string]any{
			"enhanced-mode":  "fake-ip",
			"fake-ip-filter": []any{"+.lan"},
		},
	}
	if err := Apply(cfg, []string{"steam-direct-enhanced"}); err != nil {
		t.Fatalf("apply template: %v", err)
	}
	dns := cfg["dns"].(map[string]any)
	filters := dns["fake-ip-filter"].([]any)
	if len(filters) <= 1 {
		t.Fatalf("expected added fake-ip filter entries")
	}
}

func TestApplyUnknownTemplate(t *testing.T) {
	cfg := map[string]any{}
	if err := Apply(cfg, []string{"unknown"}); err == nil {
		t.Fatalf("expected unknown template error")
	}
}
