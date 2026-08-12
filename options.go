package localengine

import (
	"io/fs"

	"github.com/he11ah0und/logger"
)

// Option customizes a Store at construction.
type Option func(*Store)

// WithDefaults sets built-in default translations as raw YAML documents
// keyed by locale code. Defaults sit under the loaded bundles: they fill in
// keys missing from the on-disk/embedded locale files, and file values win.
// Applications shipping dictionaries in the binary use this so an upgrade
// can add new keys without touching user-edited files.
func WithDefaults(defaults map[string]string) Option {
	return func(s *Store) {
		s.defaultsSource = defaults
	}
}

// WithDefaultsFS sets built-in default translations read from the *.yaml
// files at the root of fsys (e.g. an embed.FS narrowed with fs.Sub). Same
// effect as WithDefaults, but the documents stay in the filesystem.
func WithDefaultsFS(fsys fs.FS) Option {
	return func(s *Store) {
		s.defaultsFS = fsys
	}
}

// WithDefaultLocaleFunc sets a callback resolving the default locale (e.g.
// from the application's settings storage). It is called lazily and the
// result is cached until InvalidateDefault or the next LoadFromDir. An empty
// return value means "en". The store deliberately knows nothing about where
// the default comes from — no database dependency.
func WithDefaultLocaleFunc(fn func() string) Option {
	return func(s *Store) {
		s.defaultLocaleFn = fn
	}
}

// WithLogger sets the logger terminal at construction (see SetLogger).
func WithLogger(l *logger.LogTerminal) Option {
	return func(s *Store) {
		s.log = l
	}
}
