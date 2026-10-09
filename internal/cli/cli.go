package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ign1x/mihomo-config-builder/internal/build"
	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/logging"
	"github.com/ign1x/mihomo-config-builder/internal/naming"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
	"github.com/ign1x/mihomo-config-builder/internal/util"
	"github.com/ign1x/mihomo-config-builder/internal/validate"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}
	switch args[0] {
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "metadata":
		return runMetadata(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "mcb 0.3.1")
		return nil
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "mcb - mihomo config builder")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  mcb build -c profile.yaml -o config.yaml [--offline] [--metadata-output nodes.json]")
	fmt.Fprintln(w, "  mcb metadata refresh -f config.yaml --cache metadata.json [--proxy-url URL]")
	fmt.Fprintln(w, "  mcb metadata export -f config.yaml --cache metadata.json -o nodes.json")
	fmt.Fprintln(w, "  mcb validate -f config.yaml")
	fmt.Fprintln(w, "  mcb diff -c profile.yaml --against old.yaml")
}

func runBuild(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var profilePath string
	var outPath, metadataPath string
	var offline bool
	fs.StringVar(&profilePath, "c", "", "profile yaml path")
	fs.StringVar(&outPath, "o", "config.yaml", "output yaml path")
	fs.StringVar(&metadataPath, "metadata-output", "", "optional dashboard metadata JSON path")
	fs.BoolVar(&offline, "offline", false, "use cached remote sources only; never access the network")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if profilePath == "" {
		return errors.New("build requires -c profile.yaml")
	}

	p, err := profile.ReadFile(profilePath)
	if err != nil {
		return err
	}
	p.Fetch.Offline = p.Fetch.Offline || offline
	logger := logging.New(stdout, stderr)

	timeout := time.Duration((p.Fetch.TimeoutSeconds+1)*(p.Fetch.Retries+1)*(len(p.Subscriptions)+1))*time.Second + 5*time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := build.Run(ctx, p, profilePath, logger)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stderr, "[warn] %s: %s\n", w.Code, w.Message)
	}
	body, err := configfile.MarshalYAML(res.Config, p.Output.Deterministic, p.Output.SortKeys)
	if err != nil {
		return err
	}
	if err := util.WritePrivateFile(outPath, body); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	if metadataPath != "" {
		data, err := json.MarshalIndent(res.Metadata, "", "  ")
		if err != nil {
			return err
		}
		if err := util.WritePrivateFile(metadataPath, append(data, '\n')); err != nil {
			return fmt.Errorf("write metadata: %w", err)
		}
	}
	fmt.Fprintf(stdout, "written %s\n", outPath)
	return nil
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var path string
	fs.StringVar(&path, "f", "", "config yaml path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if path == "" {
		return errors.New("validate requires -f config.yaml")
	}
	cfg, err := configfile.ReadYAMLFile(path)
	if err != nil {
		return err
	}
	warnings, err := validate.Config(cfg)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(stderr, "[warn] %s: %s\n", w.Code, w.Message)
	}
	fmt.Fprintln(stdout, "config is valid")
	return nil
}

func runDiff(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var profilePath string
	var againstPath string
	fs.StringVar(&profilePath, "c", "", "profile yaml path")
	fs.StringVar(&againstPath, "against", "", "existing config yaml path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if profilePath == "" || againstPath == "" {
		return errors.New("diff requires -c profile.yaml --against old.yaml")
	}

	p, err := profile.ReadFile(profilePath)
	if err != nil {
		return err
	}
	logger := logging.New(io.Discard, stderr)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	res, err := build.Run(ctx, p, profilePath, logger)
	if err != nil {
		return err
	}
	newBody, err := configfile.MarshalYAML(res.Config, p.Output.Deterministic, p.Output.SortKeys)
	if err != nil {
		return err
	}
	oldBody, err := os.ReadFile(againstPath)
	if err != nil {
		return fmt.Errorf("read against file: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(oldBody), bytes.TrimSpace(newBody)) {
		fmt.Fprintln(stdout, "no changes")
		return nil
	}
	fmt.Fprintln(stdout, "changed")
	fmt.Fprintln(stdout, "--- old")
	fmt.Fprintln(stdout, "+++ new")
	fmt.Fprintln(stdout, "(content differs; use external diff for full patch)")
	return nil
}

func runMetadata(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || (args[0] != "refresh" && args[0] != "export") {
		return errors.New("metadata requires refresh or export")
	}
	action := args[0]
	fs := flag.NewFlagSet("metadata "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var configPath, cachePath, outputPath, endpoint, proxyURL string
	var timeout time.Duration
	var maxAge time.Duration
	var concurrency int
	fs.StringVar(&configPath, "f", "", "generated config YAML path")
	fs.StringVar(&cachePath, "cache", "", "private metadata cache JSON path")
	fs.StringVar(&outputPath, "o", "", "dashboard JSON output (export only)")
	fs.StringVar(&endpoint, "endpoint", "https://ipwho.is/{ip}", "IP metadata API template")
	fs.StringVar(&proxyURL, "proxy-url", "", "optional HTTP proxy for metadata queries")
	fs.DurationVar(&timeout, "timeout", 4*time.Second, "per-host timeout")
	fs.DurationVar(&maxAge, "max-age", 7*24*time.Hour, "metadata freshness interval")
	fs.IntVar(&concurrency, "concurrency", 2, "parallel metadata lookups")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if configPath == "" || cachePath == "" {
		return errors.New("metadata requires -f config.yaml --cache metadata.json")
	}
	if action == "export" && outputPath == "" {
		return errors.New("metadata export requires -o nodes.json")
	}
	cfg, err := configfile.ReadYAMLFile(configPath)
	if err != nil {
		return err
	}
	cache, err := naming.ReadCache(cachePath)
	if err != nil {
		return fmt.Errorf("read metadata cache: %w", err)
	}
	if action == "export" {
		data, err := json.MarshalIndent(naming.Export(cfg, cache), "", "  ")
		if err != nil {
			return err
		}
		return util.WritePrivateFile(outputPath, append(data, '\n'))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	updated, stats, err := naming.Refresh(ctx, naming.Hosts(cfg), cache, naming.RefreshOptions{Endpoint: endpoint, ProxyURL: proxyURL, Timeout: timeout, MaxAge: maxAge, Concurrency: concurrency})
	if err != nil {
		return err
	}
	if err := naming.WriteCache(cachePath, updated); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "metadata: %d updated, %d cached, %d failed, %d private\n", stats.Updated, stats.Cached, stats.Failed, stats.Private)
	if stats.Failed > 0 {
		fmt.Fprintln(stderr, "[warn] Metadata lookup failures retained previous entries; configuration builds remain offline.")
	}
	return nil
}
