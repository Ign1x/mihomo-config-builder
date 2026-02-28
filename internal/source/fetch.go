package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	httpClient *http.Client
	retries    int
}

func New(timeout time.Duration, retries int) *Fetcher {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Fetcher{
		httpClient: &http.Client{Timeout: timeout},
		retries:    retries,
	}
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
	if ref.URL != "" {
		b, err := f.loadURL(ctx, ref.URL)
		if err != nil {
			return nil, fmt.Errorf("fetch subscription url %s: %w", util.RedactURL(ref.URL), err)
		}
		return b, nil
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
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		resp, err := f.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			continue
		}

		data, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("http status %d", resp.StatusCode)
			continue
		}
		if len(data) == 0 {
			lastErr = errors.New("empty response body")
			continue
		}
		return data, nil
	}
	if lastErr == nil {
		lastErr = errors.New("unknown network error")
	}
	return nil, lastErr
}

func isHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
