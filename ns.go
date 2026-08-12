package localengine

// Namespace binds a path prefix so related keys are addressed by their short
// leaf paths — the Go counterpart of the frontend's batched useLocale:
//
//	btn := localengine.NS("main", "btn")
//	btn.T("start")                 // = T("main", "btn", "start")
//	btn.Tf(vars, "start")
type Namespace struct {
	store  *Store
	lang   string // empty = the store's active language
	prefix []string
}

// NS returns a Namespace bound to the given path prefix, using the default
// Store.
func NS(path ...string) Namespace {
	return defaultStore.NS(path...)
}

// NS returns a Namespace bound to the given path prefix. Lookups use the
// store's active language.
func (s *Store) NS(path ...string) Namespace {
	return Namespace{store: s, prefix: splitPath(path)}
}

// full joins the namespace prefix with the given leaf path.
func (n Namespace) full(path []string) []string {
	p := make([]string, 0, len(n.prefix)+len(path))
	p = append(p, n.prefix...)
	return append(p, path...)
}

// T returns the localized string for the leaf path within the namespace.
func (n Namespace) T(path ...string) string {
	return n.store.tlang(n.lang, n.full(path)...)
}

// Tf returns the localized string for the leaf path within the namespace
// with {{name}} placeholders interpolated.
func (n Namespace) Tf(vars Vars, path ...string) string {
	return interpolate(n.T(path...), vars)
}

// RecordView is a flat leaf→value view of a whole subtree, resolved at
// creation time with the usual fallback chain (language, default locale,
// English, static bundles) — the Go counterpart of the frontend's wildcard
// useLocaleRecord:
//
//	phases := localengine.Record("main", "phase")
//	phases.Get(apiPhase)           // miss → full dotted key + warn-once
type RecordView struct {
	store  *Store
	prefix []string
	leaves map[string]string
}

// Record resolves the subtree at the given path into a RecordView for the
// default Store's active language.
func Record(path ...string) RecordView {
	return defaultStore.Record(path...)
}

// Record resolves the subtree at the given path into a RecordView for the
// store's active language.
func (s *Store) Record(path ...string) RecordView {
	return s.recordLang("", splitPath(path))
}

// recordLang resolves the subtree at prefix for lang (empty = the active
// language).
func (s *Store) recordLang(lang string, prefix []string) RecordView {
	r := RecordView{store: s, prefix: prefix, leaves: make(map[string]string)}

	_ = s.ensureDefaults()
	def := s.DefaultLocale()
	if lang == "" {
		lang = s.CurrentLanguage()
	}

	s.mu.RLock()
	var trees []map[string]any
	seen := map[string]bool{}
	for _, l := range []string{lang, def, "en"} {
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		trees = append(trees, s.bundles[l], s.defaults[l])
	}
	for _, name := range s.staticBundleNames() {
		trees = append(trees, s.bundles[name])
	}
	s.mu.RUnlock()

	// First tree in the chain wins per leaf, mirroring LookupString.
	seenLeaf := map[string]bool{}
	for _, tree := range trees {
		v, ok := walkTree(tree, prefix)
		if !ok {
			continue
		}
		sub, ok := v.(map[string]any)
		if !ok {
			continue
		}
		collectKeys(sub, nil, func(p []string, val any) {
			leaf := joinPath(p)
			if seenLeaf[leaf] {
				return
			}
			if str, ok := val.(string); ok && str != "" {
				seenLeaf[leaf] = true
				r.leaves[leaf] = str
			}
		})
	}
	return r
}

// Get returns the value of a leaf in the record. A missing leaf resolves to
// the full dotted key and is reported once — silence hides bugs.
func (r RecordView) Get(leaf string) string {
	if v, ok := r.leaves[leaf]; ok {
		return v
	}
	full := r.fullLeaf(leaf)
	r.store.warnMissingOnce(full)
	return joinPath(full)
}

// Leaves returns a copy of the resolved leaf→value map.
func (r RecordView) Leaves() map[string]string {
	out := make(map[string]string, len(r.leaves))
	for k, v := range r.leaves {
		out[k] = v
	}
	return out
}

// fullLeaf joins the record prefix with a (possibly dotted) leaf.
func (r RecordView) fullLeaf(leaf string) []string {
	p := make([]string, 0, len(r.prefix)+1)
	p = append(p, r.prefix...)
	return append(p, splitPath([]string{leaf})...)
}

// tlang resolves path for an explicit language; empty lang means the active
// language.
func (s *Store) tlang(lang string, path ...string) string {
	if lang == "" {
		s.mu.RLock()
		lang = s.currentLang
		s.mu.RUnlock()
	}
	if msg, ok := s.LookupString(lang, path...); ok {
		return msg
	}
	return joinPath(splitPath(path))
}
