package naming

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ign1x/mihomo-config-builder/internal/merge"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func node(name, server string) map[string]any {
	return map[string]any{"name": name, "type": "anytls", "server": server, "port": 443, "password": "private-credential"}
}

func TestOptimizationLabelSurvivesMergeRenameAndYAMLRoundTrip(t *testing.T) {
	plain := node("ordinary", "1.1.1.1")
	optimized := node("香港 优化 private-source-label", "1.1.1.1")
	sameIP := node("ordinary peer", "1.1.1.1")
	sameIP["port"] = 8443
	before := Key(plain)
	cfg := map[string]any{"proxies": []any{plain, sameIP}}
	other := map[string]any{"proxies": []any{optimized}}
	Identify(cfg)
	Identify(other)
	if err := merge.SubscriptionInto(cfg, other); err != nil {
		t.Fatal(err)
	}
	Apply(cfg, profile.NamingOptions{}, Cache{})
	proxies := cfg["proxies"].([]any)
	if len(proxies) != 2 || Key(proxies[0].(map[string]any)) != before {
		t.Fatal("presentation label changed endpoint identity or deduplication")
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var restored map[string]any
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	// Standalone metadata export must work after renaming, without any IP cache.
	metadata := Export(restored, Cache{})
	name := proxies[0].(map[string]any)["name"].(string)
	if len(metadata) != 1 || !metadata[name].Optimized {
		t.Fatalf("lost label or marked a peer merely sharing its IP: %+v", metadata)
	}
	if strings.Contains(string(data), "private-source-label") || strings.Contains(name, "优化") {
		t.Fatal("raw source label leaked into normalized configuration")
	}
	public, _ := json.Marshal(metadata)
	if strings.Contains(string(public), "private-credential") || strings.Contains(string(public), before) {
		t.Fatal("private data leaked into dashboard metadata")
	}
}

func TestOptimizationLabelDoesNotRequireNaming(t *testing.T) {
	cfg := map[string]any{"proxies": []any{node("英国 优化", "1.1.1.1"), node("英国 普通", "8.8.8.8")}}
	metadata := Export(cfg, Cache{})
	if len(metadata) != 1 || !metadata["英国 优化"].Optimized {
		t.Fatal("original source label not recognized")
	}
}
func TestNamingRetainsDuplicateLabelsAndRewritesReferences(t *testing.T) {
	a := node("same", "1.1.1.1")
	b := node("same", "8.8.8.8")
	cfg := map[string]any{"proxies": []any{a}, "proxy-groups": []any{map[string]any{"name": "select", "proxies": []any{"same"}}}, "rules": []any{"IP-CIDR,1.0.0.0/8,same,no-resolve"}, "dns": map[string]any{"nameserver": []any{"https://1.1.1.1/dns-query#same"}}}
	other := map[string]any{"proxies": []any{b}, "proxy-groups": []any{map[string]any{"name": "select", "proxies": []any{"same"}}}}
	Identify(cfg)
	Identify(other)
	if err := merge.SubscriptionInto(cfg, other); err != nil {
		t.Fatal(err)
	}
	cache := Cache{Hosts: map[string]Entry{"1.1.1.1": {CountryCode: "HK", City: "Hong Kong", Organization: "Test Network LLC"}, "8.8.8.8": {CountryCode: "HK", City: "Hong Kong", Organization: "Test Network LLC"}}}
	Apply(cfg, profile.NamingOptions{}, cache)
	proxies := cfg["proxies"].([]any)
	if len(proxies) != 2 {
		t.Fatal("nodes lost during merge")
	}
	an := proxies[0].(map[string]any)["name"].(string)
	bn := proxies[1].(map[string]any)["name"].(string)
	if !regexp.MustCompile(`^🇭🇰 香港 0[12]$`).MatchString(an) || !regexp.MustCompile(`^🇭🇰 香港 0[12]$`).MatchString(bn) {
		t.Fatalf("unreadable node labels: %q %q", an, bn)
	}
	if an == bn || !strings.HasPrefix(an, "🇭🇰 香港") || strings.Contains(an, "anytls") || strings.Contains(an, "Test Network") {
		t.Fatalf("bad name: %q", an)
	}
	refs := cfg["proxy-groups"].([]any)[0].(map[string]any)["proxies"].([]any)
	if len(refs) != 2 || refs[0] != an || refs[1] != bn {
		t.Fatalf("bad refs: %v", refs)
	}
	if cfg["rules"].([]any)[0] != "IP-CIDR,1.0.0.0/8,"+an+",no-resolve" {
		t.Fatal("rule target lost")
	}
	if cfg["dns"].(map[string]any)["nameserver"].([]any)[0] != "https://1.1.1.1/dns-query#"+an {
		t.Fatal("DNS selector lost")
	}
	output, _ := json.Marshal(Export(cfg, cache))
	if strings.Contains(string(output), "private-credential") || strings.Contains(string(output), "password") {
		t.Fatal("metadata contains credential")
	}
	if Export(cfg, cache)[an].Network != "Test Network" {
		t.Fatal("operator not exported")
	}
}

func TestNamingStableAcrossSourceOrder(t *testing.T) {
	cfg := map[string]any{"proxies": []any{node("one", "1.1.1.1"), node("two", "8.8.8.8")}}
	Apply(cfg, profile.NamingOptions{}, Cache{})
	proxies := cfg["proxies"].([]any)
	first := proxies[0].(map[string]any)["name"]
	second := proxies[1].(map[string]any)["name"]
	cfg["proxies"] = []any{proxies[1], proxies[0]}
	Apply(cfg, profile.NamingOptions{}, Cache{})
	if proxies[0].(map[string]any)["name"] != first || proxies[1].(map[string]any)["name"] != second {
		t.Fatal("names depend on order")
	}
}

func TestRefreshPrivateAndFailureKeepPreviousMetadata(t *testing.T) {
	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/1.1.1.1" {
			http.Error(w, "failed", 503)
			return
		}
		w.Write([]byte(`{"success":true,"ip":"1.1.1.1","country_code":"HK","country":"Hong Kong","city":"Hong Kong","connection":{"asn":123,"org":"Example LLC","isp":"Carrier Network Inc."}}`))
	}))
	defer ts.Close()
	old := Entry{CountryCode: "US", City: "Old City", UpdatedAt: 1}
	cache, stats, err := Refresh(context.Background(), []string{"1.1.1.1", "8.8.8.8", "127.0.0.1", "10.0.0.1", "::1"}, Cache{Hosts: map[string]Entry{"8.8.8.8": old}}, RefreshOptions{Endpoint: ts.URL + "/{ip}", Timeout: time.Second})
	if err != nil || stats.Updated != 1 || stats.Failed != 1 || stats.Private != 3 || requests.Load() != 2 {
		t.Fatalf("stats: %+v, requests %d, %v", stats, requests.Load(), err)
	}
	if cache.Hosts["8.8.8.8"] != old || cache.Hosts["1.1.1.1"].ASN != 123 || Network(cache.Hosts["1.1.1.1"]) != "Carrier Network" {
		t.Fatal("previous cache lost or lookup failed")
	}
	before := requests.Load()
	_, stats, err = Refresh(context.Background(), []string{"1.1.1.1"}, cache, RefreshOptions{Endpoint: ts.URL + "/{ip}"})
	if err != nil || stats.Cached != 1 || requests.Load() != before {
		t.Fatal("fresh cache still queried API")
	}
}

func TestKnownCitiesHaveReadableChineseNames(t *testing.T) {
	for city, translated := range map[string]string{"Mumbai": "孟买", "Moscow": "莫斯科", "San Francisco": "旧金山", "Hanoi": "河内", "Gelnhausen": "盖尔恩豪森", "Kerkrade": "凯尔克拉德", "Bochum": "波鸿"} {
		got := Display(Entry{CountryCode: "US", City: city}, "1.1.1.1", false)
		if !strings.Contains(got, translated) || strings.Contains(got, city) {
			t.Fatalf("%s: %s", city, got)
		}
	}
}

func TestNumberedNamesAvoidGroupCollisions(t *testing.T) {
	cfg := map[string]any{"proxies": []any{node("old", "1.1.1.1")}, "proxy-groups": []any{map[string]any{"name": "🌐 未定位"}, map[string]any{"name": "🌐 未定位 01"}}}
	Apply(cfg, profile.NamingOptions{}, Cache{})
	if got := cfg["proxies"].([]any)[0].(map[string]any)["name"]; got != "🌐 未定位 02" {
		t.Fatalf("group collision: %s", got)
	}
}
