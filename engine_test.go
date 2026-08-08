package localengine

import (
	"io/fs"
	"os"
	"path/filepath"
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
