package localengine

import (
	"slices"
	"strings"
	"unicode"
)

// validateLocales compares all loaded UI bundles and warns about:
//   - keys missing in some languages;
//   - identical translations shared by several languages (a sign of an
//     untranslated key);
//   - duplicate values shared by several keys within one bundle (a sign of
//     copy-pasted entries that may deserve a shared key) — every bundle,
//     dynamic or static, is analyzed as its own scope;
//   - collisions: a key defined in more than one bundle when at least one
//     of them is static. A key across dynamic languages is a normal
//     translation; a static key must live in exactly one bundle.
//
// Each check lives in its own warn* function below. validateLocales is
// called automatically after LoadFromDir finishes.
func validateLocales() {
	validateBundleSet(bundles)
}

// validateLogBundles runs the same checks within the logs domain. The logs
// domain is a separate key space: no cross-domain duplicates or collisions
// are reported.
func validateLogBundles() {
	validateBundleSet(logBundles)
}

// validateBundleSet runs all locale checks on one bundle set. Static bundles
// are recognized by name; the logs domain has none.
func validateBundleSet(bundleSet map[string]map[string]any) {
	if len(bundleSet) == 0 {
		return
	}

	// leaves maps a bundle name to its dotted leaf paths and values
	// ("" for non-string or empty leaves; presence is tracked regardless).
	leaves := make(map[string]map[string]string)
	for name, tree := range bundleSet {
		vals := make(map[string]string)
		collectKeys(tree, nil, func(path []string, v any) {
			s, _ := v.(string)
			vals[joinPath(path)] = s
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

	warnStaticCollisions(leaves, langs, statics)
	warnMissingKeys(paths, langs)
	warnIdenticalTranslations(paths, staticKeySet(leaves, statics))
	warnDuplicateValues(leaves, langs, statics)
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

// warnStaticCollisions reports keys defined in more than one bundle when at
// least one of them is static. A static key must live in exactly one bundle.
func warnStaticCollisions(leaves map[string]map[string]string, langs, statics []string) {
	keyBundles := make(map[string][]string)
	for _, name := range append(slices.Clone(langs), statics...) {
		for key := range leaves[name] {
			keyBundles[key] = append(keyBundles[key], name)
		}
	}
	for _, key := range sortedKeys(keyBundles) {
		names := keyBundles[key]
		if len(names) > 1 && slices.ContainsFunc(names, isStaticBundle) {
			warnf("key %q defined in multiple bundles: %s", key, strings.Join(names, ", "))
		}
	}
}

// warnMissingKeys reports dynamic-language keys that are absent in some
// languages.
func warnMissingKeys(paths map[string]map[string]string, langs []string) {
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
		warnf("locale key %q missing in: %s", key, strings.Join(missing, ", "))
	}
}

// warnIdenticalTranslations reports dynamic-language keys that share the same
// non-empty translation across several languages — a sign of an untranslated
// key. Keys declared in static bundles are exempt.
func warnIdenticalTranslations(paths map[string]map[string]string, staticKeys map[string]struct{}) {
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
			warnf("locale key %q has identical translation in: %s", key, strings.Join(group, ", "))
		}
	}
}

// warnDuplicateValues reports duplicate values shared by several keys within
// one bundle — a sign of copy-pasted entries that may deserve a shared key
// (e.g. startup.mode_* vs settings.startup.mode_*). Every bundle, dynamic or
// static, is its own scope; the same duplicate usually exists in every
// language, so identical key sets are reported once with the bundle list.
func warnDuplicateValues(leaves map[string]map[string]string, langs, statics []string) {
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
	for _, dk := range dupKeys {
		namesWithDup := dups[dk]
		slices.Sort(namesWithDup)
		warnf("locale value %q duplicated in keys [%s] in: %s",
			truncateRunes(dk.value, 40), dk.keys, strings.Join(namesWithDup, ", "))
	}
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
