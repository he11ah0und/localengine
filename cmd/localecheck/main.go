// Command localecheck loads locale files headless and reports the engine's
// validation findings (missing keys, collisions, identical translations,
// duplicate values, placeholder mismatches).
//
// Usage:
//
//	localecheck [flags] <path> [path...]
//
// Paths may be directories (loaded as locale dirs, with the usual meta and
// static-bundle rules) or individual .yaml files (each loaded as a one-file
// set). Errors (unreadable files, YAML or meta failures) always exit 1;
// warnings exit 1 only with -strict.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/he11ah0und/localengine"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("localecheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	domain := fs.String("domain", "ui", "locale domain to check: ui or logs")
	strict := fs.Bool("strict", false, "exit 1 when warnings are found")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *domain != "ui" && *domain != "logs" {
		fmt.Fprintf(stderr, "invalid -domain %q (want ui or logs)\n", *domain)
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "invalid -format %q (want text or json)\n", *format)
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "usage: localecheck [flags] <path> [path...]")
		return 2
	}

	var warnings []localengine.Warning
	failed := false
	for _, p := range paths {
		ws, err := checkPath(p, *domain)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", p, err)
			failed = true
			continue
		}
		warnings = append(warnings, ws...)
	}

	switch *format {
	case "json":
		if warnings == nil {
			warnings = []localengine.Warning{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(warnings)
	default:
		for _, w := range warnings {
			fmt.Fprintf(stdout, "%s %s: %s\n", w.Kind, w.Key, w.Message)
		}
	}

	if failed || (*strict && len(warnings) > 0) {
		return 1
	}
	return 0
}

// checkPath loads one argument (a locale dir or a single .yaml file) into a
// fresh store and returns its validation findings.
func checkPath(p, domain string) ([]localengine.Warning, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	store := localengine.New()
	if info.IsDir() {
		if domain == "logs" {
			err = store.LoadLogsFromOSDir(p)
		} else {
			err = store.LoadFromOSDir(p)
		}
	} else {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		fsys := fstest.MapFS{
			filepath.Base(p): &fstest.MapFile{Data: data},
		}
		if domain == "logs" {
			err = store.LoadLogsFromDir(fsys)
		} else {
			err = store.LoadFromDir(fsys)
		}
	}
	if err != nil {
		return nil, err
	}

	if domain == "logs" {
		return store.ValidateLogs(), nil
	}
	return store.Validate(), nil
}
