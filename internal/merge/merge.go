package merge

import (
	"fmt"
	"strings"
)

var appendListKeys = map[string]struct{}{
	"proxies":      {},
	"proxy-groups": {},
	"rules":        {},
}

var mapMergeKeys = map[string]struct{}{
	"rule-providers":  {},
	"proxy-providers": {},
}

func SubscriptionInto(base map[string]any, src map[string]any) error {
	for k, v := range src {
		if _, ok := appendListKeys[k]; ok {
			dst, err := toAnySlice(base[k])
			if err != nil {
				return fmt.Errorf("merge %s: %w", k, err)
			}
			add, err := toAnySlice(v)
			if err != nil {
				return fmt.Errorf("merge %s: %w", k, err)
			}
			base[k] = append(dst, deepCopySlice(add)...)
			continue
		}
		if _, ok := mapMergeKeys[k]; ok {
			dst, err := toStringAnyMap(base[k])
			if err != nil {
				return fmt.Errorf("merge %s: %w", k, err)
			}
			add, err := toStringAnyMap(v)
			if err != nil {
				return fmt.Errorf("merge %s: %w", k, err)
			}
			for mk, mv := range add {
				dst[mk] = DeepCopy(mv)
			}
			base[k] = dst
			continue
		}

		existing, ok := base[k]
		if !ok {
			base[k] = DeepCopy(v)
			continue
		}
		dstMap, okDst := existing.(map[string]any)
		srcMap, okSrc := v.(map[string]any)
		if okDst && okSrc {
			base[k] = mergeMapKeepExisting(dstMap, srcMap)
			continue
		}
	}

	DeduplicateProxyLike(base, "proxies")
	DeduplicateProxyLike(base, "proxy-groups")
	return nil
}

func mergeMapKeepExisting(dst map[string]any, src map[string]any) map[string]any {
	out := make(map[string]any, len(dst)+len(src))
	for k, v := range src {
		out[k] = DeepCopy(v)
	}
	for k, v := range dst {
		if existing, ok := out[k]; ok {
			dm, okD := v.(map[string]any)
			sm, okS := existing.(map[string]any)
			if okD && okS {
				out[k] = mergeMapKeepExisting(dm, sm)
				continue
			}
		}
		out[k] = DeepCopy(v)
	}
	return out
}

func DeduplicateProxyLike(root map[string]any, key string) {
	list, ok := root[key].([]any)
	if !ok {
		return
	}
	if key == "proxy-groups" {
		root[key] = deduplicateProxyGroups(list)
		return
	}
	seen := map[string]struct{}{}
	out := make([]any, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		name, _ := m["name"].(string)
		if name == "" {
			out = append(out, item)
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, item)
	}
	root[key] = out
}

func deduplicateProxyGroups(list []any) []any {
	indexByName := map[string]int{}
	out := make([]any, 0, len(list))

	for _, item := range list {
		group, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		name, _ := group["name"].(string)
		if name == "" {
			out = append(out, item)
			continue
		}

		if idx, exists := indexByName[name]; exists {
			existing, ok := out[idx].(map[string]any)
			if !ok {
				continue
			}
			mergeProxyGroup(existing, group)
			out[idx] = existing
			continue
		}

		indexByName[name] = len(out)
		out = append(out, item)
	}

	return out
}

func mergeProxyGroup(dst map[string]any, src map[string]any) {
	dstProxies, hasDstProxies := dst["proxies"].([]any)
	srcProxies, hasSrcProxies := src["proxies"].([]any)
	if !hasSrcProxies {
		return
	}
	if !hasDstProxies {
		dst["proxies"] = deepCopySlice(srcProxies)
		return
	}

	seen := map[string]struct{}{}
	merged := make([]any, 0, len(dstProxies)+len(srcProxies))
	appendUniqueProxyRefs(&merged, dstProxies, seen)
	appendUniqueProxyRefs(&merged, srcProxies, seen)
	dst["proxies"] = merged
}

func appendUniqueProxyRefs(dst *[]any, src []any, seen map[string]struct{}) {
	for _, item := range src {
		name, ok := item.(string)
		if !ok {
			*dst = append(*dst, DeepCopy(item))
			continue
		}
		key := strings.TrimSpace(name)
		if key == "" {
			*dst = append(*dst, name)
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		*dst = append(*dst, name)
	}
}

func toAnySlice(v any) ([]any, error) {
	if v == nil {
		return []any{}, nil
	}
	s, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence, got %T", v)
	}
	return s, nil
}

func toStringAnyMap(v any) (map[string]any, error) {
	if v == nil {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected mapping, got %T", v)
	}
	return m, nil
}

func DeepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			out[k] = DeepCopy(vv)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, vv := range t {
			out[i] = DeepCopy(vv)
		}
		return out
	default:
		return v
	}
}

func deepCopySlice(in []any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = DeepCopy(v)
	}
	return out
}
