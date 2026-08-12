// Package localengine provides a tiny in-memory localization engine.
// It reads nested YAML locale files and supports lookup by path segments,
// e.g. localengine.T("about", "btn", "open_repo").
//
// The package-level functions are a facade over a shared default Store,
// convenient for single-engine applications: load locales and set the
// language during initialization, then call T from any goroutine.
// Applications that need several independent engines (e.g. a web server)
// create their own instances with New; Store is safe for concurrent use.
package localengine

import "github.com/he11ah0und/logger"

// defaultStore backs the package-level functions.
var defaultStore = New()

// SetLogger sets the logger terminal used by the default Store. It is
// optional: logging through a nil terminal is a no-op. It should be called
// once during application initialization.
func SetLogger(l *logger.LogTerminal) {
	defaultStore.SetLogger(l)
}
