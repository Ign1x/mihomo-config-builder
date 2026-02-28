package source

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func TestLoadSubscriptionsFromFileAndURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "proxies: []\nproxy-groups: []\nrules: []\n")
	}))
	defer ts.Close()

	dir := t.TempDir()
	local := filepath.Join(dir, "sub.yaml")
	if err := os.WriteFile(local, []byte("proxies: []\nproxy-groups: []\nrules: []\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := New(5*time.Second, 1)
	p := profile.DefaultProfile()
	p.Subscriptions = []profile.SourceRef{
		{File: "sub.yaml"},
		{URL: ts.URL},
	}
	res := f.LoadSubscriptions(context.Background(), p, filepath.Join(dir, "profile.yaml"))
	if len(res) != 2 {
		t.Fatalf("unexpected result size")
	}
	for _, r := range res {
		if r.Err != nil {
			t.Fatalf("unexpected fetch error: %v", r.Err)
		}
	}
}

func TestLoadURLRetry(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 2 {
			http.Error(w, "bad", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	f := New(3*time.Second, 2)
	_, err := f.loadURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("expected retry success, got: %v", err)
	}
}
