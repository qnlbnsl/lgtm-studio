// The first screen of a website that has no server yet: it asks for the server's address.
// A build that fixes the address with VITE_LGTM_SERVER never shows the field.
import { useEffect, useRef, useState } from 'preact/hooks';
import { Icon, cx } from '../lib';
import { SAMPLE, fixedAddress } from '../server/address';
import { connect } from '../server/sync';
import type { State } from '../types';

export function Connect({ s }: { s: State }) {
  const server = s.server; const busy = server.status === 'connecting';
  const [text, setText] = useState(server.address === SAMPLE ? '' : server.address);
  const field = useRef<HTMLInputElement>(null);
  useEffect(() => { if (!busy) field.current?.focus(); }, [busy, server.error]);
  const fixed = fixedAddress();
  const submit = (e: SubmitEvent) => { e.preventDefault(); if (!busy) void connect(fixed ?? text); };
  return <main class="auth">
    <div class="auth-card">
      <div class="auth-brand"><span class="auth-mark">LG</span><div><h1>LGTM Studio</h1><p>Connect to a server</p></div></div>
      <form class="auth-form" onSubmit={submit} novalidate>
        {fixed
          ? server.error && <div class="callout error" role="alert"><Icon name="wifi-off" size={14} /><span>{server.error}</span></div>
          : <div class="field">
            <label for="server-address">Server address</label>
            <input ref={field} id="server-address" class={cx('input lg mono', server.error && 'invalid')} value={text} placeholder="localhost:8787" inputMode="url" autocomplete="url"
              autocapitalize="off" spellcheck={false} aria-invalid={!!server.error} aria-describedby="server-note" disabled={busy} onInput={(e) => setText(e.currentTarget.value)} />
            <span id="server-note" class="error" role={server.error ? 'alert' : null!}>{server.error}</span>
          </div>}
        <button type="submit" class="btn primary lg block" disabled={busy}>
          {busy ? <><Icon name="loader-circle" size={14} class="auth-spin" /><span>Connecting…</span></> : <span>{fixed ? 'Try again' : 'Connect'}</span>}
        </button>
        {!fixed && <button type="button" class="btn lg block" disabled={busy} onClick={() => void connect(SAMPLE)}>Use sample data</button>}
      </form>
    </div>
  </main>;
}
