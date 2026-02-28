package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dop251/goja"
	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

type contextData struct {
	ProfilePath string `json:"profilePath"`
	SourceCount int    `json:"sourceCount"`
	NowUnix     int64  `json:"nowUnix"`
}

func Apply(base map[string]any, p profile.Profile, profilePath string) error {
	if len(p.Hooks.JS.Files) == 0 {
		return nil
	}
	timeout := time.Duration(p.Hooks.JS.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	ctx := contextData{
		ProfilePath: profilePath,
		SourceCount: len(p.Subscriptions),
		NowUnix:     time.Now().Unix(),
	}

	for _, f := range p.Hooks.JS.Files {
		abs := f
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(filepath.Dir(profilePath), f)
		}
		script, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("read js hook %q: %w", f, err)
		}
		next, err := runOne(string(script), base, ctx, timeout)
		if err != nil {
			return fmt.Errorf("run js hook %q: %w", f, err)
		}
		for k := range base {
			delete(base, k)
		}
		for k, v := range next {
			base[k] = v
		}
	}
	return nil
}

func runOne(script string, cfg map[string]any, ctx contextData, timeout time.Duration) (map[string]any, error) {
	rt := goja.New()
	if timeout > 0 {
		timer := time.AfterFunc(timeout, func() {
			rt.Interrupt("hook timeout")
		})
		defer timer.Stop()
	}

	if _, err := rt.RunString(script); err != nil {
		return nil, fmt.Errorf("evaluate script: %w", err)
	}
	fnv := rt.Get("mcbTransform")
	fn, ok := goja.AssertFunction(fnv)
	if !ok {
		return nil, fmt.Errorf("script must define function mcbTransform(config, ctx)")
	}
	res, err := fn(goja.Undefined(), rt.ToValue(cfg), rt.ToValue(ctx))
	if err != nil {
		return nil, fmt.Errorf("call mcbTransform: %w", err)
	}
	norm, err := configfile.Normalize(res.Export())
	if err != nil {
		return nil, fmt.Errorf("normalize hook return: %w", err)
	}
	m, ok := norm.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("mcbTransform must return an object")
	}
	return m, nil
}
