package localengine

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/he11ah0und/yamltree"
)

// LoadFromDir reads all *.yaml files from the root of fsys and parses them
// as locales. fsys may be an embed.FS (optionally narrowed with fs.Sub),
// an os.DirFS, or any other fs.FS implementation.
//
// Files named *.static.yaml load as static bundles: names that are never
// translated. They are not selectable languages and take part in validation
// like any other bundle.
func LoadFromDir(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read locale dir: %w", err)
	}
	loaded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return fmt.Errorf("read locale %s: %w", e.Name(), err)
		}
		name := strings.TrimSuffix(e.Name(), path.Ext(e.Name()))
		if isStaticBundle(name) {
			debugf("loading static bundle %s", name)
		} else {
			debugf("loading locale %s", name)
			loaded++
		}
		if err := loadLanguage(name, data); err != nil {
			return fmt.Errorf("load locale %s: %w", e.Name(), err)
		}
	}
	infof("loaded %d locale(s)", loaded)
	rebuildStaticNames()
	validateLocales()
	return nil
}

// LoadFromOSDir reads all *.yaml files from dir on the local filesystem.
func LoadFromOSDir(dir string) error {
	return LoadFromDir(os.DirFS(dir))
}

func loadLanguage(lang string, data []byte) error {
	raw, err := yamltree.LoadTree(data)
	if err != nil {
		return err
	}
	bundles[lang] = raw
	return nil
}
