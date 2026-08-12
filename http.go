package localengine

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type contextKey struct{}

// Localizer resolves translations for one request locale.
type Localizer struct {
	store  *Store
	locale string
}

// Locale returns the negotiated locale code.
func (l *Localizer) Locale() string { return l.locale }

// Get returns the translation for the path ("" and false when not found).
func (l *Localizer) Get(path ...string) (string, bool) {
	return l.store.LookupString(l.locale, path...)
}

// T returns the translation for the path, falling back to the dotted path
// itself when not found.
func (l *Localizer) T(path ...string) string {
	return l.store.tlang(l.locale, path...)
}

// Tf returns the translation with {{name}} placeholders interpolated.
func (l *Localizer) Tf(vars Vars, path ...string) string {
	return interpolate(l.T(path...), vars)
}

// NS returns a Namespace bound to the path prefix and the request locale.
func (l *Localizer) NS(path ...string) Namespace {
	return Namespace{store: l.store, lang: l.locale, prefix: splitPath(path)}
}

// Record resolves the subtree at the path into a RecordView for the request
// locale.
func (l *Localizer) Record(path ...string) RecordView {
	return l.store.recordLang(l.locale, splitPath(path))
}

// NewLocalizer builds a Localizer bound to a locale.
func (s *Store) NewLocalizer(locale string) *Localizer {
	return &Localizer{store: s, locale: locale}
}

// Middleware negotiates the request locale (?locale= query parameter first,
// then the Accept-Language header, then the store default) and stores a
// Localizer in the request context for FromContext.
func (s *Store) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := r.URL.Query().Get("locale")
		if locale == "" || !s.Has(locale) {
			locale = matchAcceptLanguage(r.Header.Get("Accept-Language"), s)
		}
		if locale == "" {
			locale = s.DefaultLocale()
		}
		ctx := context.WithValue(r.Context(), contextKey{}, &Localizer{store: s, locale: locale})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the Localizer stored by Middleware, or nil when the
// middleware was not applied.
func FromContext(ctx context.Context) *Localizer {
	l, _ := ctx.Value(contextKey{}).(*Localizer)
	return l
}

// Has reports whether the locale is known (loaded bundle or built-in
// defaults). Static bundles are not locales.
func (s *Store) Has(locale string) bool {
	if locale == "" || isStaticBundle(locale) {
		return false
	}
	_ = s.ensureDefaults()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.bundles[locale]; ok {
		return true
	}
	_, ok := s.defaults[locale]
	return ok
}

// LocaleInfo is one entry of the HandleLocales response.
type LocaleInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// HandleLocales lists the available locales and the configured default:
// {"defaultLocale": "...", "locales": [{"code": ..., "name": ...}]}.
func (s *Store) HandleLocales(w http.ResponseWriter, r *http.Request) {
	locales := make([]LocaleInfo, 0)
	for _, code := range s.AvailableLanguages() {
		locales = append(locales, LocaleInfo{Code: code, Name: s.LanguageName(code)})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"defaultLocale": s.DefaultLocale(),
		"locales":       locales,
	})
}

// HandleDictionary returns the full nested translation map for a locale,
// merged over the built-in defaults. The locale is taken from the "locale"
// query parameter; 404 when the locale is unknown.
func (s *Store) HandleDictionary(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	translations := s.Translations(locale)
	if translations == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "locale not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(translations)
}

// matchAcceptLanguage picks the best known locale from an Accept-Language
// header, honoring q-values. Matches exact codes first, then the primary
// subtag ("ru-RU" → "ru"). Returns "" when nothing matches.
func matchAcceptLanguage(header string, s *Store) string {
	type cand struct {
		code string
		q    float64
	}
	var cands []cand
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		q := 1.0
		code := part
		if i := strings.IndexByte(part, ';'); i >= 0 {
			code = strings.TrimSpace(part[:i])
			params := strings.Split(part[i+1:], ";")
			for _, p := range params {
				p = strings.TrimSpace(p)
				if v, ok := strings.CutPrefix(p, "q="); ok {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						q = f
					}
				}
			}
		}
		if code != "" && code != "*" && q > 0 {
			cands = append(cands, cand{code, q})
		}
	}
	// Stable sort by descending q (insertion — lists are short).
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && cands[j-1].q < cands[j].q; j-- {
			cands[j-1], cands[j] = cands[j], cands[j-1]
		}
	}
	for _, c := range cands {
		if s.Has(c.code) {
			return c.code
		}
		if i := strings.IndexByte(c.code, '-'); i > 0 {
			if primary := c.code[:i]; s.Has(primary) {
				return primary
			}
		}
	}
	return ""
}
