package localengine

import (
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
)

// LoadLogsFromDir reads all *.yaml files from the root of fsys and parses
// them as logs-domain locales using the default Store.
func LoadLogsFromDir(fsys fs.FS) error {
	return defaultStore.LoadLogsFromDir(fsys)
}

// LoadLogsFromOSDir reads all *.yaml logs locale files from dir on the
// local filesystem using the default Store.
func LoadLogsFromOSDir(dir string) error {
	return defaultStore.LoadLogsFromOSDir(dir)
}

// LoadLogsFromDirPlatform is LoadLogsFromDir for an explicit platform
// instead of runtime.GOOS, using the default Store.
func LoadLogsFromDirPlatform(fsys fs.FS, goos string) error {
	return defaultStore.LoadLogsFromDirPlatform(fsys, goos)
}

// LogResolver returns a resolver over the default Store's logs-domain
// bundles, suitable for logger.LogTerminal.SetResolver.
func LogResolver() func(key string) (format string, ok bool) {
	return defaultStore.LogResolver()
}

// LoadLogsFromDir reads all *.yaml files from the root of fsys and parses
// them as logs-domain locales. Every file must declare
// "meta: {type: logs, binding: internal}" — the header is what separates
// log messages from UI strings. A logs file set must contain an en file:
// English is the fallback for every lookup and the source of the keys the
// code references.
//
// Lookup semantics mirror the UI domain (active language, English fallback,
// warn-once on missing keys); validation (parity, identical translations,
// duplicate values) runs within the logs domain only.
//
// Logs bundles are a separate key space from the UI bundles: a key in a ui
// file and the same key in a logs file are unrelated.
//
// Platform overlays work like in the UI domain: <lang>.<goos>.yaml files
// merge over the base <lang> logs bundle on a matching platform and are
// skipped otherwise.
func (s *Store) LoadLogsFromDir(fsys fs.FS) error {
	return s.LoadLogsFromDirPlatform(fsys, runtime.GOOS)
}

// LoadLogsFromDirPlatform is LoadLogsFromDir for an explicit platform
// instead of runtime.GOOS.
func (s *Store) LoadLogsFromDirPlatform(fsys fs.FS, goos string) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read logs locale dir: %w", err)
	}
	parsed := make([]parsedLocaleFile, 0, len(entries))
	var overlays []parsedLocaleFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		base, platform, isOverlay := splitPlatformOverlay(e.Name())
		if isOverlay && platform != goos {
			s.debugf("skipping platform overlay %s (for %s)", e.Name(), platform)
			continue
		}
		name, tree, meta, err := parseLocaleFile(fsys, e.Name())
		if err != nil {
			return err
		}
		if meta.Type != MetaTypeLogs {
			return fmt.Errorf("load logs locale %s: meta type must be %q", e.Name(), MetaTypeLogs)
		}
		if isOverlay {
			s.debugf("loading logs platform overlay %s", e.Name())
			overlays = append(overlays, parsedLocaleFile{base, tree, meta})
			continue
		}
		s.debugf("loading logs locale %s (%s, %s)", name, meta.Type, meta.Binding)
		parsed = append(parsed, parsedLocaleFile{name, tree, meta})
	}
	parsed = mergePlatformOverlays(parsed, overlays)

	s.mu.Lock()
	for _, p := range parsed {
		s.logBundles[p.name] = p.tree
	}
	_, hasEn := s.logBundles["en"]
	s.mu.Unlock()

	if !hasEn {
		return fmt.Errorf("logs locales require an en file")
	}
	s.infof("loaded %d logs locale(s)", len(parsed))
	s.validateLogBundles()
	return nil
}

// LoadLogsFromOSDir reads all *.yaml logs locale files from dir on the local
// filesystem.
func (s *Store) LoadLogsFromOSDir(dir string) error {
	return s.LoadLogsFromDir(os.DirFS(dir))
}

// LogResolver returns a resolver over the logs-domain bundles, suitable for
// logger.LogTerminal.SetResolver. The key is a dotted path (e.g.
// "core.start.failed"); the result is the raw format string for the active
// language — formatting with args stays on the caller. The language follows
// the same SetLanguage machinery as the UI. A missing key is reported once
// per process and resolves to not-ok; the logger then prints the key itself.
func (s *Store) LogResolver() func(key string) (format string, ok bool) {
	return func(key string) (string, bool) {
		s.mu.RLock()
		lang := s.currentLang
		s.mu.RUnlock()
		return s.lookupLogs(lang, strings.Split(key, "."))
	}
}

// lookupLogs resolves path in the logs domain: the requested language, then
// English. Static bundles do not take part — log messages are internal.
func (s *Store) lookupLogs(code string, path []string) (string, bool) {
	langs := []string{code}
	if code != "en" {
		langs = append(langs, "en")
	}
	s.mu.RLock()
	found := ""
	for _, lang := range langs {
		if b, ok := s.logBundles[lang]; ok {
			if str, ok := lookup(b, path); ok && str != "" {
				found = str
				break
			}
		}
	}
	s.mu.RUnlock()
	if found != "" {
		return found, true
	}
	// The "logs" prefix keeps the warn-once key space separate from UI paths.
	s.warnMissingOnce(append([]string{"logs"}, path...))
	return "", false
}
