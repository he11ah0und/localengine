package localengine

import "strings"

// walkTree walks the nested map for the requested path and returns the value
// found there.
func walkTree(tree map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}
	current, ok := tree[path[0]]
	if !ok {
		return nil, false
	}
	for _, p := range path[1:] {
		sub, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = sub[p]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// lookup walks the nested map for the requested path and returns the string
// value found there.
func lookup(tree map[string]any, path []string) (string, bool) {
	v, ok := walkTree(tree, path)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// joinPath reconstructs a dotted path for error fallback messages.
func joinPath(path []string) string {
	return strings.Join(path, ".")
}

// Lookup returns the raw value at the given dotted path for the given language.
// The bool indicates whether the path exists in that language's bundle.
func Lookup(code string, path ...string) (any, bool) {
	b, ok := bundles[code]
	if !ok {
		return nil, false
	}
	if len(path) == 0 {
		return b, true
	}
	return walkTree(b, path)
}

// LookupString returns the string value at the given dotted path.
// Lookup order: the requested language, English, then static bundles.
// The bool indicates whether a non-empty string was found; a missing key
// is logged as a warning.
func LookupString(code string, path ...string) (string, bool) {
	langs := []string{code}
	if code != "en" {
		langs = append(langs, "en")
	}
	langs = append(langs, staticBundleNames()...)
	for _, lang := range langs {
		if v, ok := Lookup(lang, path...); ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	warnMissingOnce(path)
	return "", false
}

// LeafPaths returns all dotted leaf paths (paths that resolve to strings)
// for the given language code.
func LeafPaths(code string) []string {
	b, ok := bundles[code]
	if !ok {
		return nil
	}
	var out []string
	collectKeys(b, nil, func(p []string, _ any) {
		out = append(out, joinPath(p))
	})
	return out
}

// T returns the localized string for the given path segments.
// Lookup order: current language, English, static bundles, and finally
// the dotted path itself.
func T(path ...string) string {
	if msg, ok := LookupString(currentLang, path...); ok {
		return msg
	}
	return joinPath(path)
}

// collectKeys walks a locale tree and calls yield for every leaf path with
// the leaf value.
func collectKeys(tree map[string]any, path []string, yield func([]string, any)) {
	for k, v := range tree {
		p := keyPath(path, k)
		if sub, ok := v.(map[string]any); ok {
			collectKeys(sub, p, yield)
		} else {
			yield(p, v)
		}
	}
}

// keyPath returns a new path slice with key appended.
func keyPath(path []string, key string) []string {
	p := make([]string, len(path)+1)
	copy(p, path)
	p[len(path)] = key
	return p
}
