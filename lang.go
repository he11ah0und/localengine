package localengine

import (
	"os"
	"slices"
	"strings"
)

// SetLanguage sets the active language of the default Store.
func SetLanguage(code string) {
	defaultStore.SetLanguage(code)
}

// CurrentLanguage returns the active language code of the default Store.
func CurrentLanguage() string {
	return defaultStore.CurrentLanguage()
}

// Translations returns the full translation tree for the given language code
// from the default Store.
func Translations(code string) map[string]any {
	return defaultStore.Translations(code)
}

// AvailableLanguages returns the list of loaded language codes from the
// default Store, excluding static bundles.
func AvailableLanguages() []string {
	return defaultStore.AvailableLanguages()
}

// DetectSystemLanguage tries to detect the system language from environment
// variables using the default Store's languages.
func DetectSystemLanguage() string {
	return defaultStore.DetectSystemLanguage()
}

// LanguageName returns the native name of the language for the given code
// from the default Store.
func LanguageName(code string) string {
	return defaultStore.LanguageName(code)
}

// SetLanguage sets the active language. Falls back to English for unknown codes.
// Static bundles are not selectable languages.
func (s *Store) SetLanguage(code string) {
	s.mu.Lock()
	_, ok := s.bundles[code]
	ok = ok && !isStaticBundle(code)
	if ok {
		s.currentLang = code
	} else {
		s.currentLang = "en"
	}
	s.mu.Unlock()

	if ok {
		s.infof("language set to %s", code)
	} else {
		s.warnf("language %s not available, falling back to en", code)
	}
}

// CurrentLanguage returns the active language code.
func (s *Store) CurrentLanguage() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentLang
}

// Translations returns the full translation tree for the given language
// code: the loaded bundle deep-merged over the built-in defaults (file
// values win). Returns nil if the language is unknown.
func (s *Store) Translations(code string) map[string]any {
	_ = s.ensureDefaults()
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, hasFile := s.bundles[code]
	d, hasDef := s.defaults[code]
	if !hasFile && !hasDef {
		return nil
	}
	return mergeTree(d, b)
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

// AvailableLanguages returns the list of known language codes — loaded
// bundles plus built-in defaults — excluding static bundles.
func (s *Store) AvailableLanguages() []string {
	_ = s.ensureDefaults()
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool, len(s.bundles))
	keys := make([]string, 0, len(s.bundles))
	for k := range s.bundles {
		if isStaticBundle(k) {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
	}
	for k := range s.defaults {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// DetectSystemLanguage tries to detect the system language from environment
// variables. Falls back to "en" if detection fails or language is unsupported.
func (s *Store) DetectSystemLanguage() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = os.Getenv("LC_ALL")
	}
	if lang == "" {
		s.debugf("no LANG/LC_ALL set, defaulting to en")
		return "en"
	}
	if idx := strings.Index(lang, "."); idx != -1 {
		lang = lang[:idx]
	}
	base, _, _ := strings.Cut(lang, "_")
	if slices.Contains(s.AvailableLanguages(), base) {
		s.debugf("detected system language: %s", base)
		return base
	}
	s.debugf("system language %s not available, defaulting to en", base)
	return "en"
}

// LanguageName returns the native name of the language for the given code
// (reads locale.name from that language's messages: the loaded bundle first,
// then the built-in defaults).
func (s *Store) LanguageName(code string) string {
	_ = s.ensureDefaults()
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, tree := range []map[string]any{s.bundles[code], s.defaults[code]} {
		if name, ok := lookup(tree, []string{"locale", "name"}); ok && name != "" {
			return name
		}
	}
	return code
}
