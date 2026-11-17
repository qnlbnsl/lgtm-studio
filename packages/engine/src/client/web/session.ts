import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";
import { closeSync, constants, fchmodSync, fstatSync, fsyncSync, openSync, readFileSync, renameSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as Redacted from "effect/Redacted";
import { mintCredential } from "../../identity/credential.ts";

const digest = (text: string) => createHash("sha256").update(text).digest();
export const cookieName = "lgtm_session";
export const sessionMaxAge = 30 * 24 * 60 * 60;
type Record = { version: 1; verifier: string; passwordBinding: string; createdAt: number };
// The verifier is an HMAC key, not a bearer credential. Reading this record
// does not reveal the 256-bit token or allow inversion of its SHA-256 hash.
const binding = (verifier: string, password: string) => createHmac('sha256', Buffer.from(verifier, 'hex')).update('lgtm-web-password-v1\0').update(password).digest('hex');
const equal = (a: string, b: string) => timingSafeEqual(Buffer.from(a, 'hex'), Buffer.from(b, 'hex'));
export const makeSession = (password: Redacted.Redacted<string> | undefined, register: (secret: Redacted.Redacted<string>) => void,
  options: { stateFolder?: string | undefined; registerHash?: ((hash: string) => void) | undefined } = {}) => {
  const plain = password === undefined ? "" : Redacted.value(password), expected = digest(plain);
  const file = options.stateFolder === undefined ? undefined : join(options.stateFolder, 'web-session.json');
  let record: Record | undefined, token = '';
  const revoke = () => {
    record = undefined; token = '';
    if (file !== undefined) {
      try { unlinkSync(file); const directory = openSync(options.stateFolder!, constants.O_RDONLY | constants.O_DIRECTORY); try { fsyncSync(directory) } finally { closeSync(directory) } } catch (error) { if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw new Error('Web session could not be revoked.'); }
    }
  };
  const live = (r: Record) => Date.now() >= r.createdAt && Date.now() - r.createdAt < sessionMaxAge * 1000;
  if (file !== undefined) {
    let fd: number | undefined;
    try {
      fd = openSync(file, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
      const stat = fstatSync(fd);
      if (!stat.isFile() || (stat.mode & 0o7777) !== 0o600 || stat.uid !== process.getuid?.() || stat.nlink !== 1 || stat.size > 2048) throw new Error('invalid');
      const r = JSON.parse(readFileSync(fd, 'utf8'));
      if (r.version !== 1 || typeof r.verifier !== 'string' || !/^[a-f0-9]{64}$/.test(r.verifier)
        || typeof r.passwordBinding !== 'string' || !/^[a-f0-9]{64}$/.test(r.passwordBinding)
        || !Number.isSafeInteger(r.createdAt) || !live(r) || plain.length < 8 || !equal(r.passwordBinding, binding(r.verifier, plain))) throw new Error('invalid');
      record = r;
      options.registerHash?.(r.verifier);
    } catch {
      // Invalid records never authenticate. Removing a symlink removes only
      // the link. An inaccessible folder also fails closed without a crash.
      try { revoke() } catch {}
    } finally { if (fd !== undefined) closeSync(fd); }
  }
  const rotate = () => {
    revoke();
    if (plain.length < 8) throw new Error('Web session unavailable.');
    const minted = mintCredential('shp_w_'), next = Redacted.value(minted.token), verifier = minted.tokenHash;
    const r: Record = { version: 1, verifier, passwordBinding: binding(verifier, plain), createdAt: Date.now() };
    if (file !== undefined) {
      const temporary = file + '.' + randomBytes(16).toString('hex');
      let fd: number | undefined;
      try {
        fd = openSync(temporary, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
        fchmodSync(fd, 0o600);
        writeFileSync(fd, JSON.stringify(r)); fsyncSync(fd); closeSync(fd); fd = undefined;
        renameSync(temporary, file);
        const directory = openSync(options.stateFolder!, constants.O_RDONLY | constants.O_DIRECTORY);
        try { fsyncSync(directory) } finally { closeSync(directory) }
      } catch { try { unlinkSync(temporary) } catch {} throw new Error('Web session could not be saved.'); }
      finally { if (fd !== undefined) closeSync(fd); }
    }
    token = next; record = r; register(Redacted.make(next)); options.registerHash?.(verifier);
  };
  return {
    rotate, revoke,
    get id() { return record?.verifier; },
    get expired() { return record !== undefined && !live(record); },
    checkPassword: (value: unknown) => typeof value === "string" && value.length > 0 && Buffer.byteLength(value) <= 4096
      && timingSafeEqual(digest(value), expected),
    valid: (cookie: string | undefined) => {
      if (record === undefined || !live(record)) return false;
      const values = (cookie ?? "").split(";").map((part) => part.trim()).filter((part) => part.startsWith(`${cookieName}=`));
      if (values.length !== 1) return false;
      const candidate = values[0]!.slice(cookieName.length + 1);
      const accepted = timingSafeEqual(digest(candidate), Buffer.from(record.verifier, 'hex'));
      if (accepted && token === '') { token = candidate; register(Redacted.make(candidate)); }
      return accepted;
    },
    cookie: (secure: boolean) => `${cookieName}=${token}; Max-Age=${sessionMaxAge}; HttpOnly; SameSite=Strict; Path=/${secure ? "; Secure" : ""}`,
  };
};
export type Session = ReturnType<typeof makeSession>;
