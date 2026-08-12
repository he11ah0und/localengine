import type { LocaleBackend } from '../backend.js';

export interface HttpBackendOptions {
  // dictionaryUrl is the endpoint serving localengine's HandleDictionary,
  // including the locale query: "/i18n/dictionary?locale=ru".
  dictionaryUrl: string;
  // fetchImpl overrides fetch (tests, custom credentials).
  fetchImpl?: typeof fetch;
}

// flatten turns a nested translation tree into a flat dotted-key map.
function flatten(tree: Record<string, unknown>, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value !== null && typeof value === 'object') {
      Object.assign(out, flatten(value as Record<string, unknown>, path));
    } else if (typeof value === 'string') {
      out[path] = value;
    }
  }
  return out;
}

// httpBackend resolves locale keys from a localengine HTTP server. The whole
// dictionary is fetched once and cached; registerKeys answers from the cache
// and expands "prefix.*" wildcards client-side:
//
//   import { httpBackend } from '@he11ah0und/localengine-web/adapters/http';
//   initLocale(httpBackend({ dictionaryUrl: '/i18n/dictionary?locale=ru' }));
export function httpBackend(options: HttpBackendOptions): LocaleBackend {
  const doFetch = options.fetchImpl ?? fetch;
  let cache: Promise<Record<string, string>> | null = null;

  const dictionary = (): Promise<Record<string, string>> => {
    if (!cache) {
      cache = doFetch(options.dictionaryUrl)
        .then((res) => {
          if (!res.ok) throw new Error(`dictionary fetch failed: ${res.status}`);
          return res.json() as Promise<Record<string, unknown>>;
        })
        .then((tree) => flatten(tree));
    }
    return cache;
  };

  return {
    registerKeys: async (keys) => {
      const flat = await dictionary();
      const out: Record<string, string> = {};
      for (const key of keys) {
        if (key.endsWith('.*')) {
          const prefix = key.slice(0, -1); // keep the trailing dot
          for (const [k, v] of Object.entries(flat)) {
            if (k.startsWith(prefix)) out[k] = v;
          }
        } else if (key in flat) {
          out[key] = flat[key];
        }
      }
      return out;
    },
    ready: async () => {
      await dictionary();
    }
  };
}
