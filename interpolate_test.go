package localengine

import (
	"testing"
	"testing/fstest"
)

func TestTfInterpolates(t *testing.T) {
	resetBundles()
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("app:\n  greeting: Hello, {{name}}!\n")},
	}
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}
	if got := Tf(Vars{"name": "world"}, "app", "greeting"); got != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", got)
	}
}

func TestInterpolateUnknownPlaceholderStays(t *testing.T) {
	got := interpolate("Hello, {{name}} from {{place}}", Vars{"name": "x"})
	if got != "Hello, x from {{place}}" {
		t.Errorf("expected unknown placeholder left as-is, got %q", got)
	}
}

func TestInterpolateNonStringValues(t *testing.T) {
	got := interpolate("v={{version}} n={{count}}", Vars{"version": "1.2.3", "count": 42})
	if got != "v=1.2.3 n=42" {
		t.Errorf("expected fmt.Sprint rendering, got %q", got)
	}
}

func TestPlaceholders(t *testing.T) {
	got := placeholders("{{a}} and {{b}} and {{a}} broken {{ ")
	if len(got) != 2 {
		t.Fatalf("expected 2 unique placeholders, got %v", got)
	}
	// order of first appearance
	if got[0] != "a" || got[1] != "b" {
		t.Errorf("expected [a b], got %v", got)
	}
}

func TestTfOnNamespaceAndLocalizer(t *testing.T) {
	s := loadTestStore(t, map[string]string{
		"en.yaml": "app:\n  greeting: Hello, {{name}}!\n",
		"ru.yaml": "app:\n  greeting: Привет, {{name}}!\n",
	})

	ns := s.NS("app")
	if got := ns.Tf(Vars{"name": "мир"}, "greeting"); got != "Hello, мир!" {
		t.Errorf("namespace Tf: got %q", got)
	}

	loc := s.NewLocalizer("ru")
	if got := loc.Tf(Vars{"name": "мир"}, "app", "greeting"); got != "Привет, мир!" {
		t.Errorf("localizer Tf: got %q", got)
	}
}
