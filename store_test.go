package localengine

import (
	"sync"
	"testing"
	"testing/fstest"
)

func loadTestStore(t *testing.T, files map[string]string) *Store {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	s := New()
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}
	return s
}

func TestStoreInstancesAreIsolated(t *testing.T) {
	a := loadTestStore(t, map[string]string{
		"en.yaml": "greeting: Hello A\n",
	})
	b := loadTestStore(t, map[string]string{
		"en.yaml": "greeting: Hello B\n",
	})

	if got := a.T("greeting"); got != "Hello A" {
		t.Errorf("store A: expected 'Hello A', got %q", got)
	}
	if got := b.T("greeting"); got != "Hello B" {
		t.Errorf("store B: expected 'Hello B', got %q", got)
	}

	b.SetLanguage("ru")
	if got := a.CurrentLanguage(); got != "en" {
		t.Errorf("store A language changed by store B: got %q", got)
	}
}

func TestStoreConcurrentT(t *testing.T) {
	s := loadTestStore(t, map[string]string{
		"en.yaml": "a:\n  b: value\n",
		"ru.yaml": "a:\n  b: значение\n",
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				s.SetLanguage("en")
			} else {
				s.SetLanguage("ru")
			}
			for j := 0; j < 50; j++ {
				got := s.T("a", "b")
				if got != "value" && got != "значение" {
					t.Errorf("unexpected T result %q", got)
				}
				_ = s.Tf(Vars{"x": 1}, "a", "b")
				_ = s.Record("a")
			}
		}(i)
	}
	wg.Wait()
}

func TestFacadeMatchesDefaultStore(t *testing.T) {
	resetBundles()
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("key: facade\n")},
	}
	if err := LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}
	if got := T("key"); got != "facade" {
		t.Errorf("facade T: expected 'facade', got %q", got)
	}
	if got := defaultStore.T("key"); got != "facade" {
		t.Errorf("default store T: expected 'facade', got %q", got)
	}
}
