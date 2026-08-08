// Package localengine provides a tiny in-memory localization engine.
// It reads nested YAML locale files and supports lookup by path segments,
// e.g. localengine.T("about", "btn", "open_repo").
//
// The package keeps its state globally (loaded bundles and the current
// language) and is not safe for concurrent mutation: load locales and set
// the language during initialization, then only call T from other goroutines.
package localengine

import (
	"slices"
	"strings"
	"sync"

	"github.com/he11ah0und/logger"
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
