package localengine

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/he11ah0und/logger"
)

func resetBundles() {
	bundles = make(map[string]map[string]any)
	currentLang = "en"
}

func TestLoadFromDir(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("key: value\n")},
	}

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestLoadFromSubFS(t *testing.T) {
	fsys := fstest.MapFS{
		"locales/en.yaml": &fstest.MapFile{Data: []byte("key: value\n")},
	}
	sub, err := fs.Sub(fsys, "locales")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}

	resetBundles()
	if err := LoadFromDir(sub); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestSetLanguage(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("tab:\n  settings: Settings\nlocale:\n  name: English\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("tab:\n  settings: Настройки\nlocale:\n  name: Русский\n")},
		"zh.yaml": &fstest.MapFile{Data: []byte("tab:\n  settings: 设置\nlocale:\n  name: 中文\n")},
	}

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	SetLanguage("ru")
	if got := T("tab", "settings"); got != "Настройки" {
		t.Errorf("expected Russian 'Настройки', got %q", got)
	}
	SetLanguage("zh")
	if got := T("tab", "settings"); got != "设置" {
		t.Errorf("expected Chinese '设置', got %q", got)
	}
}

func TestNestedKeys(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("about:\n  btn:\n    open_repo: Open repo\n    open_data: Open data\n")},
	}

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := T("about", "btn", "open_repo"); got != "Open repo" {
		t.Errorf("expected 'Open repo', got %q", got)
	}
	if got := T("about", "btn", "open_data"); got != "Open data" {
		t.Errorf("expected 'Open data', got %q", got)
	}
}

func TestFallbackToEnglish(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("only:\n  english: Hello\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("other: Другое\n")},
	}

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	SetLanguage("ru")
	if got := T("only", "english"); got != "Hello" {
		t.Errorf("expected English fallback 'Hello', got %q", got)
	}
}

func TestMissingKeyReturnsPath(t *testing.T) {
	resetBundles()
	bundles["en"] = map[string]any{"existing": map[string]any{"key": "value"}}

	if got := T("missing", "key"); got != "missing.key" {
		t.Errorf("expected dotted path fallback, got %q", got)
	}
}

func TestLanguageName(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("locale:\n  name: English\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("locale:\n  name: Русский\n")},
	}

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := LanguageName("ru"); got != "Русский" {
		t.Errorf("expected 'Русский', got %q", got)
	}
	if got := LanguageName("unknown"); got != "unknown" {
		t.Errorf("expected fallback code, got %q", got)
	}
}

func TestDetectSystemLanguage(t *testing.T) {
	resetBundles()
	bundles["en"] = map[string]any{}
	bundles["ru"] = map[string]any{}
	bundles["zh"] = map[string]any{}

	orig := os.Getenv("LANG")
	defer os.Setenv("LANG", orig)

	os.Setenv("LANG", "ru_RU.UTF-8")
	if got := DetectSystemLanguage(); got != "ru" {
		t.Errorf("expected 'ru', got %q", got)
	}

	os.Setenv("LANG", "unknown_LANG")
	if got := DetectSystemLanguage(); got != "en" {
		t.Errorf("expected fallback 'en', got %q", got)
	}
}

func TestLoadFromOSDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.yaml"), []byte("key: value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	resetBundles()
	if err := LoadFromOSDir(dir); err != nil {
		t.Fatalf("load from dir: %v", err)
	}

	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestValidateIdenticalTranslations(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("same: Same text\npart: Shared\nuniq: English\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("same: Same text\npart: Shared\nuniq: Русский\n")},
		"zh.yaml": &fstest.MapFile{Data: []byte("same: 相同文本\npart: Shared\nuniq: 中文\n")},
	}

	l := logger.NewLogger(100)
	SetLogger(l.Root)
	defer SetLogger(nil)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var warns []string
	for _, line := range l.Root.GetLines() {
		if strings.Contains(line, "identical translation") {
			warns = append(warns, line)
		}
	}
	if len(warns) != 2 {
		t.Fatalf("expected 2 identical-translation warnings, got %v", warns)
	}
	joined := strings.Join(warns, "\n")
	if !strings.Contains(joined, `"same"`) || !strings.Contains(joined, "en, ru") {
		t.Errorf("expected 'same' key flagged for en+ru, got %v", warns)
	}
	if !strings.Contains(joined, `"part"`) || !strings.Contains(joined, "en, ru, zh") {
		t.Errorf("expected 'part' key flagged for en+ru+zh, got %v", warns)
	}
	if strings.Contains(joined, `"uniq"`) {
		t.Errorf("'uniq' key must not be flagged, got %v", warns)
	}
}

func TestValidateDuplicateValues(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("startup:\n  mode_tcp: tcp\nsettings:\n  startup:\n    mode_tcp: tcp\nother: Unique text\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("startup:\n  mode_tcp: tcp\nsettings:\n  startup:\n    mode_tcp: tcp\nother: Другой текст\n")},
	}

	l := logger.NewLogger(100)
	SetLogger(l.Root)
	defer SetLogger(nil)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var dups []string
	for _, line := range l.Root.GetLines() {
		if strings.Contains(line, "duplicated in keys") {
			dups = append(dups, line)
		}
	}
	if len(dups) != 1 {
		t.Fatalf("expected 1 duplicate warning, got %v", dups)
	}
	if !strings.Contains(dups[0], `"tcp"`) ||
		!strings.Contains(dups[0], "settings.startup.mode_tcp, startup.mode_tcp") ||
		!strings.Contains(dups[0], "in: en, ru") {
		t.Errorf("unexpected duplicate warning: %q", dups[0])
	}
}

func TestStaticBundle(t *testing.T) {
	fsys := fstest.MapFS{
		"app.static.yaml": &fstest.MapFile{Data: []byte("app:\n  title: Example App\nterms:\n  daemon: daemon\n")},
		"en.yaml":         &fstest.MapFile{Data: []byte("tab:\n  main: Main\n")},
		"ru.yaml":         &fstest.MapFile{Data: []byte("tab:\n  main: Главная\napp:\n  title: Example App\n")},
	}

	l := logger.NewLogger(100)
	SetLogger(l.Root)
	defer SetLogger(nil)

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
	for _, line := range l.Root.GetLines() {
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

	l := logger.NewLogger(100)
	SetLogger(l.Root)
	defer SetLogger(nil)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	// A translated static key is a collision — warned regardless of who
	// overrides whom.
	var collisions []string
	for _, line := range l.Root.GetLines() {
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

	l := logger.NewLogger(100)
	SetLogger(l.Root)
	defer SetLogger(nil)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var dups []string
	for _, line := range l.Root.GetLines() {
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
