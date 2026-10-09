package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func TestSubscriptionCacheSurvivesFailureAndOfflineBuild(t *testing.T) {
	var mode, requests atomic.Int32
	good := "proxies:\n - {name: node, type: socks5, server: example.com, port: 1080}\nproxy-groups: []\nrules: []\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch mode.Load() {
		case 1:
			http.Error(w, "failed", 503)
		case 2:
			w.Write([]byte(`{"error":"expired account"}`))
		case 3:
			w.Write([]byte("proxies:\n - {name: broken}\n"))
		default:
			w.Write([]byte(good))
		}
	}))
	defer ts.Close()
	f := New(time.Second, 0)
	f.SetCache(t.TempDir(), false, nil)
	ref := profile.SourceRef{URL: ts.URL + "/private-token"}
	fresh, err := f.loadOne(context.Background(), ref, "profile.yaml")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f.cachePath(ref.URL))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions: %v %v", info, err)
	}
	for _, m := range []int32{1, 2, 3} {
		mode.Store(m)
		cached, err := f.loadOne(context.Background(), ref, "profile.yaml")
		if err != nil || string(cached) != string(fresh) {
			t.Fatalf("mode %d lost validated cache: %v", m, err)
		}
	}
	f.offline = true
	before := requests.Load()
	cached, err := f.loadOne(context.Background(), ref, "profile.yaml")
	if err != nil || string(cached) != string(fresh) || requests.Load() != before {
		t.Fatalf("offline fetch: %v", err)
	}
	_, err = f.loadOne(context.Background(), profile.SourceRef{URL: ts.URL + "/no-cache"}, "profile.yaml")
	if err == nil || requests.Load() != before {
		t.Fatal("offline miss accessed network or succeeded")
	}
}

func TestFailedSubscriptionIsNotConvertedToHTTPNode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failed", 503) }))
	defer ts.Close()
	f := New(time.Second, 0)
	if _, err := f.convertNodesFileToSubscriptionYAML(context.Background(), []byte(ts.URL+"/feed/secret\n")); err == nil {
		t.Fatal("subscription failure became a fake HTTP node")
	}
}

func TestNetworkErrorsDoNotExposeSubscriptionToken(t *testing.T) {
	f := New(time.Millisecond, 0)
	_, err := f.loadOne(context.Background(), profile.SourceRef{URL: "http://127.0.0.1:1/feed/private-token?token=secret-value#label"}, "profile.yaml")
	if err == nil {
		t.Fatal("expected network error")
	}
	if strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "secret-value") {
		t.Fatal("URL secret leaked")
	}
}
