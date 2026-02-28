package ruletemplate

import (
	"fmt"
	"strings"
)

type mutator func(map[string]any)

var templates = map[string]mutator{
	"cn-direct":             applyCNDirect,
	"steam-direct-enhanced": applySteamDirectEnhanced,
}

func Apply(cfg map[string]any, selected []string) error {
	for _, raw := range selected {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		m, ok := templates[name]
		if !ok {
			return fmt.Errorf("unknown rule template %q", raw)
		}
		m(cfg)
	}
	return nil
}

func applyCNDirect(cfg map[string]any) {
	prependRules(cfg, []string{
		"GEOSITE,cn,DIRECT",
		"GEOIP,CN,DIRECT",
		"DOMAIN-SUFFIX,cn,DIRECT",
	})
}

func applySteamDirectEnhanced(cfg map[string]any) {
	prependRules(cfg, []string{
		"DOMAIN-SUFFIX,steampowered.com,DIRECT",
		"DOMAIN-SUFFIX,steamcommunity.com,DIRECT",
		"DOMAIN-SUFFIX,steamcontent.com,DIRECT",
		"DOMAIN-SUFFIX,steamstatic.com,DIRECT",
		"DOMAIN-SUFFIX,steamserver.net,DIRECT",
	})
	appendFakeIPFilter(cfg, []string{
		"+.steampowered.com",
		"+.steamcommunity.com",
		"+.steamcontent.com",
		"+.steamstatic.com",
		"+.steamserver.net",
	})
}

func prependRules(cfg map[string]any, rules []string) {
	existing := toAnySlice(cfg["rules"])
	seen := map[string]struct{}{}
	for _, r := range existing {
		if s, ok := r.(string); ok {
			seen[s] = struct{}{}
		}
	}
	prepend := make([]any, 0, len(rules))
	for _, r := range rules {
		if _, ok := seen[r]; ok {
			continue
		}
		prepend = append(prepend, r)
	}
	if len(prepend) == 0 {
		return
	}
	cfg["rules"] = append(prepend, existing...)
}

func appendFakeIPFilter(cfg map[string]any, filters []string) {
	dns, ok := cfg["dns"].(map[string]any)
	if !ok {
		return
	}
	mode, _ := dns["enhanced-mode"].(string)
	if strings.ToLower(mode) != "fake-ip" {
		return
	}
	existing := toAnySlice(dns["fake-ip-filter"])
	seen := map[string]struct{}{}
	for _, f := range existing {
		if s, ok := f.(string); ok {
			seen[strings.ToLower(s)] = struct{}{}
		}
	}
	for _, f := range filters {
		if _, ok := seen[strings.ToLower(f)]; ok {
			continue
		}
		existing = append(existing, f)
		seen[strings.ToLower(f)] = struct{}{}
	}
	dns["fake-ip-filter"] = existing
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
