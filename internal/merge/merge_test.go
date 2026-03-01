package merge

import "testing"

func TestSubscriptionInto(t *testing.T) {
	base := map[string]any{
		"proxies":      []any{map[string]any{"name": "A", "type": "ss"}},
		"proxy-groups": []any{map[string]any{"name": "AUTO", "type": "select", "proxies": []any{"A"}}},
		"rules":        []any{"MATCH,AUTO"},
	}
	src := map[string]any{
		"proxies": []any{map[string]any{"name": "B", "type": "ss"}},
		"rules":   []any{"DOMAIN-SUFFIX,example.com,DIRECT"},
	}
	if err := SubscriptionInto(base, src); err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if len(base["proxies"].([]any)) != 2 {
		t.Fatalf("expected 2 proxies")
	}
	if len(base["rules"].([]any)) != 2 {
		t.Fatalf("expected 2 rules")
	}
}

func TestDeduplicateProxyName(t *testing.T) {
	base := map[string]any{
		"proxies": []any{
			map[string]any{"name": "A", "type": "ss"},
			map[string]any{"name": "A", "type": "ss"},
		},
	}
	DeduplicateProxyLike(base, "proxies")
	if len(base["proxies"].([]any)) != 1 {
		t.Fatalf("expected deduped proxies")
	}
}

func TestSubscriptionIntoMergeDuplicateProxyGroup(t *testing.T) {
	base := map[string]any{
		"proxies": []any{
			map[string]any{"name": "A", "type": "ss"},
		},
		"proxy-groups": []any{
			map[string]any{"name": "PROXY", "type": "select", "proxies": []any{"A", "DIRECT"}},
		},
		"rules": []any{"MATCH,PROXY"},
	}
	src := map[string]any{
		"proxies": []any{
			map[string]any{"name": "B", "type": "ss"},
		},
		"proxy-groups": []any{
			map[string]any{"name": "PROXY", "type": "select", "proxies": []any{"B", "DIRECT"}},
		},
		"rules": []any{"MATCH,PROXY"},
	}

	if err := SubscriptionInto(base, src); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	groups := base["proxy-groups"].([]any)
	if len(groups) != 1 {
		t.Fatalf("expected one merged proxy-group, got %d", len(groups))
	}
	proxies := groups[0].(map[string]any)["proxies"].([]any)
	if len(proxies) != 3 {
		t.Fatalf("expected merged group proxies, got %v", proxies)
	}
	want := []string{"A", "DIRECT", "B"}
	for i, expected := range want {
		got, _ := proxies[i].(string)
		if got != expected {
			t.Fatalf("unexpected merged proxy order: got %v", proxies)
		}
	}
}
