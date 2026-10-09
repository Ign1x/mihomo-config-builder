package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAtomicPrivateAndFailedRefreshKeepsOutput(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	output := filepath.Join(dir, "config.yaml")
	good := []byte("proxies:\n - {name: test, type: socks5, server: 127.0.0.1, port: 1080}\nproxy-groups: []\nrules: []\n")
	if err := os.WriteFile(filepath.Join(dir, "source.yaml"), good, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte("subscriptions:\n - file: source.yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"build", "-c", profilePath, "-o", output, "--offline"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(output)
	if info.Mode().Perm() != 0600 {
		t.Fatal("config is not private")
	}
	os.WriteFile(filepath.Join(dir, "source.yaml"), []byte("proxies: broken\n"), 0600)
	if err := Run([]string{"build", "-c", profilePath, "-o", output}, &stdout, &stderr); err == nil {
		t.Fatal("bad config accepted")
	}
	after, _ := os.ReadFile(output)
	if !bytes.Equal(before, after) {
		t.Fatal("failed refresh damaged previous config")
	}
	os.WriteFile(filepath.Join(dir, "source.yaml"), good, 0600)
	symlink := filepath.Join(dir, "symlink.yaml")
	if err := os.Symlink(output, symlink); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"build", "-c", profilePath, "-o", symlink}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatal("followed output symlink")
	}
}
