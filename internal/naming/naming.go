// Package naming enriches node presentation from a local metadata snapshot.
// It never resolves DNS or makes HTTP requests during configuration builds.
package naming

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/ign1x/mihomo-config-builder/internal/profile"
	"github.com/ign1x/mihomo-config-builder/internal/util"
)

type Entry struct {
	IP           string `json:"ip"`
	CountryCode  string `json:"countryCode"`
	Country      string `json:"country,omitempty"`
	City         string `json:"city,omitempty"`
	ASN          int    `json:"asn,omitempty"`
	Organization string `json:"organization,omitempty"`
	Network      string `json:"network,omitempty"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type Cache struct {
	Version int              `json:"version"`
	Hosts   map[string]Entry `json:"hosts"`
}

func ReadCache(path string) (Cache, error) {
	c := Cache{Version: 1, Hosts: map[string]Entry{}}
	if path == "" {
		return c, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Hosts == nil {
		c.Hosts = map[string]Entry{}
	}
	return c, nil
}

func WriteCache(path string, c Cache) error {
	c.Version = 1
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return util.WritePrivateFile(path, append(b, '\n'))
}

func Hosts(cfg map[string]any) []string {
	set := map[string]bool{}
	proxies, _ := cfg["proxies"].([]any)
	for _, raw := range proxies {
		p, _ := raw.(map[string]any)
		host, _ := p["server"].(string)
		host = strings.TrimSpace(host)
		if host != "" {
			set[host] = true
		}
	}
	hosts := make([]string, 0, len(set))
	for host := range set {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}

// Key ignores presentation so names remain stable across subscription label
// changes. Credentials stay inside the hash and are never exported as metadata.
func Key(p map[string]any) string {
	identity := map[string]any{}
	for k, v := range p {
		if k != "name" {
			identity[k] = v
		}
	}
	b, _ := json.Marshal(identity)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

// Identify runs before merging: two providers may use the same display name
// for different endpoints. Such endpoints must not be discarded as duplicates.
func Identify(cfg map[string]any) {
	proxies, _ := cfg["proxies"].([]any)
	renames := map[string]string{}
	for _, raw := range proxies {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		old, _ := p["name"].(string)
		name := "mcb:" + Key(p)
		p["name"] = name
		if _, exists := renames[old]; !exists {
			renames[old] = name
		}
	}
	RewriteReferences(cfg, renames)
}

var countries = map[string]string{
	"CN": "中国", "HK": "香港", "TW": "台湾", "MO": "澳门", "JP": "日本", "KR": "韩国",
	"SG": "新加坡", "US": "美国", "CA": "加拿大", "GB": "英国", "DE": "德国", "FR": "法国",
	"NL": "荷兰", "FI": "芬兰", "SE": "瑞典", "CH": "瑞士", "RU": "俄罗斯", "AU": "澳大利亚",
	"IN": "印度", "VN": "越南", "TH": "泰国", "MY": "马来西亚", "ID": "印度尼西亚", "PH": "菲律宾",
	"IE": "爱尔兰", "IT": "意大利", "ES": "西班牙", "TR": "土耳其", "BR": "巴西", "ZA": "南非",
}

func Flag(code string) string {
	code = strings.ToUpper(code)
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return "🌐"
	}
	return string([]rune{0x1F1E6 + rune(code[0]-'A'), 0x1F1E6 + rune(code[1]-'A')})
}

func Clean(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsControl(r) || r == ',' || r == '|' || unicode.IsSpace(r) }), " ")
}

var networkSuffix = regexp.MustCompile(`(?i)(,?\s+(incorporated|inc\.?|llc|ltd\.?|limited|corporation|corp\.?))+$`)

func Network(e Entry) string {
	raw := e.Network
	if raw == "" {
		raw = e.Organization
	}
	return strings.TrimSpace(networkSuffix.ReplaceAllString(Clean(raw), ""))
}

var cities = map[string]string{
	"hong kong": "香港", "singapore": "新加坡", "tokyo": "东京", "osaka": "大阪", "seoul": "首尔",
	"taipei": "台北", "los angeles": "洛杉矶", "san jose": "圣何塞", "seattle": "西雅图",
	"new york": "纽约", "chicago": "芝加哥", "dallas": "达拉斯", "london": "伦敦", "frankfurt am main": "法兰克福",
	"amsterdam": "阿姆斯特丹", "paris": "巴黎", "sydney": "悉尼", "toronto": "多伦多", "shanghai": "上海", "beijing": "北京",
}

func Display(e Entry, host string, includeNetwork bool) string {
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && (!ip.IsGlobalUnicast() || ip.IsPrivate())) {
		return "🏠 内网"
	}
	code := strings.ToUpper(e.CountryCode)
	place := countries[code]
	if place == "" {
		place = Clean(e.Country)
	}
	if place == "" {
		place = "未定位"
	}
	city := Clean(e.City)
	if translated := cities[strings.ToLower(city)]; translated != "" {
		city = translated
	}
	if city != "" && city != place {
		place += " · " + city
	}
	name := Flag(code) + " " + place
	if network := Network(e); includeNetwork && network != "" {
		name += " | " + network
	}
	return name
}

func Apply(cfg map[string]any, options profile.NamingOptions, cache Cache) {
	proxies, _ := cfg["proxies"].([]any)
	counts := map[string]int{}
	identities := map[string][]string{}
	for _, raw := range proxies {
		p, _ := raw.(map[string]any)
		host, _ := p["server"].(string)
		base := Display(cache.Hosts[host], host, options.IncludeNetwork)
		counts[base]++
		identities[base] = append(identities[base], Key(p))
	}
	reserved := map[string]bool{"DIRECT": true, "REJECT": true, "PASS": true, "GLOBAL": true}
	groups, _ := cfg["proxy-groups"].([]any)
	for _, raw := range groups {
		g, _ := raw.(map[string]any)
		name, _ := g["name"].(string)
		reserved[name] = true
	}
	renames := map[string]string{}
	for _, raw := range proxies {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		old, _ := p["name"].(string)
		host, _ := p["server"].(string)
		base := Display(cache.Hosts[host], host, options.IncludeNetwork)
		name := base
		key := Key(p)
		if counts[base] > 1 || reserved[name] {
			length := 4
			for {
				name = base + " · " + key[:length]
				unique := !reserved[name]
				for _, other := range identities[base] {
					if other != key && other[:length] == key[:length] {
						unique = false
						break
					}
				}
				if unique {
					break
				}
				length++
				if length > len(key) {
					name += "x"
					break
				}
			}
		}
		reserved[name] = true
		p["name"] = name
		renames[old] = name
	}
	RewriteReferences(cfg, renames)
}

func RewriteReferences(cfg map[string]any, names map[string]string) {
	rename := func(s string) string {
		if n, ok := names[s]; ok && s != "" {
			return n
		}
		return s
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, value := range x {
				if k == "dialer-proxy" || k == "proxy" || k == "detour" {
					if s, ok := value.(string); ok {
						x[k] = rename(s)
					}
				}
				if k == "proxies" {
					if refs, ok := value.([]any); ok {
						for i, raw := range refs {
							if s, ok := raw.(string); ok {
								refs[i] = rename(s)
							}
						}
					}
				}
				if k == "sub-rules" {
					if sub, ok := value.(map[string]any); ok {
						for name, rules := range sub {
							wrapper := map[string]any{"rules": rules}
							walk(wrapper)
							sub[name] = wrapper["rules"]
						}
					}
				}
				if k == "rules" {
					if rules, ok := value.([]any); ok {
						for i, raw := range rules {
							if s, ok := raw.(string); ok {
								fields := strings.Split(s, ",")
								index := len(fields) - 1
								if index >= 0 && fields[index] == "no-resolve" {
									index--
								}
								if index >= 1 {
									fields[index] = rename(fields[index])
									rules[i] = strings.Join(fields, ",")
								}
							}
						}
					}
				}
				// DNS selectors also occur in nameserver-policy and may
				// include options after the proxy, e.g. #node&h3=true.
				if k == "nameserver" || k == "fallback" || k == "proxy-server-nameserver" || k == "direct-nameserver" || k == "nameserver-policy" {
					var dns func(any) any
					dns = func(v any) any {
						switch v := v.(type) {
						case string:
							if j := strings.LastIndex(v, "#"); j >= 0 {
								parts := strings.Split(v[j+1:], "&")
								for i, part := range parts {
									parts[i] = rename(part)
								}
								return v[:j+1] + strings.Join(parts, "&")
							}
						case []any:
							for i, item := range v {
								v[i] = dns(item)
							}
						case map[string]any:
							for name, item := range v {
								v[name] = dns(item)
							}
						}
						return v
					}
					x[k] = dns(value)
				}
				walk(value)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(cfg)
}

// Export contains presentation only, never passwords, UUIDs or subscription URLs.
func Export(cfg map[string]any, cache Cache) map[string]Entry {
	out := map[string]Entry{}
	proxies, _ := cfg["proxies"].([]any)
	for _, raw := range proxies {
		p, _ := raw.(map[string]any)
		host, _ := p["server"].(string)
		name, _ := p["name"].(string)
		if e, ok := cache.Hosts[host]; ok {
			e.Network = Network(e)
			out[name] = e
		}
	}
	return out
}
