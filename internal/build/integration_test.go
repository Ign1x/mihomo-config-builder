package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/logging"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func TestBuildSnapshot(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "integration")
	profilePath := filepath.Join(dir, "profile.yaml")
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile read: %v", err)
	}
	res, err := Run(context.Background(), p, profilePath, logging.New(os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("build run: %v", err)
	}
	got, err := configfile.MarshalYAML(res.Config, true, true)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "expected.yaml"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("snapshot mismatch\n--- got ---\n%s\n--- want ---\n%s", string(got), string(want))
	}
}

func TestBuildSnapshotWithTemplateAndHook(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "integration")
	profilePath := filepath.Join(dir, "profile-with-template-hook.yaml")
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile read: %v", err)
	}
	res, err := Run(context.Background(), p, profilePath, logging.New(os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("build run: %v", err)
	}
	got, err := configfile.MarshalYAML(res.Config, true, true)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "expected-with-template-hook.yaml"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("snapshot mismatch\n--- got ---\n%s\n--- want ---\n%s", string(got), string(want))
	}
}
