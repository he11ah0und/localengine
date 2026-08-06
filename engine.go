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
)

// Logger is the minimal logging surface used by the engine.
// It is satisfied by any type with formatted Debug/Info/Warn methods.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

var (
	bundles     = make(map[string]map[string]any)
	currentLang = "en"
	log         Logger
)

// SetLogger sets the logger used by the engine. It is optional: when unset,
// all log output is discarded. It should be called once during application
// initialization.
func SetLogger(l Logger) {
	log = l
}

func debugf(format string, args ...any) {
	if log != nil {
		log.Debugf(format, args...)
	}
}

func infof(format string, args ...any) {
	if log != nil {
		log.Infof(format, args...)
	}
}

func warnf(format string, args ...any) {
	if log != nil {
		log.Warnf(format, args...)
	}
}

// LoadFromDir reads all *.yaml files from the root of fsys and parses them
// as locales. fsys may be an embed.FS (optionally narrowed with fs.Sub),
// an os.DirFS, or any other fs.FS implementation.
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
		lang := strings.TrimSuffix(e.Name(), path.Ext(e.Name()))
		debugf("loading locale %s", lang)
		if err := loadLanguage(lang, data); err != nil {
			return fmt.Errorf("load locale %s: %w", e.Name(), err)
		}
		loaded++
	}
	infof("loaded %d locale(s)", loaded)
	validateLocales()
	return nil
}

// LoadFromOSDir reads all *.yaml files from dir on the local filesystem.
func LoadFromOSDir(dir string) error {
	return LoadFromDir(os.DirFS(dir))
}

func loadLanguage(lang string, data []byte) error {
	raw, err := loadTree(data)
	if err != nil {
		return err
	}
	bundles[lang] = raw
	return nil
}

// lookup walks the nested map for the requested path.
func lookup(tree map[string]any, path []string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}
	current, ok := tree[path[0]]
	if !ok {
		return "", false
	}
	for _, p := range path[1:] {
		sub, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = sub[p]
		if !ok {
			return "", false
		}
	}
	s, ok := current.(string)
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
	current, ok := b[path[0]]
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

// LookupString returns the string value at the given dotted path.
// Falls back to English. The bool indicates whether a non-empty string was found.
func LookupString(code string, path ...string) (string, bool) {
	for _, lang := range []string{code, "en"} {
		if v, ok := Lookup(lang, path...); ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
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
	collectKeys(b, nil, func(path []string) {
		out = append(out, joinPath(path))
	})
	return out
}

// T returns the localized string for the given path segments.
// Falls back to English and finally to the dotted path itself.
func T(path ...string) string {
	if msg, ok := lookup(bundles[currentLang], path); ok && msg != "" {
		return msg
	}
	if msg, ok := lookup(bundles["en"], path); ok && msg != "" {
		return msg
	}
	warnf("missing value for path %v", path)
	return joinPath(path)
}

// SetLanguage sets the active language. Falls back to English for unknown codes.
func SetLanguage(code string) {
	if _, ok := bundles[code]; ok {
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

// AvailableLanguages returns the list of loaded language codes.
func AvailableLanguages() []string {
	keys := make([]string, 0, len(bundles))
	for k := range bundles {
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

// validateLocales compares all loaded locale bundles and warns about missing
// keys. It is called automatically after LoadFromDir finishes.
func validateLocales() {
	if len(bundles) == 0 {
		return
	}

	paths := make(map[string]map[string]struct{})
	for lang, tree := range bundles {
		collectKeys(tree, nil, func(path []string) {
			dotted := joinPath(path)
			if paths[dotted] == nil {
				paths[dotted] = make(map[string]struct{})
			}
			paths[dotted][lang] = struct{}{}
		})
	}

	langs := make([]string, 0, len(bundles))
	for lang := range bundles {
		langs = append(langs, lang)
	}
	slices.Sort(langs)

	dotted := make([]string, 0, len(paths))
	for k := range paths {
		dotted = append(dotted, k)
	}
	slices.Sort(dotted)

	for _, key := range dotted {
		present := paths[key]
		if len(present) == len(bundles) {
			continue
		}
		missing := make([]string, 0, len(bundles))
		for _, lang := range langs {
			if _, ok := present[lang]; !ok {
				missing = append(missing, lang)
			}
		}
		warnf("locale key %q missing in: %s", key, strings.Join(missing, ", "))
	}
}

// collectKeys walks a locale tree and calls yield for every leaf path.
func collectKeys(tree map[string]any, path []string, yield func([]string)) {
	for k, v := range tree {
		p := keyPath(path, k)
		if sub, ok := v.(map[string]any); ok {
			collectKeys(sub, p, yield)
		} else {
			yield(p)
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
