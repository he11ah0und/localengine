package localengine

import (
	"testing"
	"testing/fstest"
)

func platformTestFS() fstest.MapFS {
	return fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte(`greeting: Hello
platform:
  thing: base value
`)},
		"en.linux.yaml": &fstest.MapFile{Data: []byte(`platform:
  thing: linux value
  only_linux: present
`)},
		"en.windows.yaml": &fstest.MapFile{Data: []byte(`platform:
  thing: windows value
  only_windows: present
`)},
	}
}

func TestPlatformOverlayMergedOnMatchingGOOS(t *testing.T) {
	s := New()
	if err := s.LoadFromDirPlatform(platformTestFS(), "linux"); err != nil {
		t.Fatalf("load: %v", err)
	}
	if v, ok := s.LookupString("en", "greeting"); !ok || v != "Hello" {
		t.Errorf("base key: got %q, %v", v, ok)
	}
	if v, ok := s.LookupString("en", "platform", "thing"); !ok || v != "linux value" {
		t.Errorf("overlay override: got %q, %v", v, ok)
	}
	if v, ok := s.LookupString("en", "platform", "only_linux"); !ok || v != "present" {
		t.Errorf("overlay-only key: got %q, %v", v, ok)
	}
	if _, ok := s.LookupString("en", "platform", "only_windows"); ok {
		t.Error("foreign platform key must not load")
	}
}

func TestPlatformOverlayOtherGOOS(t *testing.T) {
	s := New()
	if err := s.LoadFromDirPlatform(platformTestFS(), "windows"); err != nil {
		t.Fatalf("load: %v", err)
	}
	if v, ok := s.LookupString("en", "platform", "thing"); !ok || v != "windows value" {
		t.Errorf("overlay override: got %q, %v", v, ok)
	}
	if _, ok := s.LookupString("en", "platform", "only_linux"); ok {
		t.Error("foreign platform key must not load")
	}
}

func TestPlatformOverlayWithoutBase(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml":       &fstest.MapFile{Data: []byte("greeting: Hello\n")},
		"ru.linux.yaml": &fstest.MapFile{Data: []byte("greeting: Привет\n")},
	}
	s := New()
	if err := s.LoadFromDirPlatform(fsys, "linux"); err != nil {
		t.Fatalf("load: %v", err)
	}
	if v, ok := s.LookupString("ru", "greeting"); !ok || v != "Привет" {
		t.Errorf("overlay without base forms its own bundle: got %q, %v", v, ok)
	}
}

func TestLogsPlatformOverlay(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte(`meta: {type: logs, binding: internal}
scope:
  base: "base %v"
  thing: "base"
`)},
		"en.linux.yaml": &fstest.MapFile{Data: []byte(`meta: {type: logs, binding: internal}
scope:
  thing: "linux"
`)},
		"en.windows.yaml": &fstest.MapFile{Data: []byte(`meta: {type: logs, binding: internal}
scope:
  thing: "windows"
`)},
	}
	s := New()
	if err := s.LoadLogsFromDirPlatform(fsys, "linux"); err != nil {
		t.Fatalf("load: %v", err)
	}
	resolve := s.LogResolver()
	if v, ok := resolve("scope.thing"); !ok || v != "linux" {
		t.Errorf("logs overlay override: got %q, %v", v, ok)
	}
	if v, ok := resolve("scope.base"); !ok || v != "base %v" {
		t.Errorf("logs base key: got %q, %v", v, ok)
	}
}
