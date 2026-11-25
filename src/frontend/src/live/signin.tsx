// Private terminal state lives only in this component. It never enters the store.
import { useState, useEffect, useRef } from 'preact/hooks';
import { live, store, engineWrite, update, uid } from '../lib';
import * as engine from '../engine';
import * as liveGit from './git';
import type { Id, Subscription } from '../types';

/** The result of a local sign-in check. */
export interface SignInCheck { state: 'local-credential' | 'signed-out' | 'missing-executable' | 'unavailable' | string; signInCommand?: string; [more: string]: unknown }
export const checkWords = (check: SignInCheck | null | undefined): string => (({ 'local-credential': 'Local sign-in found. The provider checks it when an agent starts.', 'signed-out': 'No local sign-in found.', 'missing-executable': 'The harness is not installed.', unavailable: 'Local sign-in could not be checked.' } as Record<string, string>)[check?.state as string] || 'Local sign-in could not be checked.');
export const folderLabel = (account: Partial<Subscription> | null | undefined): string => account?.home ? account.harness === 'gemini' ? account.home + '/.gemini' : account.home : '~/' + (({ claude: '.claude', codex: '.codex', gemini: '.gemini', cursor: '.cursor' } as Record<string, string>)[account?.harness as string] || 'the harness default folder');
export async function addAccount(harness: string, name: string, folder: string, home: string, machine: Id): Promise<Subscription> {
  if (!live.on) {
    const account: Subscription = { id: uid('sub'), harness, name, machine, enabled: true, signInKnown: true, signedIn: false, home: folder === 'advanced' ? home : folder === 'separate' ? '/home/maya/.lgtm/accounts/' + harness + '/sample-account' : '', usage: null, window: null, model: '' };
    update(s => s.subscriptions.push(account)); return account;
  }
  const { account } = await engine.request('client.harness.account.add', { harnessId: harness, name, folder, ...(folder === 'advanced' ? { home } : {}) });
  // Only the engine's account reply adds a row. No optimistic record is created.
  engineWrite({ subscriptions: [...store.state.subscriptions, { id: account.id, harness: account.harness, name: account.displayName || account.id, home: account.home || '', machine: 'local', enabled: account.enabled !== false, signInKnown: false, signedIn: false, model: '', usage: null, window: null }] });
  await liveGit.refreshAccounts();
  return account;
}
function applyCheck(account: Subscription, check: SignInCheck): void {
  if (live.on) { void liveGit.run('read-service', liveGit.refreshAccounts); return; }
  if (!['local-credential', 'signed-out'].includes(check.state)) return;
  const patch = { subscriptions: store.state.subscriptions.map(a => a.id === account.id ? { ...a, signInKnown: true, signedIn: check.state === 'local-credential' } : a) };
  update(s => { s.subscriptions = patch.subscriptions; });
}
// A bounded plain-text terminal. Recognise carriage returns and clear-line; discard
// other escape sequences, including OSC hyperlinks. No terminal text becomes HTML.
function terminal(): (text: string) => string {
  let lines = [''], column = 0, escape = '';
  return (text: string) => {
    for (const ch of text) {
      if (escape) {
        escape += ch;
        if (escape.length > 8192) { escape = ''; continue; }
        if (escape === '\x1b[' || escape === '\x1b]') continue;
        if (escape.startsWith('\x1b[')) {
          if (/[\x40-\x7e]/.test(ch)) {
            if (ch === 'K') { const n = escape.slice(2, -1); lines[lines.length - 1] = n === '2' ? '' : n === '1' ? ' '.repeat(column) + lines.at(-1)!.slice(column) : lines.at(-1)!.slice(0, column); }
            escape = '';
          }
        } else if (escape.startsWith('\x1b]')) { if (ch === '\x07' || escape.endsWith('\x1b\\') || escape.length > 8192) escape = ''; }
        else escape = '';
        continue;
      }
      if (ch === '\x1b') { escape = ch; continue; }
      if (ch === '\r') { column = 0; continue; }
      if (ch === '\n') { lines.push(''); column = 0; continue; }
      if (ch === '\b') { column = Math.max(0, column - 1); continue; }
      if (ch < ' ' && ch !== '\t') continue;
      const line = lines.at(-1)!; lines[lines.length - 1] = line.padEnd(column, ' ').slice(0, column) + ch + line.slice(column + 1); column++;
    }
    if (lines.length > 500) lines = lines.slice(-500);
    if (lines.join('\n').length > 65536) lines = lines.join('\n').slice(-65536).split('\n');
    return lines.join('\n');
  };
}
export function useSignIn() {
  const [session, setSession] = useState<string | null>(null), [output, setOutput] = useState(''), [check, setCheck] = useState<SignInCheck | null>(null), [error, setError] = useState(''), [pending, setPending] = useState(false);
  const current = useRef<string | null>(null), detach = useRef<(() => void) | null>(null), render = useRef(terminal()), accountRef = useRef<Subscription | null>(null), timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined), alive = useRef(true);
  const stop = () => {
    if (!current.current) return;
    if (live.on) void engine.request('client.harness.signin.stop', { sessionId: current.current }).catch(() => { if (alive.current) setError('The terminal disconnected. The service stops sign-in after a short grace period.'); });
    else (finish as (result: SignInCheck, why: string) => void)({ state: 'signed-out' }, 'The owner stopped the sign-in.');
  };
  const finish = (result: SignInCheck) => { clearTimeout(timer.current); current.current = null; if (!alive.current) return; setCheck(result); applyCheck(accountRef.current!, result); };
  useEffect(() => () => { alive.current = false; stop(); detach.current?.(); clearTimeout(timer.current); }, []);
  const start = async (account: Subscription) => {
    if (pending || current.current) return;
    setPending(true); setError(''); setCheck(null); setSession(null); setOutput(''); render.current = terminal(); accountRef.current = account;
    try {
      if (!live.on) { current.current = 'sample'; setSession('sample'); setOutput(render.current('Sample sign-in code MAYA-4821\r\nOpen https://example.test/signin and enter the code.\r\nPress any key after signing in.')); timer.current = setTimeout(() => finish({ state: 'local-credential' }), 8000); return; }
      const reply = await engine.request('client.harness.signin.start', { accountId: account.id });
      current.current = reply.sessionId;
      if (!alive.current) { stop(); return; }
      setSession(reply.sessionId);
      detach.current = engine.connection().signInOutput!(reply.sessionId, item => {
        if (item._tag === 'output') setOutput(render.current(item.text));
        else finish(item.check);
      }, () => { current.current = null; if (alive.current) { setError('The sign-in terminal disconnected. Retry after it stops.'); setCheck({ state: 'unavailable' }); } });
    } catch (e) { if (alive.current) setError((e as Error).message || 'The sign-in program could not run. Use the command in your terminal.'); }
    finally { if (alive.current) setPending(false); }
  };
  const write = (text: string) => {
    if (!current.current || !text) return;
    if (!live.on) { finish({ state: 'local-credential' }); return; }
    void engine.request('client.harness.signin.write', { sessionId: current.current, text }).catch(() => { if (alive.current) setError('The sign-in no longer accepts keys.'); });
  };
  return { start, stop, write, session, output, check, error, pending, running: !!session && !check };
}
export type SignIn = ReturnType<typeof useSignIn>;
export function TerminalPanel({ terminal: t }: { terminal: SignIn }) {
  const log = useRef<HTMLPreElement>(null);
  useEffect(() => { if (log.current) log.current.scrollTop = log.current.scrollHeight; }, [t.output]);
  const links = [...new Set((t.output.match(/https?:\/\/[^\s<>"'\x00-\x1f]+/g) || []).map(s => s.replace(/[),.;]+$/, '')).filter(s => { try { const u = new URL(s); return ['http:', 'https:'].includes(u.protocol) && !u.username && !u.password; } catch { return false; } }))].slice(0, 8);
  const keys = (e: KeyboardEvent) => {
    if (!t.running || e.isComposing || (e.key === 'Tab' && e.shiftKey) || ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'v')) return;
    const specials: Record<string, string> = { Enter: '\r', Backspace: '\x7f', Tab: '\t', Escape: '\x1b', ArrowUp: '\x1b[A', ArrowDown: '\x1b[B', ArrowRight: '\x1b[C', ArrowLeft: '\x1b[D', Delete: '\x1b[3~' };
    let text = specials[e.key] || (e.key.length === 1 && !e.metaKey ? e.ctrlKey ? String.fromCharCode(e.key.toUpperCase().charCodeAt(0) & 31) : e.key : '');
    if (text) { e.preventDefault(); e.stopPropagation(); t.write(text); }
  };
  return <aside class="signin-terminal" aria-label="Sign-in terminal">
    <div class="signin-terminal-head">
      <b>Sign-in terminal</b>
      <button type="button" class="btn sm" disabled={!t.running} onClick={t.stop}>Stop</button>
    </div>
    <pre ref={log} role="log" aria-label="Sign-in output" class="signin-output">{t.output || 'Starting sign-in…'}</pre>
    {links.map(url => <div key={url} class="signin-link">
      <a href={url} target="_blank" rel="noopener noreferrer" referrerpolicy="no-referrer">{url}</a>
      <button type="button" class="btn sm" aria-label={'Copy ' + url} onClick={() => navigator.clipboard?.writeText(url).catch(() => {})}>
        Copy
      </button>
    </div>)}
    <label class="field">
      Terminal keys
      <textarea aria-label="Sign-in terminal keys" class="input mono" rows={1} disabled={!t.running} value="" autocomplete="off" autocapitalize="off" spellcheck={'false' as unknown as boolean} onKeyDown={keys} onInput={e => { t.write(e.currentTarget.value); e.currentTarget.value = ''; }} onPaste={e => { e.preventDefault(); t.write(e.clipboardData!.getData('text').slice(0, 4096)); }} />
    </label>
    <p class="hint">
      Keys go directly to this sign-in. Closing this dialog stops it. Plain text omits colours and full-screen cursor controls.
    </p>
    {t.check && <p role="status">{checkWords(t.check)}</p>}
    {t.error && <p role="alert">{t.error}</p>}
  </aside>;
}
