package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ign1x/mihomo-config-builder/internal/build"
	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/logging"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
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
	fmt.Fprintln(w, "  mcb build -c profile.yaml -o config.yaml")
	fmt.Fprintln(w, "  mcb validate -f config.yaml")
	fmt.Fprintln(w, "  mcb diff -c profile.yaml --against old.yaml")
}

func runBuild(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var profilePath string
	var outPath string
	fs.StringVar(&profilePath, "c", "", "profile yaml path")
	fs.StringVar(&outPath, "o", "config.yaml", "output yaml path")
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
	logger := logging.New(stdout, stderr)

	timeout := time.Duration(p.Fetch.TimeoutSeconds)*time.Second + 5*time.Second
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
	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		return fmt.Errorf("write output file: %w", err)
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
