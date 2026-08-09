package localengine

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// logBundles holds the logs-domain locales: key → format string for log
// messages, resolved at runtime through LogResolver. Logs bundles are a
// separate key space from the UI bundles: a key in a ui file and the same
// key in a logs file are unrelated.
var logBundles = make(map[string]map[string]any)

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
func LoadLogsFromDir(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read logs locale dir: %w", err)
	}
	loaded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name, tree, meta, err := parseLocaleFile(fsys, e.Name())
		if err != nil {
			return err
		}
		if meta.Type != MetaTypeLogs {
			return fmt.Errorf("load logs locale %s: meta type must be %q", e.Name(), MetaTypeLogs)
		}
		debugf("loading logs locale %s (%s, %s)", name, meta.Type, meta.Binding)
		logBundles[name] = tree
		loaded++
	}
	if _, ok := logBundles["en"]; !ok {
		return fmt.Errorf("logs locales require an en file")
	}
	infof("loaded %d logs locale(s)", loaded)
	validateLogBundles()
	return nil
}

// LoadLogsFromOSDir reads all *.yaml logs locale files from dir on the local
// filesystem.
func LoadLogsFromOSDir(dir string) error {
	return LoadLogsFromDir(os.DirFS(dir))
}

// LogResolver returns a resolver over the logs-domain bundles, suitable for
// logger.LogTerminal.SetResolver. The key is a dotted path (e.g.
// "core.start.failed"); the result is the raw format string for the active
// language — formatting with args stays on the caller. The language follows
// the same SetLanguage machinery as the UI. A missing key is reported once
// per process and resolves to not-ok; the logger then prints the key itself.
func LogResolver() func(key string) (format string, ok bool) {
	return func(key string) (string, bool) {
		return lookupLogs(currentLang, strings.Split(key, "."))
	}
}

// lookupLogs resolves path in the logs domain: the requested language, then
// English. Static bundles do not take part — log messages are internal.
func lookupLogs(code string, path []string) (string, bool) {
	langs := []string{code}
	if code != "en" {
		langs = append(langs, "en")
	}
	for _, lang := range langs {
		if b, ok := logBundles[lang]; ok {
			if s, ok := lookup(b, path); ok && s != "" {
				return s, true
			}
		}
	}
	// The "logs" prefix keeps the warn-once key space separate from UI paths.
	warnMissingOnce(append([]string{"logs"}, path...))
	return "", false
}
