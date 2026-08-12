import { get, writable } from 'svelte/store';
import { onDestroy } from 'svelte';
import { getBackend } from './backend.js';

export interface LocaleState {
  language: string;
  values: Record<string, string>;
}

// locale holds the active language and a flat map of registered translation values.
export const locale = writable<LocaleState>({
  language: 'en',
  values: {}
});

// registeredKeys remembers every key ever sent to the backend so a key
// is registered exactly once per session, no matter how many components
// subscribe to it.
const registeredKeys = new Set<string>();
const pendingKeys = new Set<string>();
let ready = false;
let flushScheduled = false;

function flush() {
  flushScheduled = false;
  if (pendingKeys.size === 0) return;
  const backend = getBackend();
  if (!backend) return;
  const keys = Array.from(pendingKeys);
  pendingKeys.clear();
  backend
    .registerKeys(keys)
    .then((values) => {
      setLocaleValues(values as Record<string, string> | null);
    })
    .catch((err: unknown) => {
      console.warn('registerKeys failed:', err);
    });
}

export function registerLocaleKey(key: string) {
  if (!key || registeredKeys.has(key)) return;
  registeredKeys.add(key);
  pendingKeys.add(key);
  if (!flushScheduled) {
    flushScheduled = true;
    queueMicrotask(flush);
  }
}

export function registerLocaleKeys(keys: string[] | null | undefined) {
  if (!keys) return;
  keys.forEach(registerLocaleKey);
}

export async function signalLocaleReady() {
  await Promise.resolve();
  ready = true;
  flush();
  const backend = getBackend();
  if (!backend) return;
  try {
    await backend.ready();
  } catch (err: unknown) {
    console.warn('locale ready failed:', err);
  }
}

export function setLocaleValues(values: Record<string, string> | null | undefined) {
  locale.update((l) => ({ ...l, values: { ...l.values, ...(values || {}) } }));
}

export function setLocale(language: string, values?: Record<string, string> | null) {
  locale.set({ language, values: values || {} });
}

// FormatVars is a set of named values substituted into a locale template.
export type FormatVars = Record<string, string | number>;

// format substitutes {{name}} placeholders in a template. A placeholder
// without a matching var is left as-is so a broken template is visible
// instead of silently rendering empty — same rule as the Go engine.
export function format(template: string, vars: FormatVars = {}): string {
  return template.replace(/\{\{([^{}]+)\}\}/g, (placeholder, name: string) =>
    name in vars ? String(vars[name]) : placeholder
  );
}

// DeriveName maps a dot-separated locale key to a camelCase property name:
// "main.btn.start_engine" → "mainBtnStart_engine". Dots are word
// boundaries; underscores are kept as-is.
type DeriveName<S extends string> =
  S extends `${infer Head}.${infer Tail}`
    ? `${Head}${Capitalize<DeriveName<Tail>>}`
    : S;

function deriveName(key: string): string {
  return key.replace(/\.([a-zA-Z0-9])/g, (_, c: string) => c.toUpperCase());
}

// reactiveValues builds a $state-backed object of name → translation and
// keeps it in sync with the locale store. A missing value renders the key
// itself — English fallbacks in code are forbidden; the single source of
// truth is the locale YAML files. Must be called during component init.
function reactiveValues(entries: [string, string][]): Record<string, string> {
  registerLocaleKeys(entries.map(([, key]) => key));
  const out = $state<Record<string, string>>({});
  const apply = (values: Record<string, string>) => {
    for (const [name, key] of entries) out[name] = values[key] ?? key;
  };
  apply(get(locale).values);
  onDestroy(locale.subscribe(($locale) => apply($locale.values)));
  return out;
}

// useLocale registers a batch of keys and returns a reactive object of
// translations. Array form (preferred) derives property names from the
// keys:
//
//   const L = useLocale(['main.btn.start', 'main.btn.stop']);
//   {L.mainBtnStart}
//
// Map form allows explicit aliases:
//
//   const L = useLocale({ start: 'main.btn.start', stop: 'main.btn.stop' });
//
// Format templates with the exported format: format(L.someKey, { name }).
export function useLocale<const K extends readonly string[]>(
  keys: K
): { [P in K[number] as DeriveName<P>]: string };
export function useLocale<M extends Record<string, string>>(
  map: M
): { [P in keyof M]: string };
export function useLocale(
  input: readonly string[] | Record<string, string>
): Record<string, string> {
  const entries: [string, string][] = Array.isArray(input)
    ? input.map((key) => [deriveName(key), key])
    : Object.entries(input);
  return reactiveValues(entries);
}

// WildcardProps maps wildcard entries ("main.phase.*") to nested
// namespaces: the property name derives from the prefix ("mainPhase") and
// holds a leaf → value map ("waiting_api" → "Waiting for API…").
type WildcardProps<K extends readonly string[]> = {
  [P in K[number] as P extends `${infer Prefix}.*` ? DeriveName<Prefix> : never]: Record<
    string,
    string
  >;
};

// useLocaleRecord registers a batch of keys and returns a reactive map for
// dynamic key access: loops over item lists and keys computed at render
// time ({R[item.key]}, R[`main.phase.${phase}`]). A key ending in ".*"
// registers the whole namespace and is exposed as a nested map:
//
//   const R = useLocaleRecord(['main.phase.*']);
//   {R.mainPhase[phase]}
//
// An unresolved value renders the full key — silence hides bugs; the
// backend additionally logs every genuinely missing key on registration.
// Static keys must be declared via useLocale instead.
export function useLocaleRecord<const K extends readonly string[]>(
  keys: K
): Record<string, string> & WildcardProps<K> {
  const exact: string[] = [];
  const namespaces: [string, string][] = []; // [property, prefix with trailing dot]
  for (const key of keys) {
    if (!key) continue;
    if (key.endsWith('.*')) {
      const prefix = key.slice(0, -2);
      namespaces.push([deriveName(prefix), prefix + '.']);
    } else {
      exact.push(key);
    }
  }
  registerLocaleKeys([...keys]);

  const out = $state<Record<string, unknown>>({});
  const apply = (values: Record<string, string>) => {
    for (const key of exact) out[key] = values[key] ?? key;
    for (const [prop, prefix] of namespaces) {
      // Null prototype: $state only deep-proxies plain Object/Array values;
      // any other prototype is stored as-is, so our Proxy keeps seeing every
      // read. With a plain target, the outer state proxy would short-circuit
      // missing keys to undefined and the fallback below would never fire.
      const ns: Record<string, string> = Object.create(null);
      for (const k of Object.keys(values)) {
        if (k.startsWith(prefix)) ns[k.slice(prefix.length)] = values[k];
      }
      // A missing leaf renders the full dotted key instead of vanishing.
      out[prop] = new Proxy(ns, {
        get: (target, leaf) => {
          const value = Reflect.get(target, leaf);
          if (value === undefined && typeof leaf === 'string') return prefix + leaf;
          return value;
        }
      });
    }
  };
  apply(get(locale).values);
  onDestroy(locale.subscribe(($locale) => apply($locale.values)));
  return out as Record<string, string> & WildcardProps<K>;
}
