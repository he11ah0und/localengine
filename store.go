package localengine

import (
	"io/fs"
	"slices"
	"strings"
	"sync"

	"github.com/he11ah0und/logger"
)

// Store is a self-contained localization engine: loaded UI bundles, the logs
// domain, the active language, and the logger. Unlike the package-level
// facade (which shares one default Store), a Store created with New is
// independent and safe for concurrent use — servers handling per-request
// locales should keep their own instances.
type Store struct {
	mu          sync.RWMutex
	bundles     map[string]map[string]any
	logBundles  map[string]map[string]any
	currentLang string
	log         *logger.LogTerminal

	// staticNames caches the sorted names of loaded static bundles so that
	// lookups do not rescan and resort the bundle map on every call. It is
	// rebuilt by LoadFromDir after all files are parsed.
	staticNames []string

	// missingWarned deduplicates "missing value" warnings: a path that
	// resolves nowhere is reported once per process, not on every lookup.
	warnMu        sync.Mutex
	missingWarned map[string]struct{}

	// defaults holds the built-in translations (set via WithDefaults or
	// WithDefaultsFS); they fill in keys missing from the loaded bundles
	// and file values win. defaultsSource/defaultsFS are the raw inputs,
	// parsed lazily once (defaultsParsed/defaultsErr make the result sticky).
	defaults       map[string]map[string]any
	defaultsSource map[string]string
	defaultsFS     fs.FS
	defaultsParsed bool
	defaultsErr    error

	// defaultLocaleFn resolves the default locale (see
	// WithDefaultLocaleFunc); the result is cached in defaultLocaleCache
	// until InvalidateDefault or the next LoadFromDir.
	defaultLocaleFn    func() string
	defaultLocaleCache string
}

// New creates an empty Store customized by opts.
func New(opts ...Option) *Store {
	s := &Store{
		bundles:       make(map[string]map[string]any),
		logBundles:    make(map[string]map[string]any),
		currentLang:   "en",
		missingWarned: make(map[string]struct{}),
		defaults:      make(map[string]map[string]any),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SetLogger sets the logger terminal used by the engine. It is optional:
// logging through a nil terminal is a no-op. It should be called once during
// application initialization.
func (s *Store) SetLogger(l *logger.LogTerminal) {
	s.mu.Lock()
	s.log = l
	s.mu.Unlock()
}

func (s *Store) logger() *logger.LogTerminal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.log
}

func (s *Store) debugf(format string, args ...any) {
	s.logger().Debugf(append([]any{format}, args...)...)
}

func (s *Store) infof(format string, args ...any) {
	s.logger().Infof(append([]any{format}, args...)...)
}

func (s *Store) warnf(format string, args ...any) {
	s.logger().Warnf(append([]any{format}, args...)...)
}

// warnMissingOnce logs a missing-value warning for the path, once per
// process. The backend reports unresolvable keys at request time; callers
// must not add their own "key not found" warnings on top.
func (s *Store) warnMissingOnce(path []string) {
	s.warnMu.Lock()
	defer s.warnMu.Unlock()
	key := joinPath(path)
	if _, ok := s.missingWarned[key]; ok {
		return
	}
	s.missingWarned[key] = struct{}{}
	s.warnf("missing value for path %v", path)
}

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
// The list is cached; see staticNames. Callers must hold at least a read
// lock.
func (s *Store) staticBundleNames() []string {
	return s.staticNames
}

// rebuildStaticNames refreshes the staticNames cache from bundles. Callers
// must hold the write lock.
func (s *Store) rebuildStaticNames() {
	s.staticNames = s.staticNames[:0]
	for name := range s.bundles {
		if isStaticBundle(name) {
			s.staticNames = append(s.staticNames, name)
		}
	}
	slices.Sort(s.staticNames)
}
