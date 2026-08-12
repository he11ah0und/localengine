package localengine

import (
	"testing"
	"testing/fstest"
)

func TestDefaultsFillMissingKeys(t *testing.T) {
	s := New(WithDefaults(map[string]string{
		"en": "a:\n  from_default: Default value\n  overridden: Default version\n",
	}))
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("a:\n  overridden: File version\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := s.T("a", "from_default"); got != "Default value" {
		t.Errorf("expected default to fill the key, got %q", got)
	}
	if got := s.T("a", "overridden"); got != "File version" {
		t.Errorf("expected file to win over default, got %q", got)
	}
}

func TestDefaultsChainWithDefaultLocale(t *testing.T) {
	s := New(
		WithDefaults(map[string]string{
			"en": "k: en-default\n",
			"ru": "k: ru-default\n",
		}),
		WithDefaultLocaleFunc(func() string { return "ru" }),
	)
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("other: x\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	// The key exists only in defaults; requesting an unknown language falls
	// back to the default locale (ru) before English.
	if got, _ := s.LookupString("zh", "k"); got != "ru-default" {
		t.Errorf("expected ru default via defaultLocale, got %q", got)
	}
}

func TestWithDefaultsFS(t *testing.T) {
	defs := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("k: from-fs-default\n")},
	}
	s := New(WithDefaultsFS(defs))
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("other: x\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}
	if got := s.T("k"); got != "from-fs-default" {
		t.Errorf("expected fs default, got %q", got)
	}
}

func TestDefaultsParseError(t *testing.T) {
	s := New(WithDefaults(map[string]string{"en": ":\ninvalid"}))
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("k: v\n")},
	}
	if err := s.LoadFromDir(fsys); err == nil {
		t.Fatal("expected defaults parse error")
	}
}

func TestDefaultLocaleCachingAndInvalidation(t *testing.T) {
	calls := 0
	current := "ru"
	s := New(WithDefaultLocaleFunc(func() string {
		calls++
		return current
	}))

	if got := s.DefaultLocale(); got != "ru" {
		t.Errorf("expected 'ru', got %q", got)
	}
	if got := s.DefaultLocale(); got != "ru" || calls != 1 {
		t.Errorf("expected cached 'ru' with one call, got %q (%d calls)", got, calls)
	}

	current = "zh"
	s.InvalidateDefault()
	if got := s.DefaultLocale(); got != "zh" {
		t.Errorf("expected 'zh' after invalidation, got %q", got)
	}

	s.SetDefaultLocale("fr")
	if got := s.DefaultLocale(); got != "fr" {
		t.Errorf("expected override 'fr', got %q", got)
	}
}

func TestDefaultLocaleEmptyMeansEn(t *testing.T) {
	s := New(WithDefaultLocaleFunc(func() string { return "" }))
	if got := s.DefaultLocale(); got != "en" {
		t.Errorf("expected 'en' for empty callback, got %q", got)
	}
}

func TestTranslationsMergeDefaults(t *testing.T) {
	s := New(WithDefaults(map[string]string{
		"en": "a:\n  x: def-x\n  y: def-y\n",
	}))
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("a:\n  x: file-x\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	tr := s.Translations("en")
	a, ok := tr["a"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %v", tr)
	}
	if a["x"] != "file-x" || a["y"] != "def-y" {
		t.Errorf("expected merged {x: file-x, y: def-y}, got %v", a)
	}
	if got := s.Translations("unknown"); got != nil {
		t.Errorf("expected nil for unknown locale, got %v", got)
	}
}

func TestAvailableLanguagesIncludesDefaults(t *testing.T) {
	s := New(WithDefaults(map[string]string{"zh": "k: v\n"}))
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("k: v\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}
	langs := s.AvailableLanguages()
	if len(langs) != 2 || langs[0] != "en" || langs[1] != "zh" {
		t.Errorf("expected [en zh], got %v", langs)
	}
}
