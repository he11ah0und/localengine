package localengine

import (
	"os"
	"slices"
	"strings"
)

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
