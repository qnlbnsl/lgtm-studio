// Only chat composer keys enter per-tab storage. Form fields never use this
// object. Keep at most 32 drafts, 8,192 characters each and 65,536 serialized
// characters in total (at most 128 KiB in UTF-16 storage).
let draftNotice: () => void = () => {};
export const setDraftNotice = (notice: () => void): void => { draftNotice = notice; };
const stores = new WeakMap<object, () => void>();
export function clearComposerDrafts(drafts: Record<string, string>): void {
  for (const key of Object.keys(drafts || {})) delete drafts[key];
  stores.get(drafts)?.();
}
const allowed = (key: string | symbol): key is string => typeof key === 'string' && /^(thread|board):.{1,256}$/.test(key);
const bounded = (value: unknown): Record<string, string> => {
  const out: Record<string, string> = Object.create(null);
  if (!value || typeof value !== 'object' || Array.isArray(value)) return out;
  for (const [key, text] of Object.entries(value).slice(-32)) {
    if (!allowed(key) || typeof text !== 'string' || !text || text.length > 8192) continue;
    out[key] = text;
    while (JSON.stringify(out).length > 65536) delete out[Object.keys(out)[0]!];
  }
  return out;
};
export function composerDrafts(mode = 'mock', framed = false, onTooLong = () => draftNotice()): Record<string, string> {
  const storageKey = 'lgtm:composer:' + mode;
  const warned = new Set<string>();
  let initial: unknown;
  try { const raw = !framed && sessionStorage.getItem(storageKey); if (raw && raw.length <= 65536) initial = JSON.parse(raw); } catch { /* storage is optional */ }
  const save = (drafts: Record<string, string>) => {
    if (framed) return;
    try {
      const saved = JSON.stringify(bounded(drafts));
      if (saved === '{}') sessionStorage.removeItem(storageKey);
      else sessionStorage.setItem(storageKey, saved);
    } catch { /* a disabled or full store must not break typing */ }
  };
  const drafts = new Proxy(bounded(initial), {
    set(target, key, value: unknown) {
      if (!allowed(key) || typeof value !== 'string') return true;
      delete target[key];
      if (value) target[key] = value;
      save(target);
      if (value.length <= 8192) warned.delete(key);
      if (value.length > 8192 && !warned.has(key)) { warned.add(key); onTooLong(); }
      return true;
    },
    deleteProperty(target, key) { delete target[key as string]; warned.delete(key as string); save(target); return true; },
  });
  stores.set(drafts, () => { if (!framed) { try { sessionStorage.removeItem(storageKey); } catch { /* optional storage */ } } });
  return drafts;
}
