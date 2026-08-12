package localengine

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// WarnKind identifies a locale validation check.
type WarnKind string

const (
	// WarnCollision: a key is defined in more than one bundle and at least
	// one of them is static — a static key must live in exactly one bundle.
	WarnCollision WarnKind = "collision"
	// WarnMissing: a dynamic-language key is absent in some languages.
	WarnMissing WarnKind = "missing"
	// WarnIdentical: a key shares the same non-empty translation across
	// several languages — a sign of an untranslated key.
	WarnIdentical WarnKind = "identical"
	// WarnDuplicate: one value is shared by several keys within a bundle — a
	// sign of copy-pasted entries that may deserve a shared key.
	WarnDuplicate WarnKind = "duplicate"
	// WarnPlaceholders: the {{name}} placeholder sets of a key differ across
	// languages — a sign of a broken or untranslated template.
	WarnPlaceholders WarnKind = "placeholders"
)

// Warning is one structured locale validation finding.
type Warning struct {
	Kind WarnKind `json:"kind"`
	Key  string   `json:"key"`
	// Message is the full human-readable finding, in the same format the
	// engine logs after loading.
	Message string `json:"message"`
}

// Validate runs all locale checks on the default Store's UI bundles.
func Validate() []Warning {
	return defaultStore.Validate()
}

// ValidateLogs runs all locale checks on the default Store's logs domain.
func ValidateLogs() []Warning {
	return defaultStore.ValidateLogs()
}

// Validate runs all locale checks on the loaded UI bundles and returns
// structured findings. LoadFromDir logs the same findings automatically.
func (s *Store) Validate() []Warning {
	s.mu.RLock()
	set := s.bundles
	s.mu.RUnlock()
	return s.validateBundleSet(set)
}

// ValidateLogs runs the same checks within the logs domain. The logs domain
// is a separate key space: no cross-domain duplicates or collisions are
// reported.
func (s *Store) ValidateLogs() []Warning {
	s.mu.RLock()
	set := s.logBundles
	s.mu.RUnlock()
	return s.validateBundleSet(set)
}

// validateLocales logs the UI-bundle findings; called automatically after
// LoadFromDir finishes.
func (s *Store) validateLocales() {
	for _, w := range s.Validate() {
		s.warnf("%s", w.Message)
	}
}

// validateLogBundles logs the logs-domain findings; called automatically
// after LoadLogsFromDir finishes.
func (s *Store) validateLogBundles() {
	for _, w := range s.ValidateLogs() {
		s.warnf("%s", w.Message)
	}
}

// validateBundleSet runs all locale checks on one bundle set. Static bundles
// are recognized by name; the logs domain has none.
func (s *Store) validateBundleSet(bundleSet map[string]map[string]any) []Warning {
	if len(bundleSet) == 0 {
		return nil
	}

	// leaves maps a bundle name to its dotted leaf paths and values
	// ("" for non-string or empty leaves; presence is tracked regardless).
	leaves := make(map[string]map[string]string)
	for name, tree := range bundleSet {
		vals := make(map[string]string)
		collectKeys(tree, nil, func(path []string, v any) {
			str, _ := v.(string)
			vals[joinPath(path)] = str
		})
		leaves[name] = vals
	}

	var langs, statics []string
	for name := range bundleSet {
		if isStaticBundle(name) {
			statics = append(statics, name)
		} else {
			langs = append(langs, name)
		}
	}
	slices.Sort(langs)
	slices.Sort(statics)
	paths := languagePaths(leaves, langs)

	var out []Warning
	out = append(out, checkStaticCollisions(leaves, langs, statics)...)
	out = append(out, checkMissingKeys(paths, langs)...)
	out = append(out, checkIdenticalTranslations(paths, staticKeySet(leaves, statics))...)
	out = append(out, checkDuplicateValues(leaves, langs, statics)...)
	out = append(out, checkPlaceholderParity(paths)...)
	return out
}

// languagePaths maps each dotted key found in dynamic languages to its
// per-language values.
func languagePaths(leaves map[string]map[string]string, langs []string) map[string]map[string]string {
	paths := make(map[string]map[string]string)
	for _, lang := range langs {
		for key, v := range leaves[lang] {
			if paths[key] == nil {
				paths[key] = make(map[string]string)
			}
			paths[key][lang] = v
		}
	}
	return paths
}

// staticKeySet collects all keys declared in static bundles — they are
// exempt from the identical-translation check.
func staticKeySet(leaves map[string]map[string]string, statics []string) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, name := range statics {
		for key := range leaves[name] {
			keys[key] = struct{}{}
		}
	}
	return keys
}

// sortedKeys returns the sorted keys of a string-keyed map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// checkStaticCollisions reports keys defined in more than one bundle when at
// least one of them is static. A static key must live in exactly one bundle.
func checkStaticCollisions(leaves map[string]map[string]string, langs, statics []string) []Warning {
	keyBundles := make(map[string][]string)
	for _, name := range append(slices.Clone(langs), statics...) {
		for key := range leaves[name] {
			keyBundles[key] = append(keyBundles[key], name)
		}
	}
	var out []Warning
	for _, key := range sortedKeys(keyBundles) {
		names := keyBundles[key]
		if len(names) > 1 && slices.ContainsFunc(names, isStaticBundle) {
			out = append(out, Warning{
				Kind:    WarnCollision,
				Key:     key,
				Message: fmt.Sprintf("key %q defined in multiple bundles: %s", key, strings.Join(names, ", ")),
			})
		}
	}
	return out
}

// checkMissingKeys reports dynamic-language keys that are absent in some
// languages.
func checkMissingKeys(paths map[string]map[string]string, langs []string) []Warning {
	var out []Warning
	for _, key := range sortedKeys(paths) {
		present := paths[key]
		if len(present) == len(langs) {
			continue
		}
		missing := make([]string, 0, len(langs))
		for _, lang := range langs {
			if _, ok := present[lang]; !ok {
				missing = append(missing, lang)
			}
		}
		out = append(out, Warning{
			Kind:    WarnMissing,
			Key:     key,
			Message: fmt.Sprintf("locale key %q missing in: %s", key, strings.Join(missing, ", ")),
		})
	}
	return out
}

// checkIdenticalTranslations reports dynamic-language keys that share the
// same non-empty translation across several languages — a sign of an
// untranslated key. Keys declared in static bundles are exempt.
func checkIdenticalTranslations(paths map[string]map[string]string, staticKeys map[string]struct{}) []Warning {
	var out []Warning
	for _, key := range sortedKeys(paths) {
		if _, ok := staticKeys[key]; ok {
			continue
		}
		// Group languages sharing the same non-empty translation.
		byValue := make(map[string][]string)
		for lang, v := range paths[key] {
			if v == "" {
				continue
			}
			byValue[v] = append(byValue[v], lang)
		}
		groups := make([][]string, 0, len(byValue))
		for _, group := range byValue {
			if len(group) > 1 {
				slices.Sort(group)
				groups = append(groups, group)
			}
		}
		slices.SortFunc(groups, func(a, b []string) int {
			return strings.Compare(a[0], b[0])
		})
		for _, group := range groups {
			out = append(out, Warning{
				Kind:    WarnIdentical,
				Key:     key,
				Message: fmt.Sprintf("locale key %q has identical translation in: %s", key, strings.Join(group, ", ")),
			})
		}
	}
	return out
}

// checkDuplicateValues reports duplicate values shared by several keys within
// one bundle — a sign of copy-pasted entries that may deserve a shared key
// (e.g. startup.mode_* vs settings.startup.mode_*). Every bundle, dynamic or
// static, is its own scope; the same duplicate usually exists in every
// language, so identical key sets are reported once with the bundle list.
func checkDuplicateValues(leaves map[string]map[string]string, langs, statics []string) []Warning {
	type dupKey struct {
		value string
		keys  string
	}
	dups := make(map[dupKey][]string)
	for _, name := range append(slices.Clone(langs), statics...) {
		byValue := make(map[string][]string)
		for key, v := range leaves[name] {
			if !hasLetters(v) {
				continue
			}
			byValue[v] = append(byValue[v], key)
		}
		for v, keys := range byValue {
			if len(keys) < 2 {
				continue
			}
			slices.Sort(keys)
			dk := dupKey{value: v, keys: strings.Join(keys, ", ")}
			dups[dk] = append(dups[dk], name)
		}
	}
	dupKeys := make([]dupKey, 0, len(dups))
	for dk := range dups {
		dupKeys = append(dupKeys, dk)
	}
	slices.SortFunc(dupKeys, func(a, b dupKey) int {
		return strings.Compare(a.keys, b.keys)
	})
	var out []Warning
	for _, dk := range dupKeys {
		namesWithDup := dups[dk]
		slices.Sort(namesWithDup)
		out = append(out, Warning{
			Kind: WarnDuplicate,
			Key:  dk.keys,
			Message: fmt.Sprintf("locale value %q duplicated in keys [%s] in: %s",
				truncateRunes(dk.value, 40), dk.keys, strings.Join(namesWithDup, ", ")),
		})
	}
	return out
}

// checkPlaceholderParity reports keys whose {{name}} placeholder sets differ
// across languages — a sign of a broken or untranslated template.
func checkPlaceholderParity(paths map[string]map[string]string) []Warning {
	var out []Warning
	for _, key := range sortedKeys(paths) {
		sets := make(map[string][]string)
		sigs := make(map[string]struct{})
		for lang, v := range paths[key] {
			ph := placeholders(v)
			slices.Sort(ph)
			sets[lang] = ph
			sigs[strings.Join(ph, "\x00")] = struct{}{}
		}
		if len(sigs) < 2 {
			continue
		}
		parts := make([]string, 0, len(sets))
		for _, lang := range sortedKeys(sets) {
			parts = append(parts, fmt.Sprintf("%s={%s}", lang, strings.Join(sets[lang], ", ")))
		}
		out = append(out, Warning{
			Kind:    WarnPlaceholders,
			Key:     key,
			Message: fmt.Sprintf("locale key %q placeholder mismatch: %s", key, strings.Join(parts, ", ")),
		})
	}
	return out
}

// hasLetters reports whether s contains at least two letter runes — shorter
// or purely symbolic values ("—", "✓", "%s") are not worth duplicate
// warnings.
func hasLetters(s string) bool {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
			if n >= 2 {
				return true
			}
		}
	}
	return false
}

// truncateRunes shortens s to at most n runes, appending an ellipsis when
// truncated.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
