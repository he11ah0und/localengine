# localengine

A tiny in-memory localization engine for Go. It reads nested YAML locale
files and supports lookup by path segments:

```go
msg := localengine.T("about", "btn", "open_repo")
```

## Features

- Nested YAML bundles, one file per language (`en.yaml`, `ru.yaml`, ...).
- Lookup by path segments with fallback chain: current language → English →
  static bundles → the dotted path itself (the UI is never blank).
- Static bundles (`*.static.yaml`): brand names and technical terms that are
  never translated, kept in shared files instead of being repeated in every
  locale. A project may have several static bundles; they always load, are
  not selectable languages, and are validated like any other bundle.
- Loads from any `io/fs.FS`: `embed.FS` (optionally narrowed with `fs.Sub`),
  `os.DirFS`, test fixtures via `testing/fstest.MapFS`.
- Locale parity linter: after loading, warns about keys missing in some
  languages (`locale key "x.y" missing in: ru, zh`), identical translations
  across languages (a sign of an untranslated key), duplicate values shared
  by several keys within one bundle (a sign of copy-pasted entries that
  deserve a shared key), and collisions — a key defined in more than one
  bundle when at least one of them is static.
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

Static names that are never translated live in `*.static.yaml` files next to
the locale files, using the same nested structure and dotted keys:

```yaml
app:
  title: Example App
startup:
  mode_systemd: systemd
```

Locale files must not repeat these keys: any key defined in more than one
bundle (when at least one is static) is reported as a collision.

## Notes

- Package state is global (loaded bundles, current language) and not safe
  for concurrent mutation: load locales and select the language during
  initialization, then only call `T` from other goroutines.
- The only dependency is `gopkg.in/yaml.v3`.
