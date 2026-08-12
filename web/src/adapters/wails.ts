import type { LocaleBackend } from '../backend.js';

// WailsLocaleBindings is the shape of the generated Wails bindings a
// localengine-backed app exposes for locale registration — e.g. the
// RegisterLocaleKeys / LocaleReady methods of the Wails app struct.
export interface WailsLocaleBindings {
  RegisterLocaleKeys(keys: string[]): Promise<Record<string, string>>;
  LocaleReady(): Promise<void>;
}

// wailsBackend adapts generated Wails bindings to a LocaleBackend:
//
//   import { wailsBackend } from '@he11ah0und/localengine-web/adapters/wails';
//   import { RegisterLocaleKeys, LocaleReady } from '../bindings/.../bindings.js';
//   initLocale(wailsBackend({ RegisterLocaleKeys, LocaleReady }));
export function wailsBackend(bindings: WailsLocaleBindings): LocaleBackend {
  return {
    registerKeys: (keys) => bindings.RegisterLocaleKeys(keys),
    ready: () => bindings.LocaleReady()
  };
}
