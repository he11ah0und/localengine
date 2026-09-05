package localengine

import (
	"strings"
)

// knownOverlayPlatforms are the GOOS values recognized as platform overlay
// suffixes in locale file names ("en.linux.yaml" overlays "en.yaml" on
// linux). Other GOOS values can be added as needed; a file whose suffix is
// not listed here loads as a regular bundle.
var knownOverlayPlatforms = map[string]bool{
	"linux":   true,
	"windows": true,
	"darwin":  true,
}

// splitPlatformOverlay reports whether fileName is a platform overlay file
// ("<lang>.<goos>.yaml") and returns the base bundle name and the platform.
func splitPlatformOverlay(fileName string) (base, platform string, ok bool) {
	name := strings.TrimSuffix(fileName, ".yaml")
	idx := strings.LastIndex(name, ".")
	if idx <= 0 {
		return "", "", false
	}
	p := name[idx+1:]
	if !knownOverlayPlatforms[p] {
		return "", "", false
	}
	return name[:idx], p, true
}

// mergePlatformOverlays deep-merges each overlay tree into the parsed base
// bundle of the same name; an overlay without a base becomes a bundle of its
// own (validation will flag the missing keys against the default locale).
func mergePlatformOverlays(parsed []parsedLocaleFile, overlays []parsedLocaleFile) []parsedLocaleFile {
	for _, o := range overlays {
		found := false
		for i := range parsed {
			if parsed[i].name == o.name {
				deepMerge(parsed[i].tree, o.tree)
				found = true
				break
			}
		}
		if !found {
			parsed = append(parsed, o)
		}
	}
	return parsed
}

// parsedLocaleFile is one parsed locale file: bundle name, key tree and the
// declared meta header.
type parsedLocaleFile struct {
	name string
	tree map[string]any
	meta localeMeta
}
