package localengine

import (
	"testing"
)

func TestNamespacePrefix(t *testing.T) {
	s := loadTestStore(t, map[string]string{
		"en.yaml": "main:\n  btn:\n    start: Start\n    stop: Stop\n",
	})

	btn := s.NS("main", "btn")
	if got := btn.T("start"); got != "Start" {
		t.Errorf("expected 'Start', got %q", got)
	}
	if got := btn.T("stop"); got != "Stop" {
		t.Errorf("expected 'Stop', got %q", got)
	}
	// Missing leaf falls back to the full dotted path.
	if got := btn.T("pause"); got != "main.btn.pause" {
		t.Errorf("expected full path fallback, got %q", got)
	}
}

func TestDottedAndSegmentPathsAreEqual(t *testing.T) {
	s := loadTestStore(t, map[string]string{
		"en.yaml": "a:\n  b:\n    c: value\n",
	})
	if got := s.T("a.b.c"); got != "value" {
		t.Errorf("dotted T: expected 'value', got %q", got)
	}
	if got := s.T("a", "b", "c"); got != "value" {
		t.Errorf("segment T: expected 'value', got %q", got)
	}
	if got := s.T("a.b", "c"); got != "value" {
		t.Errorf("mixed T: expected 'value', got %q", got)
	}
	if got, ok := s.LookupString("en", "a.b.c"); !ok || got != "value" {
		t.Errorf("dotted LookupString: got %q, %v", got, ok)
	}
}

func TestRecordResolvesSubtree(t *testing.T) {
	s := loadTestStore(t, map[string]string{
		"en.yaml": "main:\n  phase:\n    starting: Starting\n    connected: Connected\n",
		"ru.yaml": "main:\n  phase:\n    starting: Запуск\n",
	})

	r := s.Record("main", "phase")
	if got := r.Get("starting"); got != "Starting" {
		t.Errorf("expected 'Starting', got %q", got)
	}
	// Fallback to English for a leaf missing in the active language.
	s.SetLanguage("ru")
	r = s.Record("main.phase")
	if got := r.Get("starting"); got != "Запуск" {
		t.Errorf("expected 'Запуск', got %q", got)
	}
	if got := r.Get("connected"); got != "Connected" {
		t.Errorf("expected English fallback 'Connected', got %q", got)
	}
	// Unknown leaf resolves to the full dotted key.
	if got := r.Get("exploded"); got != "main.phase.exploded" {
		t.Errorf("expected full key fallback, got %q", got)
	}
}

func TestRecordIncludesStaticBundles(t *testing.T) {
	s := loadTestStore(t, map[string]string{
		"en.yaml":         "app:\n  section:\n    title: Title\n",
		"app.static.yaml": "app:\n  section:\n    brand: Example App\n",
	})

	r := s.Record("app", "section")
	if got := r.Get("brand"); got != "Example App" {
		t.Errorf("expected static value, got %q", got)
	}
	if got := r.Get("title"); got != "Title" {
		t.Errorf("expected 'Title', got %q", got)
	}
}

func TestRecordLeavesCopy(t *testing.T) {
	s := loadTestStore(t, map[string]string{
		"en.yaml": "ns:\n  a: A\n",
	})
	r := s.Record("ns")
	leaves := r.Leaves()
	leaves["a"] = "mutated"
	if got := r.Get("a"); got != "A" {
		t.Errorf("mutation of Leaves() leaked into the record: %q", got)
	}
}
