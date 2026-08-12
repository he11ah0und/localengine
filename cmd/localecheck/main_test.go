package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLocales(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCleanDirExitsZero(t *testing.T) {
	dir := writeLocales(t, map[string]string{
		"en.yaml": "a: x\n",
		"ru.yaml": "a: y\n",
	})
	var out, errOut bytes.Buffer
	if code := run([]string{dir}, &out, &errOut); code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %s)", code, errOut.String())
	}
}

func TestWarningsTextAndStrict(t *testing.T) {
	dir := writeLocales(t, map[string]string{
		"en.yaml": "a: x\n",
		"ru.yaml": "b: y\n",
	})

	var out bytes.Buffer
	if code := run([]string{dir}, &out, &bytes.Buffer{}); code != 0 {
		t.Errorf("warnings without -strict: expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "missing") {
		t.Errorf("expected missing-key warning in output, got %q", out.String())
	}

	if code := run([]string{"-strict", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("warnings with -strict: expected exit 1, got %d", code)
	}
}

func TestJSONFormat(t *testing.T) {
	dir := writeLocales(t, map[string]string{
		"en.yaml": "a: x\n",
		"ru.yaml": "b: y\n",
	})
	var out bytes.Buffer
	if code := run([]string{"-format", "json", dir}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), `"kind": "missing"`) {
		t.Errorf("expected JSON warning, got %q", out.String())
	}
}

func TestBrokenFileExitsOne(t *testing.T) {
	dir := writeLocales(t, map[string]string{
		"en.yaml": ":\ninvalid",
	})
	if code := run([]string{dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("expected exit 1 on parse error, got %d", code)
	}
}

func TestSingleFile(t *testing.T) {
	dir := writeLocales(t, map[string]string{
		"en.yaml": "a: x\n",
	})
	var out bytes.Buffer
	if code := run([]string{filepath.Join(dir, "en.yaml")}, &out, &bytes.Buffer{}); code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestLogsDomainRequiresEn(t *testing.T) {
	dir := writeLocales(t, map[string]string{
		"ru.yaml": "meta:\n  type: logs\n  binding: internal\nk: v\n",
	})
	if code := run([]string{"-domain", "logs", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("expected exit 1 without en logs file, got %d", code)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"-domain", "bogus", "x"},
		{"-format", "bogus", "x"},
	} {
		if code := run(args, &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
			t.Errorf("args %v: expected exit 2, got %d", args, code)
		}
	}
}
