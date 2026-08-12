package localengine

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestValidatePlaceholderParity(t *testing.T) {
	s := New()
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("a:\n  x: Hello {{name}}\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("a:\n  x: Привет\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	var found []Warning
	for _, w := range s.Validate() {
		if w.Kind == WarnPlaceholders {
			found = append(found, w)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 placeholder warning, got %v", found)
	}
	w := found[0]
	if w.Key != "a.x" {
		t.Errorf("expected key a.x, got %q", w.Key)
	}
	if !strings.Contains(w.Message, "en={name}") || !strings.Contains(w.Message, "ru={}") {
		t.Errorf("unexpected message: %q", w.Message)
	}
}

func TestValidateStructuredKinds(t *testing.T) {
	s := New()
	fsys := fstest.MapFS{
		"en.yaml":         &fstest.MapFile{Data: []byte("a: same\nb: only-en\n")},
		"ru.yaml":         &fstest.MapFile{Data: []byte("a: same\nc: only-ru\n")},
		"app.static.yaml": &fstest.MapFile{Data: []byte("b: static-value\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	kinds := map[WarnKind]int{}
	for _, w := range s.Validate() {
		kinds[w.Kind]++
		if w.Message == "" {
			t.Errorf("warning without message: %+v", w)
		}
	}
	for _, want := range []WarnKind{WarnCollision, WarnMissing, WarnIdentical} {
		if kinds[want] == 0 {
			t.Errorf("expected at least one %q warning, got kinds %v", want, kinds)
		}
	}
}

func TestValidateNoFindings(t *testing.T) {
	s := New()
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("a: x\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("a: y\n")},
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}
	if ws := s.Validate(); len(ws) != 0 {
		t.Errorf("expected no findings, got %v", ws)
	}
}
