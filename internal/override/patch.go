package override

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/ign1x/mihomo-config-builder/internal/configfile"
	"github.com/ign1x/mihomo-config-builder/internal/profile"
)

func ApplyAll(base map[string]any, p profile.Profile, profilePath string) error {
	for _, f := range p.Overrides.Files {
		abs := f
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(filepath.Dir(profilePath), f)
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("read override file %q: %w", f, err)
		}
		data, err := configfile.DecodeYAMLBytesAny(content)
		if err != nil {
			return fmt.Errorf("decode override file %q: %w", f, err)
		}
		patchMap, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("override file %q must be yaml mapping", f)
		}
		if err := applyYAMLMerge(base, patchMap); err != nil {
			return err
		}
	}

	for i, ps := range p.Overrides.Patches {
		if ps.Enabled != nil && !*ps.Enabled {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(ps.Type))
		switch typ {
		case "yaml-merge", "merge", "yaml_merge":
			m, ok := ps.Patch.(map[string]any)
			if !ok {
				return fmt.Errorf("patch[%d]: yaml-merge requires mapping patch", i)
			}
			if err := applyYAMLMerge(base, m); err != nil {
				return fmt.Errorf("patch[%d]: %w", i, err)
			}
		case "json-patch", "json_patch":
			if err := applyJSONPatch(base, ps.Patch); err != nil {
				return fmt.Errorf("patch[%d]: %w", i, err)
			}
		case "strategy":
			if err := applyStrategy(base, ps); err != nil {
				return fmt.Errorf("patch[%d]: %w", i, err)
			}
		default:
			return fmt.Errorf("patch[%d]: unsupported type %q", i, ps.Type)
		}
	}
	return nil
}

func applyYAMLMerge(base map[string]any, patch map[string]any) error {
	merged, err := deepMerge(base, patch)
	if err != nil {
		return err
	}
	for k := range base {
		delete(base, k)
	}
	for k, v := range merged {
		base[k] = v
	}
	return nil
}

func deepMerge(dst map[string]any, patch map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range patch {
		if v == nil {
			delete(out, k)
			continue
		}
		if existing, ok := out[k]; ok {
			dm, okD := existing.(map[string]any)
			pm, okP := v.(map[string]any)
			if okD && okP {
				m, err := deepMerge(dm, pm)
				if err != nil {
					return nil, err
				}
				out[k] = m
				continue
			}
		}
		out[k] = v
	}
	return out, nil
}

func applyJSONPatch(base map[string]any, patch any) error {
	orig, err := json.Marshal(base)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal json patch: %w", err)
	}
	op, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return fmt.Errorf("decode json patch: %w", err)
	}
	patched, err := op.Apply(orig)
	if err != nil {
		return fmt.Errorf("apply json patch: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(patched, &out); err != nil {
		return fmt.Errorf("decode patched config: %w", err)
	}
	for k := range base {
		delete(base, k)
	}
	for k, v := range out {
		base[k] = v
	}
	return nil
}

func applyStrategy(base map[string]any, ps profile.PatchSpec) error {
	target := strings.TrimSpace(ps.Target)
	action := strings.TrimSpace(ps.Action)
	if target == "" || action == "" {
		return fmt.Errorf("strategy patch requires target and action")
	}
	switch {
	case target == "rules" && action == "append":
		val, ok := ps.Value.(string)
		if !ok {
			return fmt.Errorf("rules append value must be string")
		}
		rules := anySlice(base["rules"])
		rules = append(rules, val)
		base["rules"] = rules
		return nil
	case target == "rules" && action == "prepend":
		val, ok := ps.Value.(string)
		if !ok {
			return fmt.Errorf("rules prepend value must be string")
		}
		rules := anySlice(base["rules"])
		rules = append([]any{val}, rules...)
		base["rules"] = rules
		return nil
	default:
		return fmt.Errorf("unsupported strategy target/action %s/%s", target, action)
	}
}

func anySlice(v any) []any {
	if v == nil {
		return []any{}
	}
	s, ok := v.([]any)
	if !ok {
		return []any{}
	}
	return s
}
