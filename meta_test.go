package localengine

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestMetaAbsentDefaultsToUIExternal(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("key: value\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var loading []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "loading locale") {
			loading = append(loading, line)
		}
	}
	if len(loading) != 1 || !strings.Contains(loading[0], "en (ui, external)") {
		t.Errorf("expected 'loading locale en (ui, external)', got %v", loading)
	}
	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestMetaHeaderParsedAndStripped(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("meta:\n  type: ui\n  binding: external\nkey: value\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var loading []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "loading locale") {
			loading = append(loading, line)
		}
	}
	if len(loading) != 1 || !strings.Contains(loading[0], "en (ui, external)") {
		t.Errorf("expected 'loading locale en (ui, external)', got %v", loading)
	}

	// The meta header must not enter the key space.
	if _, ok := Lookup("en", "meta"); ok {
		t.Error("meta header must be stripped from the bundle")
	}
	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestMetaUnknownTypeIsLoadError(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("meta:\n  type: bogus\nkey: value\n")},
	}

	resetBundles()
	err := LoadFromDir(fsys)
	if err == nil || !strings.Contains(err.Error(), "unknown meta type") {
		t.Errorf("expected unknown meta type error, got %v", err)
	}
}

func TestMetaUnknownBindingIsLoadError(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("meta:\n  type: ui\n  binding: sideways\nkey: value\n")},
	}

	resetBundles()
	err := LoadFromDir(fsys)
	if err == nil || !strings.Contains(err.Error(), "unknown meta binding") {
		t.Errorf("expected unknown meta binding error, got %v", err)
	}
}

func TestMetaLogsFileRejectedByLoadFromDir(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("meta:\n  type: logs\n  binding: internal\nkey: value\n")},
	}

	resetBundles()
	err := LoadFromDir(fsys)
	if err == nil || !strings.Contains(err.Error(), "LoadLogsFromDir") {
		t.Errorf("expected logs-file rejection, got %v", err)
	}
}
