package localengine

import "fmt"

// Locale domains declared in a locale file's meta header.
const (
	// MetaTypeUI marks user-facing locales (the default domain).
	MetaTypeUI = "ui"
	// MetaTypeLogs marks log-message locales resolved by log key at runtime.
	MetaTypeLogs = "logs"
)

// Locale bindings declared in a locale file's meta header.
const (
	// MetaBindingInternal marks locales whose keys are referenced from the
	// compiled code (log keys). An internal file set must contain English.
	MetaBindingInternal = "internal"
	// MetaBindingExternal marks user-facing locales (translatable content).
	MetaBindingExternal = "external"
)

// localeMeta describes a locale file: which domain its keys belong to
// (ui or logs) and how it is bound (internal = referenced from code,
// external = user-facing).
type localeMeta struct {
	Type    string
	Binding string
}

// defaultMeta is the backward-compatible meta for files without a header.
var defaultMeta = localeMeta{Type: MetaTypeUI, Binding: MetaBindingExternal}

// parseMeta extracts and validates the optional top-level meta mapping of a
// parsed locale tree. The meta key is removed from the tree so it never
// enters the key space. An absent meta yields defaultMeta; a present meta
// with unknown type or binding values is a load error.
func parseMeta(tree map[string]any) (localeMeta, error) {
	meta := defaultMeta
	raw, ok := tree["meta"]
	if !ok {
		return meta, nil
	}
	delete(tree, "meta")
	m, ok := raw.(map[string]any)
	if !ok {
		return meta, fmt.Errorf("meta must be a mapping")
	}
	if v, ok := m["type"]; ok {
		s, ok := v.(string)
		if !ok || (s != MetaTypeUI && s != MetaTypeLogs) {
			return meta, fmt.Errorf("unknown meta type %v (want %q or %q)", v, MetaTypeUI, MetaTypeLogs)
		}
		meta.Type = s
	}
	if v, ok := m["binding"]; ok {
		s, ok := v.(string)
		if !ok || (s != MetaBindingInternal && s != MetaBindingExternal) {
			return meta, fmt.Errorf("unknown meta binding %v (want %q or %q)", v, MetaBindingInternal, MetaBindingExternal)
		}
		meta.Binding = s
	}
	return meta, nil
}
