package localengine

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/he11ah0und/yamltree"
)

// LoadFromDir reads all *.yaml files from the root of fsys and parses them
// as UI locales using the default Store.
func LoadFromDir(fsys fs.FS) error {
	return defaultStore.LoadFromDir(fsys)
}

// LoadFromOSDir reads all *.yaml files from dir on the local filesystem
// using the default Store.
func LoadFromOSDir(dir string) error {
	return defaultStore.LoadFromOSDir(dir)
}

// LoadFromDirPlatform is LoadFromDir for an explicit platform instead of
// runtime.GOOS, using the default Store.
func LoadFromDirPlatform(fsys fs.FS, goos string) error {
	return defaultStore.LoadFromDirPlatform(fsys, goos)
}

// LoadFromDir reads all *.yaml files from the root of fsys and parses them
// as UI locales. fsys may be an embed.FS (optionally narrowed with fs.Sub),
// an os.DirFS, or any other fs.FS implementation.
//
// Files named *.static.yaml load as static bundles: names that are never
// translated. They are not selectable languages and take part in validation
// like any other bundle.
//
// Files named <lang>.<goos>.yaml (e.g. en.linux.yaml) are platform overlays:
// on a matching runtime.GOOS their keys are deep-merged over the base <lang>
// bundle; files for other platforms are skipped. Use LoadFromDirPlatform to
// load for an explicit platform (e.g. validating every platform in tests).
//
// A file whose meta header declares type "logs" is rejected here; load it
// with LoadLogsFromDir instead.
func (s *Store) LoadFromDir(fsys fs.FS) error {
	return s.LoadFromDirPlatform(fsys, runtime.GOOS)
}

// LoadFromDirPlatform is LoadFromDir for an explicit platform instead of
// runtime.GOOS.
func (s *Store) LoadFromDirPlatform(fsys fs.FS, goos string) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read locale dir: %w", err)
	}
	parsed := make([]parsedLocaleFile, 0, len(entries))
	var overlays []parsedLocaleFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		base, platform, isOverlay := splitPlatformOverlay(e.Name())
		if isOverlay && platform != goos {
			s.debugf("skipping platform overlay %s (for %s)", e.Name(), platform)
			continue
		}
		name, tree, meta, err := parseLocaleFile(fsys, e.Name())
		if err != nil {
			return err
		}
		if meta.Type == MetaTypeLogs {
			return fmt.Errorf("load locale %s: declared as logs, use LoadLogsFromDir", e.Name())
		}
		if isOverlay {
			s.debugf("loading platform overlay %s", e.Name())
			overlays = append(overlays, parsedLocaleFile{base, tree, meta})
			continue
		}
		parsed = append(parsed, parsedLocaleFile{name, tree, meta})
	}
	parsed = mergePlatformOverlays(parsed, overlays)

	loaded := 0
	for _, p := range parsed {
		if isStaticBundle(p.name) {
			s.debugf("loading static bundle %s", p.name)
		} else {
			s.debugf("loading locale %s (%s, %s)", p.name, p.meta.Type, p.meta.Binding)
			loaded++
		}
	}

	s.mu.Lock()
	if err := s.ensureDefaultsLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	for _, p := range parsed {
		s.bundles[p.name] = p.tree
	}
	s.rebuildStaticNames()
	s.defaultLocaleCache = ""
	s.mu.Unlock()

	s.infof("loaded %d locale(s)", loaded)
	s.validateLocales()
	return nil
}

// LoadFromOSDir reads all *.yaml files from dir on the local filesystem.
func (s *Store) LoadFromOSDir(dir string) error {
	return s.LoadFromDir(os.DirFS(dir))
}

// parseLocaleFile reads and parses one locale file: it loads the YAML tree,
// strips and validates the meta header, and returns the bundle name (file
// name without extension), the tree, and the declared meta.
func parseLocaleFile(fsys fs.FS, fileName string) (string, map[string]any, localeMeta, error) {
	data, err := fs.ReadFile(fsys, fileName)
	if err != nil {
		return "", nil, localeMeta{}, fmt.Errorf("read locale %s: %w", fileName, err)
	}
	tree, err := yamltree.LoadTree(data)
	if err != nil {
		return "", nil, localeMeta{}, fmt.Errorf("load locale %s: %w", fileName, err)
	}
	meta, err := parseMeta(tree)
	if err != nil {
		return "", nil, localeMeta{}, fmt.Errorf("load locale %s: %w", fileName, err)
	}
	name := strings.TrimSuffix(fileName, path.Ext(fileName))
	return name, tree, meta, nil
}
