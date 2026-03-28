package build

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/hook"
	"github.com/ign1x/mihomo-config-builder/internal/logging"
	"github.com/ign1x/mihomo-config-builder/internal/merge"
	"github.com/ign1x/mihomo-config-builder/internal/override"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
	"github.com/ign1x/mihomo-config-builder/internal/render"
	"github.com/ign1x/mihomo-config-builder/internal/ruletemplate"
	"github.com/ign1x/mihomo-config-builder/internal/source"
	"github.com/ign1x/mihomo-config-builder/internal/validate"
)

type Result struct {
	Config   map[string]any
	Warnings []validate.Warning
}

func Run(ctx context.Context, p profile.Profile, profilePath string, logger *logging.Logger) (Result, error) {
	fetcher, err := source.NewWithOptions(
		time.Duration(p.Fetch.TimeoutSeconds)*time.Second,
		p.Fetch.Retries,
		p.Fetch.UserAgent,
		p.Fetch.ProxyURL,
	)
	if err != nil {
		return Result{}, err
	}

	var cfg map[string]any
	if p.Template != "" {
		tpl, err := fetcher.LoadTemplate(ctx, p.Template, profilePath)
		if err != nil {
			return Result{}, err
		}
		m, err := configfile.DecodeYAMLBytes(tpl)
		if err != nil {
			return Result{}, fmt.Errorf("decode template: %w", err)
		}
		cfg = m
	} else {
		cfg = map[string]any{}
	}

	results := fetcher.LoadSubscriptions(ctx, p, profilePath)
	successCount := 0
	for _, r := range results {
		if r.Err != nil {
			if p.Fetch.IgnoreFailed {
				logger.Warn("subscription #%d failed: %v", r.Index, r.Err)
				continue
			}
			return Result{}, r.Err
		}
		m, err := configfile.DecodeYAMLBytes(r.Data)
		if err != nil {
			if p.Fetch.IgnoreFailed {
				logger.Warn("subscription #%d decode failed: %v", r.Index, err)
				continue
			}
			return Result{}, fmt.Errorf("decode subscription #%d: %w", r.Index, err)
		}
		if err := merge.SubscriptionInto(cfg, m); err != nil {
			return Result{}, fmt.Errorf("merge subscription #%d: %w", r.Index, err)
		}
		successCount++
	}
	if successCount == 0 && len(p.Subscriptions) > 0 {
		return Result{}, errors.New("all subscriptions failed")
	}

	if err := override.ApplyAll(cfg, p, profilePath); err != nil {
		return Result{}, err
	}
	if err := ruletemplate.Apply(cfg, p.RuleTemplates); err != nil {
		return Result{}, err
	}
	render.ApplyGamePlatformDirect(cfg, p.Policy.GamePlatformDirect)
	if err := hook.Apply(cfg, p, profilePath); err != nil {
		return Result{}, err
	}

	warnings, err := validate.Config(cfg)
	if err != nil {
		return Result{}, err
	}

	return Result{Config: cfg, Warnings: warnings}, nil
}
