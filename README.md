# localengine

A tiny in-memory localization engine for Go. It reads nested YAML locale
files and supports lookup by path segments:

```go
msg := localengine.T("about", "btn", "open_repo")
```

## Features

- Nested YAML bundles, one file per language (`en.yaml`, `ru.yaml`, ...).
- Lookup by path segments with fallback chain: current language → English →
  the dotted path itself (the UI is never blank).
- Loads from any `io/fs.FS`: `embed.FS` (optionally narrowed with `fs.Sub`),
  `os.DirFS`, test fixtures via `testing/fstest.MapFS`.
- Locale parity linter: after loading, warns about keys missing in some
  languages (`locale key "x.y" missing in: ru, zh`).
- System language detection from `LANG`/`LC_ALL`.
- Native language names read from the bundle itself (`locale.name`).
- Optional pluggable logger (`SetLogger`); discarded when unset.

## Usage

```go
//go:embed locales
var localesFS embed.FS

func main() {
	sub, _ := fs.Sub(localesFS, "locales")
	if err := localengine.LoadFromDir(sub); err != nil {
		log.Fatal(err)
	}
	localengine.SetLanguage(localengine.DetectSystemLanguage())

	fmt.Println(localengine.T("tab", "settings"))
}
```

Loading from the OS filesystem instead:

```go
err := localengine.LoadFromOSDir("/path/to/locales")
```

## Notes

- Package state is global (loaded bundles, current language) and not safe
  for concurrent mutation: load locales and select the language during
  initialization, then only call `T` from other goroutines.
- The only dependency is `gopkg.in/yaml.v3`.
