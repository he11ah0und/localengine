# localengine

A tiny in-memory localization engine for Go. It reads nested YAML locale
files and supports lookup by path segments:

```go
msg := localengine.T("about", "btn", "open_repo")
```

## Features

- Nested YAML bundles, one file per language (`en.yaml`, `ru.yaml`, ...).
- Lookup by path segments or dotted strings (`T("a.b.c")` ≡ `T("a","b","c")`)
  with fallback chain: current language → default locale → English → static
  bundles → the dotted path itself (the UI is never blank).
- **Store instances**: `New()` creates an independent, thread-safe engine —
  the package-level functions are a facade over a shared default Store.
  Desktop apps use the facade; servers keep per-instance stores.
- **Built-in defaults** (`WithDefaults` / `WithDefaultsFS`): translations
  embedded in the binary that fill in keys missing from the loaded files;
  file values win. Lets upgrades add keys without touching user-edited
  files.
- **Default locale callback** (`WithDefaultLocaleFunc`): resolve the default
  language lazily from your settings storage, with `InvalidateDefault` to
  re-read it after a change.
- **Interpolation**: `{{name}}` placeholders — `Tf(Vars{"name": "world"},
  "app", "greeting")`. Unknown placeholders are left as-is (visible, not
  silent).
- **Namespaces and records** (Go counterparts of the Svelte `useLocale` /
  `useLocaleRecord`): `NS("main", "btn").T("start")` binds a prefix;
  `Record("main", "phase").Get(phase)` resolves a whole subtree for dynamic
  leaves.
- **HTTP layer**: `Middleware` with locale negotiation (`?locale=`,
  `Accept-Language` with q-values, default locale), per-request `Localizer`
  in the context, `HandleLocales` and `HandleDictionary` handlers.
- Static bundles (`*.static.yaml`): brand names and technical terms that are
  never translated, kept in shared files instead of being repeated in every
  locale. A project may have several static bundles; they always load, are
  not selectable languages, and are validated like any other bundle.
- Loads from any `io/fs.FS`: `embed.FS` (optionally narrowed with `fs.Sub`),
  `os.DirFS`, test fixtures via `testing/fstest.MapFS`.
- Meta header: a locale file may declare its domain and binding at the top
  level — `meta: {type: ui|logs, binding: internal|external}`. Absent meta
  defaults to `ui, external` (backward compatible); unknown values are load
  errors. UI files load via `LoadFromDir`, logs files via `LoadLogsFromDir`;
  each loader rejects files of the other domain.
- Logs domain: `LoadLogsFromDir` loads `logs, internal` files into a
  separate key space (no cross-domain duplicate/collision warnings) and
  requires an `en` file. `LogResolver()` returns a
  `func(key string) (format string, ok bool)` for the logger's keyed
  methods, with the same semantics as UI lookup: active language (`SetLanguage`),
  English fallback, warn-once on missing keys.
- Locale validation: after loading, the engine reports structured findings
  (`Validate() []Warning`, also logged automatically): keys missing in some
  languages, identical translations across languages (a sign of an
  untranslated key), duplicate values shared by several keys within one
  bundle, collisions with static bundles, and `{{name}}` placeholder
  mismatches across languages.
- **`localecheck` CLI** (`cmd/localecheck`): headless validation of locale
  dirs or individual files — `localecheck [-domain ui|logs] [-strict]
  [-format text|json] <path>...`. Built for CI.
- **Svelte helpers** (`web/`): the frontend half — batched key registration,
  typed `useLocale` / wildcard `useLocaleRecord`, `{{name}}` formatting, and
  Wails/HTTP backend adapters. See `web/README.md`.
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
	fmt.Println(localengine.Tf(localengine.Vars{"name": user}, "app", "greeting"))
}
```

Loading from the OS filesystem instead:

```go
err := localengine.LoadFromOSDir("/path/to/locales")
```

### Store instances (servers)

```go
store := localengine.New(
	localengine.WithDefaultsFS(embeddedLocales),
	localengine.WithDefaultLocaleFunc(settings.LanguageCode),
)
if err := store.LoadFromOSDir("/data/locales"); err != nil {
	log.Fatal(err)
}

mux.Handle("/i18n/locales", http.HandlerFunc(store.HandleLocales))
mux.Handle("/i18n/dictionary", http.HandlerFunc(store.HandleDictionary))
mux.Handle("/", store.Middleware(app))

// In a handler:
loc := localengine.FromContext(r.Context())
msg := loc.Tf(localengine.Vars{"name": name}, "mail", "welcome")
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

Logs locales live in their own directory/FS and declare the logs domain:

```yaml
meta:
  type: logs
  binding: internal
core:
  download: downloading %s
```

```go
if err := localengine.LoadLogsFromDir(logsSub); err != nil {
	log.Fatal(err)
}
log.Root.SetResolver(logger.Resolver(localengine.LogResolver()))
```

Headless validation in CI:

```console
$ localecheck -strict internal/app/locales
missing b.y: locale key "b.y" missing in: en
```

## Notes

- The package-level facade shares one default Store: load locales and select
  the language during initialization, then call `T` from any goroutine.
  Stores created with `New` are fully independent and safe for concurrent
  use.
- The only dependency is `gopkg.in/yaml.v3` (plus the author's `logger` and
  `yamltree` modules).
- Future work: CLDR-based pluralization (languages like ru and zh need
  one/few/many/other forms selected by a numeric variable).
