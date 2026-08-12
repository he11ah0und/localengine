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

// splitPath normalizes path segments: each segment is split on dots, so
// T("a.b.c") and T("a", "b", "c") address the same key. Empty parts are
// dropped.
func splitPath(path []string) []string {
	out := make([]string, 0, len(path))
	for _, seg := range path {
		for part := range strings.SplitSeq(seg, ".") {
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// Lookup returns the raw value at the given dotted path for the given
// language using the default Store.
func Lookup(code string, path ...string) (any, bool) {
	return defaultStore.Lookup(code, path...)
}

// LookupString returns the string value at the given dotted path using the
// default Store.
func LookupString(code string, path ...string) (string, bool) {
	return defaultStore.LookupString(code, path...)
}

// LeafPaths returns all dotted leaf paths (paths that resolve to strings)
// for the given language code using the default Store.
func LeafPaths(code string) []string {
	return defaultStore.LeafPaths(code)
}

// T returns the localized string for the given path segments using the
// default Store.
func T(path ...string) string {
	return defaultStore.T(path...)
}

// Lookup returns the raw value at the given dotted path for the given language:
// the loaded bundle first, then the built-in defaults for that language.
// The bool indicates whether the path exists in either layer.
func (s *Store) Lookup(code string, path ...string) (any, bool) {
	_ = s.ensureDefaults()
	path = splitPath(path)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, tree := range []map[string]any{s.bundles[code], s.defaults[code]} {
		if tree == nil {
			continue
		}
		if len(path) == 0 {
			return tree, true
		}
		if v, ok := walkTree(tree, path); ok {
			return v, true
		}
	}
	return nil, false
}

// LookupString returns the string value at the given dotted path.
// Lookup order: the requested language, the default locale, English (each
// checking the loaded bundle first, then its defaults), then static bundles.
// The bool indicates whether a non-empty string was found; a missing key
// is logged as a warning.
func (s *Store) LookupString(code string, path ...string) (string, bool) {
	_ = s.ensureDefaults()
	def := s.DefaultLocale()
	path = splitPath(path)

	s.mu.RLock()
	langs := []string{code, def, "en"}
	seen := map[string]bool{}
	found := ""
chain:
	for _, lang := range langs {
		if lang == "" || seen[lang] || isStaticBundle(lang) {
			continue
		}
		seen[lang] = true
		for _, tree := range []map[string]any{s.bundles[lang], s.defaults[lang]} {
			if v, ok := walkTree(tree, path); ok {
				if str, ok := v.(string); ok && str != "" {
					found = str
					break chain
				}
			}
		}
	}
	if found == "" {
		for _, name := range s.staticBundleNames() {
			if v, ok := walkTree(s.bundles[name], path); ok {
				if str, ok := v.(string); ok && str != "" {
					found = str
					break
				}
			}
		}
	}
	s.mu.RUnlock()
	if found != "" {
		return found, true
	}
	s.warnMissingOnce(path)
	return "", false
}

// LeafPaths returns all dotted leaf paths (paths that resolve to strings)
// for the given language code: the union of the loaded bundle and its
// built-in defaults.
func (s *Store) LeafPaths(code string) []string {
	_ = s.ensureDefaults()
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, hasFile := s.bundles[code]
	d, hasDef := s.defaults[code]
	if !hasFile && !hasDef {
		return nil
	}
	var out []string
	collectKeys(mergeTree(d, b), nil, func(p []string, _ any) {
		out = append(out, joinPath(p))
	})
	return out
}

// T returns the localized string for the given path segments.
// Lookup order: current language, the default locale, English, static
// bundles, and finally the dotted path itself.
func (s *Store) T(path ...string) string {
	return s.tlang("", path...)
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
