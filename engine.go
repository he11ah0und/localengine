// Package localengine provides a tiny in-memory localization engine.
// It reads nested YAML locale files and supports lookup by path segments,
// e.g. localengine.T("about", "btn", "open_repo").
//
// The package keeps its state globally (loaded bundles and the current
// language) and is not safe for concurrent mutation: load locales and set
// the language during initialization, then only call T from other goroutines.
package localengine

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/he11ah0und/logger"
	"github.com/he11ah0und/yamltree"
)

var (
	bundles     = make(map[string]map[string]any)
	currentLang = "en"
	log         *logger.LogTerminal

	// staticNames caches the sorted names of loaded static bundles so that
	// lookups do not rescan and resort the bundle map on every call. It is
	// rebuilt by LoadFromDir after all files are parsed.
	staticNames []string

	// missingWarned deduplicates "missing value" warnings: a path that
	// resolves nowhere is reported once per process, not on every lookup.
	missingWarned   = make(map[string]struct{})
	missingWarnedMu sync.Mutex
)

// staticSuffix marks static bundles: any file named *.static.yaml holds names
// that are never translated (brand names, technical terms). Static bundles
// load like regular locale files and take part in validation, but they are
// not selectable UI languages. There may be several static bundles in one
// project (e.g. app.static.yaml, terms.static.yaml).
const staticSuffix = ".static"

// isStaticBundle reports whether the bundle name refers to a static bundle.
func isStaticBundle(name string) bool {
	return strings.HasSuffix(name, staticSuffix)
}

// staticBundleNames returns the sorted names of all loaded static bundles.
// The list is cached; see staticNames.
func staticBundleNames() []string {
	return staticNames
}

// rebuildStaticNames refreshes the staticNames cache from bundles.
func rebuildStaticNames() {
	staticNames = staticNames[:0]
	for name := range bundles {
		if isStaticBundle(name) {
			staticNames = append(staticNames, name)
		}
	}
	slices.Sort(staticNames)
}

// SetLogger sets the logger terminal used by the engine. It is optional:
// logging through a nil terminal is a no-op. It should be called once during
// application initialization.
func SetLogger(l *logger.LogTerminal) {
	log = l
}

func debugf(format string, args ...any) {
	log.Debugf(append([]any{format}, args...)...)
}

func infof(format string, args ...any) {
	log.Infof(append([]any{format}, args...)...)
}

func warnf(format string, args ...any) {
	log.Warnf(append([]any{format}, args...)...)
}

// warnMissingOnce logs a missing-value warning for the path, once per
// process. The backend reports unresolvable keys at request time; callers
// must not add their own "key not found" warnings on top.
func warnMissingOnce(path []string) {
	missingWarnedMu.Lock()
	defer missingWarnedMu.Unlock()
	key := joinPath(path)
	if _, ok := missingWarned[key]; ok {
		return
	}
	missingWarned[key] = struct{}{}
	warnf("missing value for path %v", path)
}

// LoadFromDir reads all *.yaml files from the root of fsys and parses them
// as locales. fsys may be an embed.FS (optionally narrowed with fs.Sub),
// an os.DirFS, or any other fs.FS implementation.
//
// Files named *.static.yaml load as static bundles: names that are never
// translated. They are not selectable languages and take part in validation
// like any other bundle.
func LoadFromDir(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read locale dir: %w", err)
	}
	loaded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return fmt.Errorf("read locale %s: %w", e.Name(), err)
		}
		name := strings.TrimSuffix(e.Name(), path.Ext(e.Name()))
		if isStaticBundle(name) {
			debugf("loading static bundle %s", name)
		} else {
			debugf("loading locale %s", name)
			loaded++
		}
		if err := loadLanguage(name, data); err != nil {
			return fmt.Errorf("load locale %s: %w", e.Name(), err)
		}
	}
	infof("loaded %d locale(s)", loaded)
	rebuildStaticNames()
	validateLocales()
	return nil
}

// LoadFromOSDir reads all *.yaml files from dir on the local filesystem.
func LoadFromOSDir(dir string) error {
	return LoadFromDir(os.DirFS(dir))
}

func loadLanguage(lang string, data []byte) error {
	raw, err := yamltree.LoadTree(data)
	if err != nil {
		return err
	}
	bundles[lang] = raw
	return nil
}

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

// SetLanguage sets the active language. Falls back to English for unknown codes.
// Static bundles are not selectable languages.
func SetLanguage(code string) {
	if _, ok := bundles[code]; ok && !isStaticBundle(code) {
		currentLang = code
		infof("language set to %s", code)
	} else {
		currentLang = "en"
		warnf("language %s not available, falling back to en", code)
	}
}

// CurrentLanguage returns the active language code.
func CurrentLanguage() string {
	return currentLang
}

// Translations returns the full translation tree for the given language code.
// Returns nil if the language is not loaded.
func Translations(code string) map[string]any {
	if b, ok := bundles[code]; ok {
		return copyMap(b)
	}
	return nil
}

func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[k] = copyMap(val)
		default:
			dst[k] = val
		}
	}
	return dst
}

// AvailableLanguages returns the list of loaded language codes,
// excluding static bundles.
func AvailableLanguages() []string {
	keys := make([]string, 0, len(bundles))
	for k := range bundles {
		if isStaticBundle(k) {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// DetectSystemLanguage tries to detect the system language from environment
// variables. Falls back to "en" if detection fails or language is unsupported.
func DetectSystemLanguage() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = os.Getenv("LC_ALL")
	}
	if lang == "" {
		debugf("no LANG/LC_ALL set, defaulting to en")
		return "en"
	}
	if idx := strings.Index(lang, "."); idx != -1 {
		lang = lang[:idx]
	}
	base, _, _ := strings.Cut(lang, "_")
	if slices.Contains(AvailableLanguages(), base) {
		debugf("detected system language: %s", base)
		return base
	}
	debugf("system language %s not available, defaulting to en", base)
	return "en"
}

// LanguageName returns the native name of the language for the given code
// (reads locale.name from that language's messages).
func LanguageName(code string) string {
	if b, ok := bundles[code]; ok {
		if name, ok := lookup(b, []string{"locale", "name"}); ok && name != "" {
			return name
		}
	}
	return code
}

// validateLocales compares all loaded bundles and warns about:
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
	if len(bundles) == 0 {
		return
	}

	// leaves maps a bundle name to its dotted leaf paths and values
	// ("" for non-string or empty leaves; presence is tracked regardless).
	leaves := make(map[string]map[string]string)
	for name, tree := range bundles {
		vals := make(map[string]string)
		collectKeys(tree, nil, func(path []string, v any) {
			s, _ := v.(string)
			vals[joinPath(path)] = s
		})
		leaves[name] = vals
	}

	langs := AvailableLanguages()
	statics := staticBundleNames()
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
