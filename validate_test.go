package localengine

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestValidateIdenticalTranslations(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("same: Same text\npart: Shared\nuniq: English\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("same: Same text\npart: Shared\nuniq: Русский\n")},
		"zh.yaml": &fstest.MapFile{Data: []byte("same: 相同文本\npart: Shared\nuniq: 中文\n")},
	}

	term := withTestLogger(t)

	resetBundles()
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var warns []string
	for _, line := range term.GetLines() {
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
	if len(dups) != 1 {
		t.Fatalf("expected 1 duplicate warning, got %v", dups)
	}
	if !strings.Contains(dups[0], `"tcp"`) ||
		!strings.Contains(dups[0], "settings.startup.mode_tcp, startup.mode_tcp") ||
		!strings.Contains(dups[0], "in: en, ru") {
		t.Errorf("unexpected duplicate warning: %q", dups[0])
	}
}
