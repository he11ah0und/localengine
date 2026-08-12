package localengine

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/he11ah0und/yamltree"
)

// ensureDefaults parses the configured defaults source once, acquiring the
// write lock only when parsing is still pending.
func (s *Store) ensureDefaults() error {
	s.mu.RLock()
	parsed, err := s.defaultsParsed, s.defaultsErr
	s.mu.RUnlock()
	if parsed {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureDefaultsLocked()
}

// ensureDefaultsLocked parses the configured defaults source once. Callers
// must hold the write lock. A parse error is sticky: it is returned on every
// call, and lookups simply ignore the defaults layer.
func (s *Store) ensureDefaultsLocked() error {
	if s.defaultsParsed {
		return s.defaultsErr
	}
	s.defaultsParsed = true

	raws := s.defaultsSource
	if raws == nil && s.defaultsFS != nil {
		entries, err := fs.ReadDir(s.defaultsFS, ".")
		if err != nil {
			s.defaultsErr = fmt.Errorf("read defaults dir: %w", err)
			return s.defaultsErr
		}
		raws = make(map[string]string, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			data, err := fs.ReadFile(s.defaultsFS, e.Name())
			if err != nil {
				s.defaultsErr = fmt.Errorf("read defaults %s: %w", e.Name(), err)
				return s.defaultsErr
			}
			raws[strings.TrimSuffix(e.Name(), path.Ext(e.Name()))] = string(data)
		}
	}

	for locale, raw := range raws {
		tree, err := yamltree.LoadTree([]byte(raw))
		if err != nil {
			s.defaultsErr = fmt.Errorf("parse defaults %s: %w", locale, err)
			return s.defaultsErr
		}
		if _, err := parseMeta(tree); err != nil {
			s.defaultsErr = fmt.Errorf("parse defaults %s: %w", locale, err)
			return s.defaultsErr
		}
		s.defaults[locale] = tree
	}
	return nil
}

// deepMerge copies src into dst recursively; src values win unless both
// values are maps, in which case they are merged. Called with defaults first
// and the loaded file second, so file values override defaults.
func deepMerge(dst, src map[string]any) {
	for key, value := range src {
		if existing, ok := dst[key]; ok {
			existingMap, existingIsMap := existing.(map[string]any)
			valueMap, valueIsMap := value.(map[string]any)
			if existingIsMap && valueIsMap {
				deepMerge(existingMap, valueMap)
				continue
			}
		}
		dst[key] = value
	}
}

// mergeTree returns a fresh tree: overlay deep-merged over base.
func mergeTree(base, overlay map[string]any) map[string]any {
	out := copyMap(base)
	deepMerge(out, overlay)
	return out
}

// DefaultLocale returns the default locale of the default Store.
func DefaultLocale() string {
	return defaultStore.DefaultLocale()
}

// SetDefaultLocale overrides the cached default of the default Store.
func SetDefaultLocale(locale string) {
	defaultStore.SetDefaultLocale(locale)
}

// InvalidateDefault clears the default Store's cached default locale so the
// next call re-resolves it from the configured source.
func InvalidateDefault() {
	defaultStore.InvalidateDefault()
}

// DefaultLocale returns the configured default locale ("en" when unset),
// caching the result until InvalidateDefault or the next LoadFromDir.
func (s *Store) DefaultLocale() string {
	s.mu.RLock()
	cached, fn := s.defaultLocaleCache, s.defaultLocaleFn
	s.mu.RUnlock()
	if cached != "" {
		return cached
	}

	def := "en"
	if fn != nil {
		if v := fn(); v != "" {
			def = v
		}
	}

	s.mu.Lock()
	s.defaultLocaleCache = def
	s.mu.Unlock()
	return def
}

// SetDefaultLocale overrides the cached default (useful for tests and for
// applications that track the setting themselves).
func (s *Store) SetDefaultLocale(locale string) {
	s.mu.Lock()
	s.defaultLocaleCache = locale
	s.mu.Unlock()
}

// InvalidateDefault clears the cached default locale so the next call
// re-resolves it from the source set via WithDefaultLocaleFunc.
func (s *Store) InvalidateDefault() {
	s.mu.Lock()
	s.defaultLocaleCache = ""
	s.mu.Unlock()
}
