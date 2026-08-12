# @he11ah0und/localengine-web

Svelte helpers for apps localized with the Go `localengine` engine. One
store, batched key registration, typed key derivation — the frontend half of
the engine.

No build step: `exports` points at the TypeScript sources, your Vite/Svelte
setup compiles them. `svelte` (v5) is a peer dependency.

## Install

```json
{
  "dependencies": {
    "@he11ah0und/localengine-web": "file:../localengine/web"
  }
}
```

## Setup

Pick a backend adapter and initialize once at startup:

```ts
// Wails app — pass the generated bindings:
import { initLocale } from '@he11ah0und/localengine-web/backend';
import { wailsBackend } from '@he11ah0und/localengine-web/adapters/wails';
import { RegisterLocaleKeys, LocaleReady } from '../bindings/.../bindings.js';

initLocale(wailsBackend({ RegisterLocaleKeys, LocaleReady }));
```

```ts
// HTTP server exposing localengine's HandleDictionary:
import { initLocale } from '@he11ah0und/localengine-web/backend';
import { httpBackend } from '@he11ah0und/localengine-web/adapters/http';

initLocale(httpBackend({ dictionaryUrl: '/i18n/dictionary?locale=ru' }));
```

## Usage

```svelte
<script lang="ts">
  import { useLocale, useLocaleRecord, format } from '@he11ah0und/localengine-web';

  // Property names derive from the keys: "main.btn.start" → L.mainBtnStart.
  const L = useLocale(['main.btn.start', 'main.active.label']);

  // Wildcard namespaces for dynamic leaves:
  const R = useLocaleRecord(['main.phase.*']);
</script>

<button>{L.mainBtnStart}</button>
<p>{R.mainPhase[apiPhase]}</p>
<p>{format(L.greeting, { name: userName })}</p>
```

- Templates use `{{name}}` placeholders — the same syntax the Go engine
  interpolates with `Tf`. Unknown placeholders are left as-is.
- A missing key renders the key itself, never a blank string; the backend
  logs genuinely missing keys once per process.
- English fallbacks in code are forbidden: the locale YAML files are the
  single source of truth.
