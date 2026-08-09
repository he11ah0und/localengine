package localengine

import (
	"strings"
	"testing"
	"testing/fstest"
)

const logsMeta = "meta:\n  type: logs\n  binding: internal\n"

func TestLoadLogsFromDir(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte(logsMeta + "core:\n  start:\n    failed: 'start failed: %v'\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte(logsMeta + "core:\n  start:\n    failed: 'ошибка запуска: %v'\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadLogsFromDir(fsys); err != nil {
		t.Fatalf("load logs locales: %v", err)
	}

	var loading []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "loading logs locale") {
			loading = append(loading, line)
		}
	}
	if len(loading) != 2 {
		t.Fatalf("expected 2 loading lines, got %v", loading)
	}
	if !strings.Contains(loading[0]+loading[1], "en (logs, internal)") {
		t.Errorf("expected 'en (logs, internal)' loading line, got %v", loading)
	}

	resolve := LogResolver()
	format, ok := resolve("core.start.failed")
	if !ok || format != "start failed: %v" {
		t.Errorf("expected en format, got %q (ok=%v)", format, ok)
	}
}

func TestLogsResolverLanguageFallback(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte(logsMeta + "a: A\nb: B\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte(logsMeta + "a: А\n")},
	}

	resetBundles()
	if err := LoadFromDir(fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("x: x\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("x: x\n")},
	}); err != nil {
		t.Fatalf("load ui locales: %v", err)
	}
	if err := LoadLogsFromDir(fsys); err != nil {
		t.Fatalf("load logs locales: %v", err)
	}

	SetLanguage("ru")
	resolve := LogResolver()

	// Hit in the active language.
	if format, ok := resolve("a"); !ok || format != "А" {
		t.Errorf("expected ru 'А', got %q (ok=%v)", format, ok)
	}
	// Missing in ru: English fallback.
	if format, ok := resolve("b"); !ok || format != "B" {
		t.Errorf("expected en fallback 'B', got %q (ok=%v)", format, ok)
	}
}

func TestLogsResolverMissingKeyWarnsOnce(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte(logsMeta + "a: A\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadLogsFromDir(fsys); err != nil {
		t.Fatalf("load logs locales: %v", err)
	}

	resolve := LogResolver()
	if _, ok := resolve("missing.key"); ok {
		t.Error("missing key must resolve to not-ok")
	}
	if _, ok := resolve("missing.key"); ok {
		t.Error("missing key must resolve to not-ok")
	}

	var warns []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "missing value for path") {
			warns = append(warns, line)
		}
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "logs") || !strings.Contains(warns[0], "missing") {
		t.Errorf("expected one warn-once missing warning, got %v", warns)
	}
}

func TestLoadLogsRequiresEnglish(t *testing.T) {
	fsys := fstest.MapFS{
		"ru.yaml": &fstest.MapFile{Data: []byte(logsMeta + "a: А\n")},
	}

	resetBundles()
	err := LoadLogsFromDir(fsys)
	if err == nil || !strings.Contains(err.Error(), "require an en file") {
		t.Errorf("expected en-required error, got %v", err)
	}
}

func TestLoadLogsRejectsNonLogsFiles(t *testing.T) {
	// No meta header at all.
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("a: A\n")},
	}
	resetBundles()
	if err := LoadLogsFromDir(fsys); err == nil || !strings.Contains(err.Error(), "meta type must be") {
		t.Errorf("expected meta-type error for headerless file, got %v", err)
	}

	// Explicit ui type.
	fsys = fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("meta:\n  type: ui\na: A\n")},
	}
	resetBundles()
	if err := LoadLogsFromDir(fsys); err == nil || !strings.Contains(err.Error(), "meta type must be") {
		t.Errorf("expected meta-type error for ui file, got %v", err)
	}
}

func TestLogsDomainIsSeparateKeySpace(t *testing.T) {
	// Same value in a ui file and a logs file must not be reported as a
	// duplicate across domains.
	uiFS := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("shared: Same text here\n")},
	}
	logsFS := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte(logsMeta + "shared: Same text here\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadFromDir(uiFS); err != nil {
		t.Fatalf("load ui locales: %v", err)
	}
	if err := LoadLogsFromDir(logsFS); err != nil {
		t.Fatalf("load logs locales: %v", err)
	}

	for _, line := range term.GetLines() {
		if strings.Contains(line, "duplicated in keys") || strings.Contains(line, "defined in multiple bundles") {
			t.Errorf("cross-domain duplicates must not be flagged, got %q", line)
		}
	}

	// Parity checks still run within the logs domain.
	logsFS2 := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte(logsMeta + "a: A\nb: B\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte(logsMeta + "a: А\n")},
	}
	term.Clear()
	resetBundles()
	if err := LoadLogsFromDir(logsFS2); err != nil {
		t.Fatalf("load logs locales: %v", err)
	}
	var missing []string
	for _, line := range term.GetLines() {
		if strings.Contains(line, "missing in:") {
			missing = append(missing, line)
		}
	}
	if len(missing) != 1 || !strings.Contains(missing[0], `"b"`) || !strings.Contains(missing[0], "ru") {
		t.Errorf("expected parity warning for key b in ru, got %v", missing)
	}
}
