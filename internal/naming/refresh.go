package naming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type RefreshOptions struct {
	Endpoint    string
	Timeout     time.Duration
	MaxAge      time.Duration
	Concurrency int
	ProxyURL    string
}

type RefreshStats struct{ Updated, Cached, Failed, Private int }

func Refresh(ctx context.Context, hosts []string, cache Cache, options RefreshOptions) (Cache, RefreshStats, error) {
	if options.Endpoint == "" {
		options.Endpoint = "https://ipwho.is/{ip}"
	}
	if !strings.Contains(options.Endpoint, "{ip}") {
		return cache, RefreshStats{}, fmt.Errorf("metadata endpoint must contain {ip}")
	}
	if options.Timeout <= 0 {
		options.Timeout = 4 * time.Second
	}
	if options.MaxAge <= 0 {
		options.MaxAge = 7 * 24 * time.Hour
	}
	if options.Concurrency <= 0 {
		options.Concurrency = 2
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if options.ProxyURL != "" {
		u, err := url.Parse(options.ProxyURL)
		if err != nil {
			return cache, RefreshStats{}, fmt.Errorf("invalid metadata proxy")
		}
		transport.Proxy = http.ProxyURL(u)
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: options.Timeout}
	if cache.Hosts == nil {
		cache.Hosts = map[string]Entry{}
	}
	stats := RefreshStats{}
	jobs := make(chan string)
	var mutex sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < options.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for host := range jobs {
				mutex.Lock()
				previous := cache.Hosts[host]
				mutex.Unlock()
				if len(previous.CountryCode) == 2 && time.Since(time.Unix(previous.UpdatedAt, 0)) < options.MaxAge {
					mutex.Lock()
					stats.Cached++
					mutex.Unlock()
					continue
				}
				lookupCtx, cancel := context.WithTimeout(ctx, options.Timeout)
				ip := net.ParseIP(host)
				if strings.EqualFold(host, "localhost") {
					ip = net.ParseIP("127.0.0.1")
				}
				if ip == nil {
					addresses, err := net.DefaultResolver.LookupIP(lookupCtx, "ip", host)
					if err == nil {
						sort.Slice(addresses, func(i, j int) bool {
							a, b := addresses[i].To4(), addresses[j].To4()
							if (a != nil) != (b != nil) {
								return a != nil
							}
							return addresses[i].String() < addresses[j].String()
						})
						if len(addresses) > 0 {
							ip = addresses[0]
						}
						for _, address := range addresses {
							if address.IsGlobalUnicast() && !address.IsPrivate() {
								ip = address
								break
							}
						}
					}
				}
				if ip == nil {
					cancel()
					mutex.Lock()
					stats.Failed++
					mutex.Unlock()
					continue
				}
				if !ip.IsGlobalUnicast() || ip.IsPrivate() {
					cancel()
					mutex.Lock()
					stats.Private++
					mutex.Unlock()
					continue
				}
				// Reuse a known IP when multiple node hostnames share an endpoint.
				mutex.Lock()
				known := cache.Hosts[ip.String()]
				mutex.Unlock()
				if len(known.CountryCode) == 2 && time.Since(time.Unix(known.UpdatedAt, 0)) < options.MaxAge {
					cancel()
					mutex.Lock()
					cache.Hosts[host] = known
					stats.Cached++
					mutex.Unlock()
					continue
				}
				entry, err := lookup(lookupCtx, client, options.Endpoint, ip.String())
				cancel()
				mutex.Lock()
				if err != nil {
					stats.Failed++
				} else {
					cache.Hosts[host] = entry
					cache.Hosts[ip.String()] = entry
					stats.Updated++
				}
				mutex.Unlock()
			}
		}()
	}
sendJobs:
	for _, host := range hosts {
		select {
		case jobs <- host:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	wg.Wait()
	return cache, stats, nil
}

func lookup(ctx context.Context, client *http.Client, endpoint, ip string) (Entry, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.ReplaceAll(endpoint, "{ip}", url.PathEscape(ip)), nil)
	if err != nil {
		return Entry{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return Entry{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Entry{}, fmt.Errorf("metadata HTTP status %d", response.StatusCode)
	}
	var result struct {
		Success     *bool  `json:"success"`
		IP          string `json:"ip"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		City        string `json:"city"`
		Connection  struct {
			ASN          int    `json:"asn"`
			Organization string `json:"org"`
			ISP          string `json:"isp"`
		} `json:"connection"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 128*1024)).Decode(&result); err != nil {
		return Entry{}, err
	}
	if result.Success != nil && !*result.Success {
		return Entry{}, fmt.Errorf("metadata lookup rejected")
	}
	if len(result.CountryCode) != 2 {
		return Entry{}, fmt.Errorf("metadata response has no country code")
	}
	if actual := net.ParseIP(result.IP); actual == nil || !actual.Equal(net.ParseIP(ip)) {
		return Entry{}, fmt.Errorf("metadata response IP does not match endpoint")
	}
	organization := result.Connection.Organization
	if organization == "" {
		organization = result.Connection.ISP
	}
	return Entry{IP: ip, CountryCode: strings.ToUpper(result.CountryCode), Country: Clean(result.Country), City: Clean(result.City), ASN: result.Connection.ASN, Organization: Clean(organization), Network: Clean(result.Connection.ISP), UpdatedAt: time.Now().Unix()}, nil
}
