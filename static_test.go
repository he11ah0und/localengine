package localengine

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticBundle(t *testing.T) {
	fsys := fstest.MapFS{
		"app.static.yaml": &fstest.MapFile{Data: []byte("app:\n  title: Example App\nterms:\n  daemon: daemon\n")},
		"en.yaml":         &fstest.MapFile{Data: []byte("tab:\n  main: Main\n")},
		"ru.yaml":         &fstest.MapFile{Data: []byte("tab:\n  main: Главная\napp:\n  title: Example App\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	// Static-only keys resolve through the static fallback.
	if got := T("terms", "daemon"); got != "daemon" {
		t.Errorf("expected static 'daemon', got %q", got)
	}
	if got := T("app", "title"); got != "Example App" {
		t.Errorf("expected 'Example App', got %q", got)
	}

	// Static bundles are not selectable languages.
	if langs := AvailableLanguages(); slices.Contains(langs, "app.static") {
		t.Errorf("static bundle must not be a language, got %v", langs)
	}
	SetLanguage("app.static")
	if got := CurrentLanguage(); got == "app.static" {
		t.Errorf("static bundle must not be selectable, got %q", got)
	}

	// Validation: no identical-translation warning for static keys, but the
	// copy in ru.yaml collides with the static bundle.
	var identical, collisions []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "identical translation") {
			identical = append(identical, line)
		}
		if strings.Contains(line, "defined in multiple bundles") {
			collisions = append(collisions, line)
		}
	}
	if len(identical) != 0 {
		t.Errorf("static keys must not trigger identical warnings, got %v", identical)
	}
	if len(collisions) != 1 || !strings.Contains(collisions[0], `"app.title"`) ||
		!strings.Contains(collisions[0], "app.static") || !strings.Contains(collisions[0], "ru") {
		t.Errorf("expected collision warning for app.title, got %v", collisions)
	}
}

func TestStaticCollisionTranslated(t *testing.T) {
	fsys := fstest.MapFS{
		"app.static.yaml": &fstest.MapFile{Data: []byte("app:\n  title: Example App\n")},
		"en.yaml":         &fstest.MapFile{Data: []byte("tab:\n  main: Main\n")},
		"ru.yaml":         &fstest.MapFile{Data: []byte("app:\n  title: Пример\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	// A translated static key is a collision — warned regardless of who
	// overrides whom.
	var collisions []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "defined in multiple bundles") {
			collisions = append(collisions, line)
		}
	}
	if len(collisions) != 1 || !strings.Contains(collisions[0], `"app.title"`) {
		t.Errorf("expected collision warning for app.title, got %v", collisions)
	}

	// Lookup stays deterministic: dynamic languages win over static bundles.
	SetLanguage("ru")
	if got := T("app", "title"); got != "Пример" {
		t.Errorf("dynamic translation must win at lookup time, got %q", got)
	}
}

func TestStaticDuplicateDetection(t *testing.T) {
	fsys := fstest.MapFS{
		"app.static.yaml": &fstest.MapFile{Data: []byte("app:\n  title: Example App\nnotify:\n  started:\n    title: Example App\n")},
		"en.yaml":         &fstest.MapFile{Data: []byte("tab:\n  main: Main\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var dups []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "duplicated in keys") {
			dups = append(dups, line)
		}
	}
	if len(dups) != 1 ||
		!strings.Contains(dups[0], `"Example App"`) ||
		!strings.Contains(dups[0], "app.title, notify.started.title") ||
		!strings.Contains(dups[0], "in: app.static") {
		t.Errorf("expected duplicate warning scoped to app.static, got %v", dups)
	}
}
