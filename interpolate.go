package localengine

import (
	"fmt"
	"slices"
	"strings"
)

// Vars holds named values substituted into {{name}} placeholders in
// translation templates.
type Vars map[string]any

// Tf returns the localized string for the given path segments with {{name}}
// placeholders interpolated, using the default Store.
func Tf(vars Vars, path ...string) string {
	return defaultStore.Tf(vars, path...)
}

// Tf returns the localized string for the given path segments with {{name}}
// placeholders interpolated: Tf(Vars{"name": "world"}, "app", "greeting")
// renders "Hello, world!" from "Hello, {{name}}!".
func (s *Store) Tf(vars Vars, path ...string) string {
	return interpolate(s.T(path...), vars)
}

// interpolate replaces {{name}} placeholders in text with values from vars.
// A placeholder without a matching var is left as-is so a broken template is
// visible instead of silently rendering empty.
func interpolate(text string, vars Vars) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{{"+k+"}}", fmt.Sprint(v))
	}
	return text
}

// placeholders returns the unique {{name}} placeholder names found in text,
// in order of first appearance.
func placeholders(text string) []string {
	var out []string
	rest := text
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			return out
		}
		end := strings.Index(rest[start+2:], "}}")
		if end < 0 {
			return out
		}
		name := rest[start+2 : start+2+end]
		if name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
		rest = rest[start+2+end+2:]
	}
}
