package localengine

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/he11ah0und/logger"
)

func resetBundles() {
	bundles = make(map[string]map[string]any)
	logBundles = make(map[string]map[string]any)
	currentLang = "en"
	staticNames = nil
	missingWarned = make(map[string]struct{})
}

// withTestLogger routes engine output into an in-memory logger terminal and
// restores the nil logger on cleanup.
func withTestLogger(t *testing.T) *logger.LogTerminal {
	t.Helper()
	l := logger.NewLogger(100)
	SetLogger(l.Root)
	t.Cleanup(func() { SetLogger(nil) })
	return l.Root
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
