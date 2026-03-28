package source

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
	"github.com/ign1x/mihomo-config-builder/internal/util"
)

type Result struct {
	Index int
	Data  []byte
	Err   error
	Ref   profile.SourceRef
}

type Fetcher struct {
	httpClient     *http.Client
	retries        int
	defaultHeaders http.Header
}

const defaultRemoteUserAgent = "Clash-verge/v2.0.0"

func New(timeout time.Duration, retries int) *Fetcher {
	f, err := NewWithOptions(timeout, retries, "", "")
	if err != nil {
		panic(err)
	}
	return f
}

func NewWithOptions(timeout time.Duration, retries int, userAgent string, proxyURL string) (*Fetcher, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = defaultRemoteUserAgent
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if strings.TrimSpace(proxyURL) != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse proxy url: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxy)
	}

	return &Fetcher{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		retries: retries,
		defaultHeaders: http.Header{
			"User-Agent": []string{userAgent},
			"Accept":     []string{"application/json,text/plain,*/*"},
		},
	}, nil
}

func (f *Fetcher) LoadTemplate(ctx context.Context, template string, profilePath string) ([]byte, error) {
	if template == "" {
		return nil, nil
	}
	if isHTTP(template) {
		b, err := f.loadURL(ctx, template)
		if err != nil {
			return nil, fmt.Errorf("load template from url %s: %w", util.RedactURL(template), err)
		}
		return b, nil
	}
	path := template
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(profilePath), path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load template from file %q: %w", template, err)
	}
	return b, nil
}

func (f *Fetcher) LoadSubscriptions(ctx context.Context, p profile.Profile, profilePath string) []Result {
	results := make([]Result, len(p.Subscriptions))
	if len(p.Subscriptions) == 0 {
		return results
	}
	concurrency := p.Fetch.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	sem := make(chan struct{}, concurrency)
	wg := sync.WaitGroup{}

	for i, ref := range p.Subscriptions {
		i := i
		ref := ref
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data, err := f.loadOne(ctx, ref, profilePath)
			results[i] = Result{Index: i, Data: data, Err: err, Ref: ref}
		}()
	}
	wg.Wait()
	return results
}

func (f *Fetcher) loadOne(ctx context.Context, ref profile.SourceRef, profilePath string) ([]byte, error) {
	if ref.NodesFile != "" {
		path := ref.NodesFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(profilePath), ref.NodesFile)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read nodes file %q: %w", ref.NodesFile, err)
		}
		converted, err := f.convertNodesFileToSubscriptionYAML(ctx, b)
		if err != nil {
			return nil, fmt.Errorf("convert nodes file %q: %w", ref.NodesFile, err)
		}
		return converted, nil
	}
	if ref.URL != "" {
		fetchURL, sourceLabel := splitSubscriptionURLMetadata(ref.URL)
		b, err := f.loadURL(ctx, fetchURL)
		if err != nil {
			return nil, fmt.Errorf("fetch subscription url %s: %w", util.RedactURL(fetchURL), err)
		}
		normalized, err := normalizeSubscriptionPayload(b, sourceLabel)
		if err != nil {
			return nil, fmt.Errorf("parse subscription content from %s: %w", util.RedactURL(fetchURL), err)
		}
		return normalized, nil
	}
	path := ref.File
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(profilePath), ref.File)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read subscription file %q: %w", ref.File, err)
	}
	return b, nil
}

func (f *Fetcher) loadURL(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= f.retries; attempt++ {
		data, statusCode, err := f.loadURLWithHeaders(ctx, rawURL, f.defaultHeaders)
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		lastErr = err

		shouldRetry := true
		if isCompatibilityRetryStatus(statusCode) {
			shouldRetry = false
			data, compatStatus, compatErr := f.loadURLWithHeaders(ctx, rawURL, nil)
			if compatErr == nil {
				return data, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = compatErr
			if !isCompatibilityRetryStatus(compatStatus) {
				shouldRetry = true
			}
		}
		if !shouldRetry {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("unknown network error")
	}
	return nil, lastErr
}

func (f *Fetcher) loadURLWithHeaders(ctx context.Context, rawURL string, headers http.Header) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}
	if headers != nil {
		req.Header = headers.Clone()
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		return nil, 0, err
	}

	data, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	if closeErr != nil {
		return nil, resp.StatusCode, closeErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("http status %d", resp.StatusCode)
	}
	if len(data) == 0 {
		return nil, resp.StatusCode, errors.New("empty response body")
	}
	return data, resp.StatusCode, nil
}

func isCompatibilityRetryStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

func isHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

type vmessNode struct {
	Add  string `json:"add"`
	Port any    `json:"port"`
	ID   string `json:"id"`
	Aid  any    `json:"aid"`
	Net  string `json:"net"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	PS   string `json:"ps"`
	Scy  string `json:"scy"`
}

func (f *Fetcher) convertNodesFileToSubscriptionYAML(ctx context.Context, data []byte) ([]byte, error) {
	proxies := make([]map[string]any, 0)
	seenProxyNames := make(map[string]struct{})
	nameCounts := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := normalizeNodeLine(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lineProxies, err := f.parseNodesFileLine(ctx, line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		for _, proxy := range lineProxies {
			ensureUniqueProxyName(proxy, lineNo, seenProxyNames, nameCounts)
			proxies = append(proxies, proxy)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan nodes file: %w", err)
	}
	if len(proxies) == 0 {
		return nil, errors.New("nodes file has no valid nodes")
	}

	return buildSubscriptionYAMLFromProxies(proxies)
}

func buildSubscriptionYAMLFromProxies(proxies []map[string]any) ([]byte, error) {
	groupProxies := make([]any, 0, len(proxies)+1)
	for _, proxy := range proxies {
		name, _ := proxy["name"].(string)
		if name != "" {
			groupProxies = append(groupProxies, name)
		}
	}
	groupProxies = append(groupProxies, "DIRECT")

	rawProxies := make([]any, 0, len(proxies))
	for _, proxy := range proxies {
		rawProxies = append(rawProxies, proxy)
	}

	subscription := map[string]any{
		"proxies": rawProxies,
		"proxy-groups": []any{
			map[string]any{
				"name":    "PROXY",
				"type":    "select",
				"proxies": groupProxies,
			},
		},
		"rules": []any{"MATCH,PROXY"},
	}

	return configfile.MarshalYAML(subscription, true, true)
}

func (f *Fetcher) parseNodesFileLine(ctx context.Context, line string) ([]map[string]any, error) {
	if subSpec, ok := parseNodesFileSubscriptionSpec(line); ok {
		proxies, err := f.loadSubscriptionURLProxies(ctx, subSpec.URL)
		if err == nil {
			appendProxyNameSuffix(proxies, subSpec.NameSuffix)
			return proxies, nil
		}
		proxy, parseErr := parseNodeLine(line)
		if parseErr == nil {
			return []map[string]any{proxy}, nil
		}
		return nil, err
	}

	if !isHTTP(line) {
		proxy, err := parseNodeLine(line)
		if err != nil {
			return nil, err
		}
		return []map[string]any{proxy}, nil
	}

	proxy, err := parseNodeLine(line)
	if err == nil {
		return []map[string]any{proxy}, nil
	}

	proxies, subErr := f.loadSubscriptionURLProxies(ctx, line)
	if subErr == nil {
		return proxies, nil
	}
	return nil, err
}

type nodesFileSubscriptionSpec struct {
	URL        string
	NameSuffix string
}

func parseNodesFileSubscriptionSpec(line string) (nodesFileSubscriptionSpec, bool) {
	u, err := url.Parse(strings.TrimSpace(line))
	if err != nil {
		return nodesFileSubscriptionSpec{}, false
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return nodesFileSubscriptionSpec{}, false
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return nodesFileSubscriptionSpec{}, false
	}

	suffix := strings.TrimSpace(u.Fragment)
	u.Fragment = ""
	if !looksLikeSubscriptionURL(u) {
		return nodesFileSubscriptionSpec{}, false
	}
	return nodesFileSubscriptionSpec{
		URL:        u.String(),
		NameSuffix: suffix,
	}, true
}

func looksLikeSubscriptionURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	if strings.TrimSpace(u.RawQuery) != "" {
		return true
	}
	path := strings.TrimSpace(u.EscapedPath())
	if path != "" && path != "/" {
		return true
	}
	return strings.TrimSpace(u.Port()) == ""
}

func (f *Fetcher) loadSubscriptionURLProxies(ctx context.Context, rawURL string) ([]map[string]any, error) {
	body, err := f.loadURL(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription url %s: %w", util.RedactURL(rawURL), err)
	}
	proxies, err := parseSubscriptionPayloadProxies(body)
	if err != nil {
		return nil, fmt.Errorf("parse subscription content from %s: %w", util.RedactURL(rawURL), err)
	}
	return proxies, nil
}

func normalizeSubscriptionPayload(data []byte, sourceLabel string) ([]byte, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errors.New("subscription content is empty")
	}

	if cfg, err := configfile.DecodeYAMLBytes([]byte(trimmed)); err == nil {
		if err := normalizeSubscriptionConfigProxyNames(cfg, sourceLabel); err != nil {
			return nil, err
		}
		return configfile.MarshalYAML(cfg, true, true)
	}

	proxies, err := parseSubscriptionPayloadProxies([]byte(trimmed))
	if err != nil {
		return nil, err
	}
	normalizeProxyNamesBySource(proxies, sourceLabel)
	return buildSubscriptionYAMLFromProxies(proxies)
}

func splitSubscriptionURLMetadata(rawURL string) (fetchURL string, sourceLabel string) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL, ""
	}
	sourceLabel = decodeNodeNameFragment(u.Fragment)
	u.Fragment = ""
	return u.String(), sourceLabel
}

func normalizeSubscriptionConfigProxyNames(cfg map[string]any, sourceLabel string) error {
	trimmedLabel := strings.TrimSpace(sourceLabel)
	if trimmedLabel == "" {
		return nil
	}

	rawProxies, ok := cfg["proxies"].([]any)
	if !ok || len(rawProxies) == 0 {
		return nil
	}

	proxies := make([]map[string]any, 0, len(rawProxies))
	for i, rawProxy := range rawProxies {
		proxy, ok := rawProxy.(map[string]any)
		if !ok {
			return fmt.Errorf("proxies[%d] must be mapping", i)
		}
		proxies = append(proxies, proxy)
	}

	renameMap := normalizeProxyNamesBySource(proxies, trimmedLabel)
	rewriteProxyGroupRefs(cfg, renameMap)
	return nil
}

func normalizeProxyNamesBySource(proxies []map[string]any, sourceLabel string) map[string]string {
	trimmedLabel := strings.TrimSpace(sourceLabel)
	if trimmedLabel == "" {
		return map[string]string{}
	}

	counters := map[string]int{}
	renameMap := map[string]string{}
	for _, proxy := range proxies {
		oldName, _ := proxy["name"].(string)
		prefix := proxyRegionPrefix(oldName)
		if prefix == "" {
			prefix = "NODE"
		}
		if isDedicatedProxyName(oldName) {
			prefix += "专"
		}
		counters[prefix]++
		newName := fmt.Sprintf("%s-k%d-%s", prefix, counters[prefix], trimmedLabel)
		proxy["name"] = newName
		if strings.TrimSpace(oldName) != "" {
			if _, exists := renameMap[oldName]; !exists {
				renameMap[oldName] = newName
			}
		}
	}
	return renameMap
}

func rewriteProxyGroupRefs(cfg map[string]any, renameMap map[string]string) {
	if len(renameMap) == 0 {
		return
	}
	groups, ok := cfg["proxy-groups"].([]any)
	if !ok {
		return
	}
	for _, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		refs, ok := group["proxies"].([]any)
		if !ok {
			continue
		}
		for i, rawRef := range refs {
			name, ok := rawRef.(string)
			if !ok {
				continue
			}
			if renamed, exists := renameMap[name]; exists {
				refs[i] = renamed
			}
		}
		group["proxies"] = refs
	}
}

func proxyRegionPrefix(name string) string {
	raw := strings.TrimSpace(name)
	tokens := uppercaseAlphaNumTokens(raw)

	if strings.Contains(raw, "🇭🇰") || strings.Contains(raw, "香港") || hasToken(tokens, "HK") || (hasToken(tokens, "HONG") && hasToken(tokens, "KONG")) {
		return "HK"
	}
	if strings.Contains(raw, "🇯🇵") || strings.Contains(raw, "日本") || strings.Contains(raw, "东京") || strings.Contains(raw, "大阪") || hasToken(tokens, "JP") || hasToken(tokens, "JAPAN") {
		return "JP"
	}
	if strings.Contains(raw, "🇹🇼") || strings.Contains(raw, "台湾") || strings.Contains(raw, "台北") || hasToken(tokens, "TW") || hasToken(tokens, "TAIWAN") {
		return "TW"
	}
	if strings.Contains(raw, "🇸🇬") || strings.Contains(raw, "新加坡") || hasToken(tokens, "SG") || hasToken(tokens, "SINGAPORE") {
		return "SG"
	}
	if strings.Contains(raw, "🇰🇷") || strings.Contains(raw, "韩国") || strings.Contains(raw, "首尔") || hasToken(tokens, "KR") || hasToken(tokens, "KOREA") {
		return "KR"
	}
	if strings.Contains(raw, "🇻🇳") || strings.Contains(raw, "越南") || hasToken(tokens, "VN") || hasToken(tokens, "VIETNAM") {
		return "VN"
	}
	if strings.Contains(raw, "🇹🇭") || strings.Contains(raw, "泰国") || hasToken(tokens, "TH") || hasToken(tokens, "THAILAND") {
		return "TH"
	}
	if strings.Contains(raw, "🇵🇭") || strings.Contains(raw, "菲律宾") || hasToken(tokens, "PH") || hasToken(tokens, "PHILIPPINES") {
		return "PH"
	}
	if strings.Contains(raw, "🇮🇳") || strings.Contains(raw, "印度") || hasToken(tokens, "IN") || hasToken(tokens, "INDIA") {
		return "IN"
	}
	if strings.Contains(raw, "🇺🇸") || strings.Contains(raw, "美国") || hasToken(tokens, "US") || hasToken(tokens, "USA") || hasToken(tokens, "AMERICA") || (hasToken(tokens, "UNITED") && hasToken(tokens, "STATES")) {
		return "US"
	}
	if strings.Contains(raw, "🇬🇧") || strings.Contains(raw, "英国") || hasToken(tokens, "UK") || (hasToken(tokens, "UNITED") && hasToken(tokens, "KINGDOM")) {
		return "UK"
	}
	if strings.Contains(raw, "🇩🇪") || strings.Contains(raw, "德国") || hasToken(tokens, "DE") || hasToken(tokens, "GERMANY") {
		return "DE"
	}
	if strings.Contains(raw, "🇫🇷") || strings.Contains(raw, "法国") || hasToken(tokens, "FR") || hasToken(tokens, "FRANCE") {
		return "FR"
	}
	if strings.Contains(raw, "🇳🇱") || strings.Contains(raw, "荷兰") || hasToken(tokens, "NL") || hasToken(tokens, "NETHERLANDS") {
		return "NL"
	}
	if strings.Contains(raw, "🇨🇦") || strings.Contains(raw, "加拿大") || hasToken(tokens, "CA") || hasToken(tokens, "CANADA") {
		return "CA"
	}
	if strings.Contains(raw, "🇷🇺") || strings.Contains(raw, "俄罗斯") || hasToken(tokens, "RU") || hasToken(tokens, "RUSSIA") {
		return "RU"
	}
	if strings.Contains(raw, "欧洲") || hasToken(tokens, "EU") || hasToken(tokens, "EUROPE") {
		return "EU"
	}

	return ""
}

func isDedicatedProxyName(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	return strings.Contains(upper, "IEPL") || strings.Contains(upper, "IPLC") || strings.Contains(name, "专线") || strings.Contains(name, "专用")
}

func uppercaseAlphaNumTokens(value string) []string {
	parts := strings.FieldsFunc(strings.ToUpper(value), func(r rune) bool {
		return !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
	return parts
}

func hasToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}

func parseSubscriptionPayloadProxies(data []byte) ([]map[string]any, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errors.New("subscription content is empty")
	}

	if proxies, err := extractYAMLProxies([]byte(trimmed)); err == nil {
		return proxies, nil
	}
	if proxies, err := parseNodeLinesToProxies([]byte(trimmed)); err == nil {
		return proxies, nil
	}

	compact := compactWhitespace(trimmed)
	decoded, err := decodeNodeBase64(compact)
	if err != nil {
		return nil, errors.New("subscription content is neither yaml nor node links nor base64 payload")
	}

	if proxies, err := extractYAMLProxies([]byte(decoded)); err == nil {
		return proxies, nil
	}
	if proxies, err := parseNodeLinesToProxies([]byte(decoded)); err == nil {
		return proxies, nil
	}
	return nil, errors.New("decoded subscription payload has no valid proxies")
}

func extractYAMLProxies(data []byte) ([]map[string]any, error) {
	cfg, err := configfile.DecodeYAMLBytes(data)
	if err != nil {
		return nil, err
	}
	rawProxies, ok := cfg["proxies"].([]any)
	if !ok {
		return nil, errors.New("yaml has no proxies field")
	}
	if len(rawProxies) == 0 {
		return nil, errors.New("yaml proxies is empty")
	}

	proxies := make([]map[string]any, 0, len(rawProxies))
	for i, rawProxy := range rawProxies {
		m, ok := rawProxy.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("proxies[%d] must be mapping", i)
		}
		proxies = append(proxies, m)
	}
	return proxies, nil
}

func parseNodeLinesToProxies(data []byte) ([]map[string]any, error) {
	proxies := make([]map[string]any, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := normalizeNodeLine(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		proxy, err := parseNodeLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		proxies = append(proxies, proxy)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan node lines: %w", err)
	}
	if len(proxies) == 0 {
		return nil, errors.New("no valid node lines")
	}
	return proxies, nil
}

func appendProxyNameSuffix(proxies []map[string]any, suffix string) {
	trimmedSuffix := strings.TrimSpace(suffix)
	if trimmedSuffix == "" {
		return
	}
	for _, proxy := range proxies {
		baseName := ""
		if rawName, ok := proxy["name"].(string); ok {
			baseName = strings.TrimSpace(rawName)
		}
		if baseName == "" {
			baseName = "NODE"
		}
		proxy["name"] = baseName + " " + trimmedSuffix
	}
}

func compactWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func ensureUniqueProxyName(proxy map[string]any, lineNo int, seen map[string]struct{}, counts map[string]int) {
	name := ""
	if rawName, ok := proxy["name"].(string); ok {
		name = strings.TrimSpace(rawName)
	}
	if name == "" {
		name = fmt.Sprintf("NODE-%d", lineNo)
	}

	if _, exists := seen[name]; !exists {
		seen[name] = struct{}{}
		if _, ok := counts[name]; !ok {
			counts[name] = 1
		}
		proxy["name"] = name
		return
	}

	idx := counts[name]
	if idx < 1 {
		idx = 1
	}
	for {
		idx++
		candidate := fmt.Sprintf("%s-%d", name, idx)
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		counts[name] = idx
		proxy["name"] = candidate
		return
	}
}

func parseNodeLine(line string) (map[string]any, error) {
	sep := strings.Index(line, "://")
	if sep <= 0 {
		return nil, fmt.Errorf("unsupported node scheme")
	}
	scheme := strings.ToLower(strings.TrimSpace(line[:sep]))
	normalized := scheme + "://" + line[sep+3:]

	switch scheme {
	case "ss":
		return parseSS(normalized)
	case "vmess":
		return parseVMess(normalized)
	case "trojan":
		return parseTrojan(normalized)
	case "vless":
		return parseVLess(normalized)
	case "hysteria2", "hy2":
		return parseHysteria2(normalized)
	case "socks5", "socks", "sock5":
		return parseSocks(normalized)
	case "http", "https":
		return parseHTTPProxy(normalized)
	default:
		return nil, fmt.Errorf("unsupported node scheme %q", scheme)
	}
}

func parseURLHostPort(u *url.URL, scheme string) (string, int, error) {
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return "", 0, fmt.Errorf("invalid %s host: host is required", scheme)
	}
	portText := strings.TrimSpace(u.Port())
	if portText == "" {
		return "", 0, fmt.Errorf("invalid %s port: port is required", scheme)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, fmt.Errorf("invalid %s port: %w", scheme, err)
	}
	return host, port, nil
}

func parseSocks(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse socks url: %w", err)
	}
	host, port, err := parseURLHostPort(u, "socks")
	if err != nil {
		return nil, err
	}
	name := decodeNodeNameFragment(u.Fragment)
	if name == "" {
		name = "SOCKS5"
	}

	out := map[string]any{
		"name":   name,
		"type":   "socks5",
		"server": host,
		"port":   port,
		"udp":    true,
	}

	if u.User != nil {
		if username := u.User.Username(); username != "" {
			out["username"] = username
		}
		if password, ok := u.User.Password(); ok {
			out["password"] = password
		}
	}

	return out, nil
}

func parseHTTPProxy(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse http proxy url: %w", err)
	}
	host, port, err := parseURLHostPort(u, "http proxy")
	if err != nil {
		return nil, err
	}
	name := decodeNodeNameFragment(u.Fragment)
	if name == "" {
		name = "HTTP"
	}

	out := map[string]any{
		"name":   name,
		"type":   "http",
		"server": host,
		"port":   port,
	}

	if strings.EqualFold(u.Scheme, "https") {
		out["tls"] = true
	}

	if u.User != nil {
		if username := u.User.Username(); username != "" {
			out["username"] = username
		}
		if password, ok := u.User.Password(); ok {
			out["password"] = password
		}
	}

	return out, nil
}

func parseHysteria2(raw string) (map[string]any, error) {
	normalized := raw
	if strings.HasPrefix(normalized, "hy2://") {
		normalized = "hysteria2://" + strings.TrimPrefix(normalized, "hy2://")
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("parse hysteria2 url: %w", err)
	}
	host, port, err := parseURLHostPort(u, "hysteria2")
	if err != nil {
		return nil, err
	}

	name := decodeNodeNameFragment(u.Fragment)
	if name == "" {
		name = "Hysteria2"
	}

	password := ""
	if u.User != nil {
		password = u.User.Username()
	}
	if password == "" {
		password = strings.TrimSpace(u.Query().Get("password"))
	}
	if password == "" {
		return nil, errors.New("hysteria2 password is required")
	}

	query := u.Query()
	out := map[string]any{
		"name":     name,
		"type":     "hysteria2",
		"server":   host,
		"port":     port,
		"password": password,
	}

	if sni := strings.TrimSpace(query.Get("sni")); sni != "" {
		out["sni"] = sni
	}
	if strings.EqualFold(query.Get("insecure"), "1") || strings.EqualFold(query.Get("insecure"), "true") {
		out["skip-cert-verify"] = true
	}
	if alpnRaw := strings.TrimSpace(query.Get("alpn")); alpnRaw != "" {
		parts := strings.Split(alpnRaw, ",")
		alpn := make([]any, 0, len(parts))
		for _, part := range parts {
			p := strings.TrimSpace(part)
			if p != "" {
				alpn = append(alpn, p)
			}
		}
		if len(alpn) > 0 {
			out["alpn"] = alpn
		}
	}
	if obfs := strings.TrimSpace(query.Get("obfs")); obfs != "" {
		out["obfs"] = obfs
	}
	if obfsPassword := strings.TrimSpace(query.Get("obfs-password")); obfsPassword != "" {
		out["obfs-password"] = obfsPassword
	}

	return out, nil
}

func parseSS(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse ss url: %w", err)
	}

	name := decodeNodeNameFragment(u.Fragment)
	if name == "" {
		name = "SS"
	}

	server := ""
	port := 0
	cipher := ""
	password := ""

	if u.User != nil && u.Host != "" {
		username := strings.TrimSpace(u.User.Username())
		if username == "" {
			return nil, errors.New("invalid ss credential")
		}

		pwd, hasPassword := u.User.Password()
		if hasPassword {
			cipher = username
			password = pwd
		} else {
			if method, pass, ok := splitCipherPassword(username); ok {
				cipher = method
				password = pass
			} else {
				decoded, decodeErr := decodeNodeBase64(username)
				if decodeErr != nil {
					return nil, errors.New("invalid ss credential")
				}
				method, pass, ok := splitCipherPassword(decoded)
				if !ok {
					return nil, errors.New("invalid ss credential")
				}
				cipher = method
				password = pass
			}
		}
		server, port, err = parseURLHostPort(u, "ss")
		if err != nil {
			return nil, err
		}
	} else {
		base := strings.TrimPrefix(raw, "ss://")
		if idx := strings.Index(base, "#"); idx >= 0 {
			base = base[:idx]
		}
		if idx := strings.Index(base, "?"); idx >= 0 {
			base = base[:idx]
		}
		decoded, decodeErr := decodeNodeBase64(base)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode ss payload: %w", decodeErr)
		}
		at := strings.LastIndex(decoded, "@")
		if at < 0 {
			return nil, errors.New("invalid ss payload")
		}
		methodPwd := decoded[:at]
		hostPort := decoded[at+1:]
		colon := strings.Index(methodPwd, ":")
		if colon < 0 {
			return nil, errors.New("invalid ss cipher/password")
		}
		cipher = methodPwd[:colon]
		password = methodPwd[colon+1:]
		host, portStr, splitErr := net.SplitHostPort(hostPort)
		if splitErr != nil {
			return nil, fmt.Errorf("invalid ss host: %w", splitErr)
		}
		server = host
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid ss port: %w", err)
		}
	}

	if strings.TrimSpace(server) == "" {
		return nil, errors.New("ss server is required")
	}
	if port <= 0 {
		return nil, errors.New("ss port is required")
	}
	if strings.TrimSpace(cipher) == "" {
		return nil, errors.New("ss cipher is required")
	}

	return map[string]any{
		"name":     name,
		"type":     "ss",
		"server":   server,
		"port":     port,
		"cipher":   cipher,
		"password": password,
	}, nil
}

func parseVMess(raw string) (map[string]any, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	decoded, err := decodeNodeBase64(payload)
	if err != nil {
		return nil, fmt.Errorf("decode vmess payload: %w", err)
	}

	var n vmessNode
	if err := json.Unmarshal([]byte(decoded), &n); err != nil {
		return nil, fmt.Errorf("unmarshal vmess json: %w", err)
	}

	port, err := parseIntLike(n.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid vmess port: %w", err)
	}
	if port <= 0 {
		return nil, errors.New("vmess port is required")
	}
	aid, _ := parseIntLike(n.Aid)
	server := strings.TrimSpace(n.Add)
	if server == "" {
		return nil, errors.New("vmess server is required")
	}
	uuid := strings.TrimSpace(n.ID)
	if uuid == "" {
		return nil, errors.New("vmess uuid is required")
	}

	name := strings.TrimSpace(n.PS)
	if name == "" {
		name = "VMess"
	}

	out := map[string]any{
		"name":    name,
		"type":    "vmess",
		"server":  server,
		"port":    port,
		"uuid":    uuid,
		"alterId": aid,
		"cipher":  firstNonEmpty(n.Scy, "auto"),
		"tls":     strings.EqualFold(n.TLS, "tls") || strings.EqualFold(n.TLS, "1") || strings.EqualFold(n.TLS, "true"),
		"udp":     true,
	}

	network := strings.TrimSpace(n.Net)
	if network != "" {
		out["network"] = network
	}

	if network == "ws" {
		wsOpts := map[string]any{}
		if path := strings.TrimSpace(n.Path); path != "" {
			wsOpts["path"] = path
		}
		if host := strings.TrimSpace(n.Host); host != "" {
			wsOpts["headers"] = map[string]any{"Host": host}
		}
		if len(wsOpts) > 0 {
			out["ws-opts"] = wsOpts
		}
	}

	if sni := strings.TrimSpace(firstNonEmpty(n.SNI, n.Host)); sni != "" {
		out["servername"] = sni
	}

	return out, nil
}

func parseTrojan(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse trojan url: %w", err)
	}
	host, port, err := parseURLHostPort(u, "trojan")
	if err != nil {
		return nil, err
	}
	name := decodeNodeNameFragment(u.Fragment)
	if name == "" {
		name = "Trojan"
	}
	password := ""
	if u.User != nil {
		password = strings.TrimSpace(u.User.Username())
	}
	if password == "" {
		return nil, errors.New("trojan password is required")
	}

	out := map[string]any{
		"name":     name,
		"type":     "trojan",
		"server":   host,
		"port":     port,
		"password": password,
		"udp":      true,
	}

	query := u.Query()
	if sni := strings.TrimSpace(query.Get("sni")); sni != "" {
		out["sni"] = sni
	} else {
		out["sni"] = host
	}
	if strings.EqualFold(query.Get("allowInsecure"), "1") || strings.EqualFold(query.Get("allowInsecure"), "true") {
		out["skip-cert-verify"] = true
	}

	if strings.EqualFold(query.Get("type"), "ws") {
		wsOpts := map[string]any{}
		if path := query.Get("path"); path != "" {
			wsOpts["path"] = path
		}
		if hostHeader := query.Get("host"); hostHeader != "" {
			wsOpts["headers"] = map[string]any{"Host": hostHeader}
		}
		if len(wsOpts) > 0 {
			out["network"] = "ws"
			out["ws-opts"] = wsOpts
		}
	}

	return out, nil
}

func parseVLess(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse vless url: %w", err)
	}
	host, port, err := parseURLHostPort(u, "vless")
	if err != nil {
		return nil, err
	}
	name := decodeNodeNameFragment(u.Fragment)
	if name == "" {
		name = "VLESS"
	}
	uuid := ""
	if u.User != nil {
		uuid = strings.TrimSpace(u.User.Username())
	}
	if uuid == "" {
		return nil, errors.New("vless uuid is required")
	}

	query := u.Query()
	network := firstNonEmpty(query.Get("type"), query.Get("net"))
	if network == "" {
		network = "tcp"
	}
	security := firstNonEmpty(query.Get("security"), "none")

	out := map[string]any{
		"name":       name,
		"type":       "vless",
		"server":     host,
		"port":       port,
		"uuid":       uuid,
		"network":    network,
		"tls":        strings.EqualFold(security, "tls") || strings.EqualFold(security, "reality"),
		"udp":        true,
		"servername": firstNonEmpty(query.Get("sni"), host),
	}

	if flow := strings.TrimSpace(query.Get("flow")); flow != "" {
		out["flow"] = flow
	}
	if fp := strings.TrimSpace(query.Get("fp")); fp != "" {
		out["client-fingerprint"] = fp
	}
	if pbk := strings.TrimSpace(query.Get("pbk")); pbk != "" {
		realityOpts := map[string]any{"public-key": pbk}
		if sid := strings.TrimSpace(query.Get("sid")); sid != "" {
			realityOpts["short-id"] = sid
		}
		out["reality-opts"] = realityOpts
	}

	if network == "ws" {
		wsOpts := map[string]any{}
		if path := query.Get("path"); path != "" {
			wsOpts["path"] = path
		}
		if hostHeader := query.Get("host"); hostHeader != "" {
			wsOpts["headers"] = map[string]any{"Host": hostHeader}
		}
		if len(wsOpts) > 0 {
			out["ws-opts"] = wsOpts
		}
	}

	return out, nil
}

func decodeNodeBase64(s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	trimmed = strings.ReplaceAll(trimmed, "-", "+")
	trimmed = strings.ReplaceAll(trimmed, "_", "/")
	if mod := len(trimmed) % 4; mod != 0 {
		trimmed += strings.Repeat("=", 4-mod)
	}

	if b, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return string(b), nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil {
		return string(b), nil
	}
	if b, err := base64.URLEncoding.DecodeString(trimmed); err == nil {
		return string(b), nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(trimmed); err == nil {
		return string(b), nil
	}

	return "", errors.New("invalid base64 payload")
}

func parseIntLike(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(t))
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizeNodeLine(line string) string {
	trimmed := strings.TrimSpace(line)
	return strings.TrimPrefix(trimmed, "\uFEFF")
}

func decodeNodeNameFragment(fragment string) string {
	name := strings.TrimSpace(fragment)
	if name == "" {
		return ""
	}
	decoded, err := url.PathUnescape(name)
	if err != nil {
		return name
	}
	return strings.TrimSpace(decoded)
}

func splitCipherPassword(value string) (cipher string, password string, ok bool) {
	raw := strings.TrimSpace(value)
	sep := strings.Index(raw, ":")
	if sep <= 0 || sep == len(raw)-1 {
		return "", "", false
	}
	return raw[:sep], raw[sep+1:], true
}
