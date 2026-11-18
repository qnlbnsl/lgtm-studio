import { createHash, createHmac, randomBytes, scryptSync, timingSafeEqual } from "node:crypto";
import { closeSync, constants, fchmodSync, fstatSync, fsyncSync, openSync, readFileSync, renameSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as Redacted from "effect/Redacted";
import { mintCredential } from "../../identity/credential.ts";

const digest = (text: string) => createHash("sha256").update(text).digest();
export const cookieName = "lgtm_session";
export const sessionMaxAge = 30 * 24 * 60 * 60;
export const sessionLimit = 8;
const clockSkewMs = 5 * 60 * 1000;
type Record = { verifier: string; passwordSalt: string; passwordBinding: string; createdAt: number };
// A file reader must perform the memory-hard derivation for every password
// guess. Salts are independent, and parameters cannot be supplied by the file.
const binding = (r: Record, password: string) => {
  const key = scryptSync(password, Buffer.from(r.passwordSalt, 'hex'), 32, { N: 65536, r: 8, p: 1, maxmem: 80 * 1024 * 1024 });
  try { return createHmac('sha256', key).update('lgtm-web-password-v3\0').update(r.verifier).update('\0').update(String(r.createdAt)).digest('hex'); }
  finally { key.fill(0); }
};
const equal = (a: string, b: string) => timingSafeEqual(Buffer.from(a, 'hex'), Buffer.from(b, 'hex'));
const hex = (value: unknown, length: number): value is string => typeof value === 'string' && value.length === length && /^[a-f0-9]+$/.test(value);
export const makeSession = (password: Redacted.Redacted<string> | undefined, register: (secret: Redacted.Redacted<string>) => void,
  options: { stateFolder?: string | undefined; registerHash?: ((hash: string) => void) | undefined } = {}) => {
  const plain = password === undefined ? "" : Redacted.value(password), expected = digest(plain);
  const file = options.stateFolder === undefined ? undefined : join(options.stateFolder, 'web-session.json');
  let records: Record[] = [], latestToken = '', loadFailed = false;
  const revoked = new Set<string>(), registered = new Set<string>();
  // A backward wall-clock step delays expiry; it must not revoke every device.
  const live = (r: Record) => Date.now() - r.createdAt < sessionMaxAge * 1000;
  const write = (next: Record[]) => {
    if (file === undefined) return;
    const temporary = file + '.' + randomBytes(16).toString('hex');
    let fd: number | undefined;
    try {
      fd = openSync(temporary, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
      fchmodSync(fd, 0o600);
      writeFileSync(fd, JSON.stringify({ version: 3, sessions: next })); fsyncSync(fd); closeSync(fd); fd = undefined;
      renameSync(temporary, file);
      const directory = openSync(options.stateFolder!, constants.O_RDONLY | constants.O_DIRECTORY);
      try { fsyncSync(directory); } finally { closeSync(directory); }
    } finally {
      if (fd !== undefined) closeSync(fd);
      try { unlinkSync(temporary); } catch {}
    }
  };
  const invalidate = () => {
    try { write([]); return; } catch {}
    if (file === undefined) return;
    // Emergency fallback after an atomic write failure. Do not follow a link,
    // modify a shared inode, or clear memory before storage rejects the record.
    const fd = openSync(file, constants.O_WRONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
    try {
      const stat = fstatSync(fd);
      if (!stat.isFile() || stat.nlink !== 1 || stat.uid !== process.getuid?.()) throw new Error('invalid');
      fchmodSync(fd, 0); // Even a later write/flush failure leaves a refused mode.
      fsyncSync(fd);
    } finally { closeSync(fd); }
  };
  const replace = (next: Record[]) => {
    try { write(next); }
    catch {
      // If even invalidation is impossible, keep the pre-operation memory
      // state and report failure. We never claim an unpersisted revocation.
      invalidate();
      for (const r of records) revoked.add(r.verifier);
      records = []; latestToken = '';
      throw new Error('Web session storage is unavailable.');
    }
    const kept = new Set(next.map(r => r.verifier));
    for (const r of records) if (!kept.has(r.verifier)) revoked.add(r.verifier);
    records = next;
  };
  if (file !== undefined) {
    let fd: number | undefined;
    try {
      fd = openSync(file, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
      const stat = fstatSync(fd);
      if (!stat.isFile() || (stat.mode & 0o7777) !== 0o600 || stat.uid !== process.getuid?.() || stat.nlink !== 1 || stat.size > 4096) throw new Error('invalid');
      const saved = JSON.parse(readFileSync(fd, 'utf8'));
      if (saved.version !== 3 || !Array.isArray(saved.sessions) || saved.sessions.length > sessionLimit || plain.length < 8) throw new Error('invalid');
      const parsed: Record[] = saved.sessions;
      const seen = new Set<string>();
      for (const r of parsed) {
        if (r === null || !hex(r.verifier, 64) || !hex(r.passwordSalt, 32) || !hex(r.passwordBinding, 64)
          || !Number.isSafeInteger(r.createdAt) || seen.has(r.verifier)
          || !equal(r.passwordBinding, binding(r, plain))) throw new Error('invalid');
        seen.add(r.verifier);
      }
      // A clock step can place one authenticated record in the future. Skip
      // only that record; it cannot invalidate other devices in the collection.
      const next = parsed.filter(r => live(r) && r.createdAt <= Date.now() + clockSkewMs);
      // Prove this file can be updated before accepting its cookies. If both
      // checkpoint and invalidation fail, this instance stays closed to login.
      write(next);
      records = next;
      for (const r of records) options.registerHash?.(r.verifier);
    } catch {
      records = [];
      try { invalidate(); } catch { loadFailed = true; }
    } finally { if (fd !== undefined) closeSync(fd); }
  }
  const identify = (cookie: string | undefined) => {
    const values = (cookie ?? '').split(';').map(part => part.trim()).filter(part => part.startsWith(`${cookieName}=`));
    if (values.length !== 1) return undefined;
    const candidate = values[0]!.slice(cookieName.length + 1), hash = digest(candidate);
    // Bound the collection, and compare every verifier without a string match.
    const found = records.filter(r => timingSafeEqual(hash, Buffer.from(r.verifier, 'hex')) && live(r))[0];
    if (found !== undefined && !registered.has(found.verifier)) { register(Redacted.make(candidate)); registered.add(found.verifier); }
    return found?.verifier;
  };
  return {
    rotate: () => {
      if (loadFailed) throw new Error('Web session storage is unavailable.');
      if (plain.length < 8) throw new Error('Web session unavailable.');
      const minted = mintCredential('shp_w_'), next = Redacted.value(minted.token);
      const r = { verifier: minted.tokenHash, passwordSalt: randomBytes(16).toString('hex'), passwordBinding: '', createdAt: Date.now() };
      r.passwordBinding = binding(r, plain);
      replace([...records.filter(live).sort((a, b) => a.createdAt - b.createdAt).slice(-(sessionLimit - 1)), r]);
      latestToken = next; registered.add(r.verifier); register(minted.token); options.registerHash?.(r.verifier);
    },
    revoke: (cookieOrId: string) => {
      const id = hex(cookieOrId, 64) ? cookieOrId : identify(cookieOrId);
      if (id !== undefined) replace(records.filter(r => r.verifier !== id));
    },
    expire: () => { if (records.some(r => !live(r))) replace(records.filter(live)); },
    takeRevoked: () => { const ids = [...revoked]; revoked.clear(); return ids; },
    get ids() { return records.filter(live).map(r => r.verifier); },
    identify,
    checkPassword: (value: unknown) => typeof value === "string" && value.length > 0 && Buffer.byteLength(value) <= 4096
      && timingSafeEqual(digest(value), expected),
    valid: (cookie: string | undefined) => identify(cookie) !== undefined,
    cookie: (secure: boolean) => `${cookieName}=${latestToken}; Max-Age=${sessionMaxAge}; HttpOnly; SameSite=Strict; Path=/${secure ? "; Secure" : ""}`,
  };
};
export type Session = ReturnType<typeof makeSession>;
