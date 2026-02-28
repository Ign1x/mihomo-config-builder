package render

import (
	"fmt"
	"strings"
)

var namedGamePlatforms = map[string][]string{
	"steam": {
		"steampowered.com",
		"steamcommunity.com",
		"steamstatic.com",
		"steamcontent.com",
		"steamserver.net",
	},
	"epic": {
		"epicgames.com",
		"unrealengine.com",
	},
	"xbox": {
		"xboxlive.com",
		"xbox.com",
		"xboxservices.com",
	},
}

func ApplyGamePlatformDirect(cfg map[string]any, entries []string) {
	if len(entries) == 0 {
		return
	}
	domains := expandDomains(entries)
	if len(domains) == 0 {
		return
	}
	prependRules(cfg, domains)
	appendFakeIPFilter(cfg, domains)
}

func expandDomains(entries []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(entries))
	for _, raw := range entries {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if values, ok := namedGamePlatforms[v]; ok {
			for _, d := range values {
				if _, exists := seen[d]; exists {
					continue
				}
				seen[d] = struct{}{}
				out = append(out, d)
			}
			continue
		}
		if strings.Contains(v, ".") {
			if _, exists := seen[v]; !exists {
				seen[v] = struct{}{}
				out = append(out, v)
			}
		}
	}
	return out
}

func prependRules(cfg map[string]any, domains []string) {
	rules := toAnySlice(cfg["rules"])
	ruleSeen := map[string]struct{}{}
	for _, r := range rules {
		if s, ok := r.(string); ok {
			ruleSeen[s] = struct{}{}
		}
	}
	prepend := make([]any, 0, len(domains))
	for _, domain := range domains {
		r := fmt.Sprintf("DOMAIN-SUFFIX,%s,DIRECT", domain)
		if _, exists := ruleSeen[r]; exists {
			continue
		}
		prepend = append(prepend, r)
	}
	if len(prepend) == 0 {
		return
	}
	cfg["rules"] = append(prepend, rules...)
}

func appendFakeIPFilter(cfg map[string]any, domains []string) {
	dns, ok := cfg["dns"].(map[string]any)
	if !ok {
		return
	}
	mode, _ := dns["enhanced-mode"].(string)
	if strings.ToLower(mode) != "fake-ip" {
		return
	}
	filters := toAnySlice(dns["fake-ip-filter"])
	seen := map[string]struct{}{}
	for _, f := range filters {
		if s, ok := f.(string); ok {
			seen[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
		}
	}
	for _, d := range domains {
		token := "+." + d
		if _, exists := seen[token]; exists {
			continue
		}
		filters = append(filters, token)
		seen[token] = struct{}{}
	}
	dns["fake-ip-filter"] = filters
	cfg["dns"] = dns
}

func toAnySlice(v any) []any {
	if v == nil {
		return []any{}
	}
	s, ok := v.([]any)
	if !ok {
		return []any{}
	}
	return s
}
