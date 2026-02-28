package validate

import (
	"fmt"
	"strings"
)

type Warning struct {
	Code    string
	Message string
}

func Config(cfg map[string]any) ([]Warning, error) {
	if err := requireTopLevel(cfg, "proxies", "proxy-groups", "rules"); err != nil {
		return nil, err
	}
	if err := ensureProxiesAndGroupsShape(cfg); err != nil {
		return nil, err
	}
	warnings := []Warning{}
	if w := fakeIPWarnings(cfg); len(w) > 0 {
		warnings = append(warnings, w...)
	}
	return warnings, nil
}

func requireTopLevel(cfg map[string]any, keys ...string) error {
	for _, k := range keys {
		if _, ok := cfg[k]; !ok {
			return fmt.Errorf("missing required top-level key: %s", k)
		}
	}
	return nil
}

func ensureProxiesAndGroupsShape(cfg map[string]any) error {
	proxies, ok := cfg["proxies"].([]any)
	if !ok {
		return fmt.Errorf("proxies must be a sequence")
	}
	for i, p := range proxies {
		m, ok := p.(map[string]any)
		if !ok {
			return fmt.Errorf("proxies[%d] must be a mapping", i)
		}
		if _, ok := m["name"].(string); !ok {
			return fmt.Errorf("proxies[%d].name must be string", i)
		}
	}
	groups, ok := cfg["proxy-groups"].([]any)
	if !ok {
		return fmt.Errorf("proxy-groups must be a sequence")
	}
	for i, g := range groups {
		m, ok := g.(map[string]any)
		if !ok {
			return fmt.Errorf("proxy-groups[%d] must be a mapping", i)
		}
		if _, ok := m["name"].(string); !ok {
			return fmt.Errorf("proxy-groups[%d].name must be string", i)
		}
		if _, ok := m["type"].(string); !ok {
			return fmt.Errorf("proxy-groups[%d].type must be string", i)
		}
	}
	if _, ok := cfg["rules"].([]any); !ok {
		return fmt.Errorf("rules must be a sequence")
	}
	return nil
}

func fakeIPWarnings(cfg map[string]any) []Warning {
	dns, ok := cfg["dns"].(map[string]any)
	if !ok {
		return nil
	}
	mode, _ := dns["enhanced-mode"].(string)
	if strings.ToLower(mode) != "fake-ip" {
		return nil
	}
	filters, _ := dns["fake-ip-filter"].([]any)
	knownConflicts := []string{"+.stun.", "+.steamcontent.com", "time.windows.com"}
	content := map[string]struct{}{}
	for _, item := range filters {
		if s, ok := item.(string); ok {
			content[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
		}
	}
	warnings := []Warning{}
	for _, expected := range knownConflicts {
		if _, ok := content[strings.ToLower(expected)]; !ok {
			warnings = append(warnings, Warning{
				Code:    "fake-ip-filter-missing",
				Message: fmt.Sprintf("dns.fake-ip-filter may miss recommended entry %q for compatibility", expected),
			})
		}
	}
	return warnings
}
