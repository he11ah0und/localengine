// LocaleBackend abstracts how the locale store talks to the host
// application: a Wails backend via generated bindings, or an HTTP server
// exposing localengine's HandleDictionary.
export interface LocaleBackend {
  // registerKeys resolves a batch of dotted keys to their translated
  // values. Keys ending in ".*" request the whole namespace.
  registerKeys(keys: string[]): Promise<Record<string, string>>;
  // ready signals that the initial key batch has been registered.
  ready(): Promise<void>;
}

let backend: LocaleBackend | null = null;
let warned = false;

// initLocale installs the backend the locale store will use. Call once
// during app startup, before any component registers keys.
export function initLocale(b: LocaleBackend): void {
  backend = b;
}

// getBackend returns the installed backend, or null after logging a single
// warning when initLocale was not called.
export function getBackend(): LocaleBackend | null {
  if (!backend && !warned) {
    warned = true;
    console.warn('localengine-web: initLocale() was not called — locale keys will not resolve');
  }
  return backend;
}
