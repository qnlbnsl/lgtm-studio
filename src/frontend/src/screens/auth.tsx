// The login page and the three first-run pages. LGTM Studio has one user and one password,
// so there is no user name, no account and no pairing here.
import { useState, useEffect, useRef } from 'preact/hooks';
import { Icon, cx, update, go, href, parseRoute, live, refuse, FRAMED } from '../lib';
import { usableSubscription, HARNESSES, machineOf } from '../data';
import * as act from '../actions';
import { AddHarness, SUPPORTED } from './harness-catalog';
import { changeServer } from '../server/sync';
import { SAMPLE, serverUrl } from '../server/address';
import * as board from '../live/board';
import * as alertsLive from '../live/alerts';
import type { TargetedEvent } from 'preact';
import type { Loose, State } from '../types';

interface ScreenProps { s: State }
// `null!` keeps a null the source passes where the attribute types want undefined. Preact 11
// writes title={null} as an empty attribute and leaves title={undefined} out, so the value stays.

const LOOPBACK = /^(localhost|127(\.\d+){3}|\[::1\])$/;
// The service accepts sign-in only over HTTPS or from the computer it runs on.
// The password travels from this page to the server, so both ends must be HTTPS or this computer.
// The sample inside the page sends nothing anywhere, so it is never blocked.
function insecureOrigin(s: State) {
  if (parseRoute().query.get('insecure') === '1') return true;
  const open = (protocol: string, hostname: string) => protocol !== 'https:' && !LOOPBACK.test(hostname);
  if (live.on) return open(location.protocol, location.hostname);
  if (s.server.address === SAMPLE) return false;
  if (open(location.protocol, location.hostname)) return true;
  try { const u = new URL(serverUrl(s.server.address)); return open(u.protocol, u.hostname); } catch { return false; }
}
function Brand({ sub }: { sub?: string }) {
  return <div class="auth-brand"><span class="auth-mark">LG</span><div><h1>LGTM Studio</h1>{sub && <p>{sub}</p>}</div></div>;
}
function Spinner({ size = 14 }: { size?: number }) { return <Icon name="loader-circle" size={size} class="auth-spin" />; }

// ---- login --------------------------------------------------------------------
export function Login({ s }: ScreenProps) {
  const status = s.session.status; // out | rejected | expired | unavailable | in
  const [pw, setPw] = useState('');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [retrying, setRetrying] = useState(false);
  const [empty, setEmpty] = useState(false);
  const [caps, setCaps] = useState(false);
  const field = useRef<HTMLInputElement>(null);
  const insecure = insecureOrigin(s);
  // The live website knows no machine until it is signed in, so it names the page's own address.
  const master = s.machines.find((m) => m.role === 'master') || s.machines[0];
  // A signed-out page knows no machine. It names the server it is connected to.
  const address = live.on ? (master ? master.address : location.host) : s.server.label;
  const down = status === 'unavailable';
  const blocked = insecure || down;
  // A screen drawn inside a frame is a picture: taking the focus there would scroll the page around it.
  useEffect(() => { if (!blocked && !FRAMED) field.current?.focus(); }, [status, blocked]);

  const submit = (e: SubmitEvent) => {
    e.preventDefault();
    if (blocked || busy) return;
    if (!pw) { setEmpty(true); field.current?.focus(); return; }
    setBusy(true);
    act.signIn(pw, () => go(live.on ? alertsLive.afterSignIn(s.session.firstRun) : s.session.firstRun ? '/setup' : '/home')).then((next) => {
      setBusy(false);
      if (next === 'rejected') setPw('');
    });
  };
  const retry = () => { setRetrying(true); act.retryService().then(() => setRetrying(false)); };
  const typed = (e: TargetedEvent<HTMLInputElement>) => { setPw(e.currentTarget.value); setEmpty(false); if (status === 'rejected') act.setSession('out'); };
  const error = empty ? 'Enter the password.' : status === 'rejected' ? 'That password is wrong. Nothing was signed in.' : null;

  return <main class="auth">
    <div class="auth-card">
      <Brand sub={master ? `Sign in to the service on ${master.name}` : live.on ? 'Sign in to the service' : 'Sign in to the server'} />
      {status === 'logout-unconfirmed' && <div class="callout error" role="alert">
        <span>
          <b>Session revocation was not confirmed.</b>
          {' Records were removed from this browser. The service may still accept this session. Sign in and try logging out again when the service is reachable.'}
        </span>
      </div>}
      {status === 'expired' && !insecure && <div class="callout info" role="status">
        <Icon name="clock" size={14} />
        <span>You were signed out because the session expired. Sign in again and you return to where you were.</span>
      </div>}
      {down && !insecure && <div class="callout error" role="alert">
        <Icon name="wifi-off" size={14} />
        <span class="grow">
          <b>The service is not answering.</b>
          {' This page could not reach '}
          <span class="mono">{address}</span>
          {'. Agents that were running keep running. Start the service again with '}
          <span class="mono">lgtm serve --detach</span>
          .
        </span>
      </div>}
      {insecure && <div class="callout warn" role="alert">
        <Icon name="shield-alert" size={14} />
        <span>
          <b>This page is not on a secure connection.</b>
          {' LGTM Studio accepts the password only over HTTPS, or from the computer the service runs on. Open the HTTPS address, or forward the port and open '}
          <span class="mono">127.0.0.1</span>
          .
        </span>
      </div>}
      <form class="auth-form" onSubmit={submit} novalidate>
        <div class="field">
          <label for="login-password">Password</label>
          <div class="auth-pass">
            <input ref={field} id="login-password" class={cx('input lg', error && 'invalid')} type={(show ? 'text' : 'password') as 'text'} value={pw} autocomplete="current-password" aria-invalid={!!error} aria-describedby="login-note" disabled={blocked || busy} onInput={typed} onKeyUp={(e) => setCaps(!!e.getModifierState?.('CapsLock'))} />
            <button type="button" class="icon-btn" aria-label={show ? 'Hide the password' : 'Show the password'} aria-pressed={show} disabled={blocked} onClick={() => { setShow(!show); field.current?.focus(); }}>
              <Icon name={show ? 'eye-off' : 'eye'} size={15} />
            </button>
          </div>
          <span id="login-note" class={error ? 'error' : 'hint'} role={error ? 'alert' : null!}>
            {error || (caps ? <span class="warn-text">Caps Lock is on.</span> : <>The one password set on the service as <span class="mono">LGTM_WEB_PASSWORD</span>.</>)}
          </span>
        </div>
        {down
          ? <button type="button" class="btn lg block" disabled={retrying} onClick={retry}>
            {retrying ? <><Spinner /><span>Trying…</span></> : <><Icon name="refresh-cw" size={14} /><span>Try again</span></>}
          </button>
          : <button type="submit" class="btn primary lg block" disabled={blocked || busy}>
            {busy ? <><Spinner /><span>Signing in…</span></> : <span>Sign in</span>}
          </button>}
      </form>
      <div class="auth-foot">
        <span class="auth-conn">
          <i class={cx('dot', down ? 'failed' : insecure ? 'waiting' : 'working')} />
          {down ? 'Service unreachable' : insecure ? 'Sign-in is blocked on this connection' : 'Service reachable'}
        </span>
        {live.on || s.server.fixed ? <span class="mono">{insecure ? location.host : address}</span>
          : <button type="button" class="link mono" aria-label={`Change the server. It is ${address}`} data-tip="Change server" onClick={changeServer}>{address}</button>}
      </div>
    </div>
  </main>;
}

// ---- first run ------------------------------------------------------------------
const STEPS = ['Harnesses', 'Repository', 'First project'];
const PLANS: Record<string, string> = { claude: 'Max', codex: 'Pro' };
const CODES: Record<string, string> = { claude: 'WXKP-3QHD', codex: 'R7MN-42TC' };

function Steps({ step, go: goTo, reach }: { step: number; go: (i: number) => unknown; reach: number }) {
  return <ol class="auth-steps" aria-label="Setup steps">
    {STEPS.map((label, i) => <li key={label} class={cx(i < step && 'done', i === step && 'now')} aria-current={i === step ? 'step' : undefined}>
      <button type="button" disabled={i > reach} onClick={() => goTo(i)}>
        <span class="mark">{i < step ? <Icon name="check" size={11} /> : i + 1}</span>
        <span class="word">{label}</span>
      </button>
    </li>)}
  </ol>;
}

/** The result of checking a repository folder in the first run. */
interface PathCheck { error?: string; name?: string; repo?: string | null; branch?: string; remote?: string | null | undefined; commits?: string; path?: string }
function checkPath(raw: string, s: State): PathCheck {
  const path = raw.trim().replace(/\/+$/, '');
  if (!path) return { error: 'Enter the folder that holds the repository.' };
  if (!/^(~|\/)/.test(path)) return { error: 'Use a full path, such as ~/projects/my-app.' };
  const name = path.split('/').pop();
  if (!name || name === '~' || path.split('/').length < 3) return { error: 'That is a folder of projects, not one repository. Add the repository folder to the path.' };
  if (/\s/.test(name)) return { error: 'No Git repository was found there. Check the spelling of the folder.' };
  const known = s.repos.find((r) => Object.values(r.folders).includes(path)) || s.repos.find((r) => r.name === name);
  return known
    ? { name: known.name, repo: known.id, branch: known.main, remote: known.remote, commits: known.id === 'lgtm' ? '1,204' : '386', path }
    : { name, repo: null, branch: 'main', remote: null, commits: '57', path };
}

export function FirstRun({ s }: ScreenProps) {
  if (live.on) return <LiveFirstRun s={s} />;
  const master = s.machines.find((m) => m.role === 'master') || s.machines[0];
  const [step, setStep] = useState(0);
  const [reach, setReach] = useState(0);
  const installing = (s.ui.installing as string | null) || null;
  const [signin, setSignin] = useState<Record<string, string>>({}); // harness -> 'waiting' | 'done'
  const [path, setPath] = useState(master.workspace + '/');
  const [checked, setChecked] = useState<PathCheck | null>(null); // result of the folder check
  const [checking, setChecking] = useState(false);
  const [gitModel, setGitModel] = useState(s.settings.gitModel || 'Haiku 5.5');
  const [goal, setGoal] = useState('');
  const [tried, setTried] = useState(false);
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);
  useEffect(() => () => timers.current.forEach(clearTimeout), []);
  const later = (fn: () => void, ms: number) => timers.current.push(setTimeout(fn, ms));

  const to = (i: number) => { setTried(false); setStep(i); setReach((r) => Math.max(r, i)); };
  const connected = Object.keys(signin).filter((h) => signin[h] === 'done');
  const startSignin = (h: string) => { setSignin((v) => ({ ...v, [h]: 'waiting' })); later(() => setSignin((v) => (v[h] === 'waiting' ? { ...v, [h]: 'done' } : v)), 2600); };
  // A harness is in the list when its program is on this machine. "Add harness" installs one more.
  const detected: [string, { name: string; version?: string | null }][] = Object.entries(HARNESSES).filter(([id, h]) => h.installed && installing !== id);
  if (installing) detected.push([installing, { name: SUPPORTED[installing]!.name }]);
  const cancelSignin = (h: string) => setSignin((v) => { const n = { ...v }; delete n[h]; return n; });

  // The folder is checked a moment after typing stops.
  useEffect(() => {
    if (step !== 1) return;
    setChecking(true); setChecked(null);
    const t = setTimeout(() => { setChecking(false); setChecked(checkPath(path, s)); }, 550);
    return () => clearTimeout(t);
  }, [path, step]);
  const found = checked && !checked.error ? checked : null;
  const suggestions = s.repos.filter((r) => r.folders[master.id]).map((r) => r.folders[master.id]);
  const goalError = !goal.trim() ? 'Write the goal.' : null;

  const finish = () => {
    setTried(true);
    if (goalError || !found) return;
    const sub = s.subscriptions.find((x) => x.harness === connected[0] && usableSubscription(x)) || s.subscriptions[0];
    act.setSession('in');
    update((st) => { st.session.firstRun = false; st.settings.gitModel = gitModel; });
    act.createProject({ repo: found.repo!, newRepo: found.repo ? null : found.name, goal: goal.trim(), machine: master.id, subscription: sub!.id });
  };
  const next = (e?: SubmitEvent) => {
    e?.preventDefault();
    setTried(true);
    if (step === 0 && connected.length) to(1);
    else if (step === 1 && found) to(2);
    else if (step === 2) finish();
  };
  const skipAll = () => { act.setSession('in'); go('/home'); };

  let body;
  if (step === 0) {
    body = <>
      <h2>Harnesses</h2>
      <ul class="auth-options auth-harnesses" aria-label="Detected harnesses">
        {detected.map(([id, h]) => {
          const state = signin[id];
          return <li key={id} class={cx('auth-option', state === 'done' && 'done')}>
            <div class="auth-h">
              <span class="grow"><b>{h.name}</b>{installing === id ? <small>Installing…</small> : h.version && <small class="num">Version {h.version}</small>}</span>
              {installing === id ? <Spinner />
                : state === 'done' ? <span class="success-text auth-ok"><Icon name="circle-check" size={14} /> Signed in{PLANS[id] ? ' · ' + PLANS[id] : ''}</span>
                : state === 'waiting' ? <button type="button" class="btn sm ghost" onClick={() => cancelSignin(id)}>Cancel</button>
                : <button type="button" class="btn sm" onClick={() => startSignin(id)}><Icon name="log-in" size={14} /><span>Sign in</span></button>}
            </div>
            {state === 'waiting' && <div class="auth-option-body">
              <p>A sign-in page opened in your browser. Approve the request there, and check that it shows this code.</p>
              <div class="auth-code" aria-label="Sign-in code">{CODES[id] || 'K4TZ-9PLW'}</div>
              <div class="row" style="gap:8px"><Spinner /><span class="muted grow">Waiting for you to approve…</span></div>
            </div>}
          </li>;
        })}
        {detected.length === 0 && <li class="auth-h muted" data-empty>No harness detected</li>}
      </ul>
      <div><AddHarness s={s} /></div>
      {tried && !connected.length && <p class="error-text auth-need" role="alert">Sign in to one harness to continue, or skip this step.</p>}
    </>;
  } else if (step === 1) {
    body = <>
      <h2>Add a repository</h2>
      <div class="field">
        <label for="setup-path">Repository folder on {master.name}</label>
        <input id="setup-path" class={cx('input lg mono', tried && checked?.error && 'invalid')} value={path} spellcheck={false} autocapitalize="off" autocomplete="off" onInput={(e) => setPath(e.currentTarget.value)} autofocus />
        <div class="auth-check" aria-live="polite">
          {checking ? <><Spinner size={13} /><span class="muted">Looking in the folder…</span></>
            : found ? <>
              <Icon name="circle-check" size={14} class="success-text" />
              <span>
                <b>Git repository found.</b>
                {' Branch '}
                <span class="mono">{found.branch}</span>
                {', '}
                {found.commits}
                {' commits'}
                {found.remote ? <>, remote <span class="mono">{found.remote}</span></> : ', no remote'}
                .
              </span>
            </>
            : checked?.error ? <>
              <Icon name="circle-alert" size={14} class={tried ? 'error-text' : 'faint'} />
              <span class={tried ? 'error-text' : 'muted'}>{checked.error}</span>
            </> : null}
        </div>
      </div>
      <div class="auth-suggest">
        <span class="label">Found in <span class="mono">{master.workspace}</span></span>
        <ul>
          {suggestions.map((p) => <li key={p}>
          <button type="button" class={cx('auth-pick', path.replace(/\/+$/, '') === p && 'on')} onClick={() => setPath(p)}>
            <Icon name="folder-git-2" size={14} />
            <span class="mono truncate">{p}</span>
          </button>
        </li>)}
        </ul>
      </div>
    </>;
  } else {
    const sub = connected.length ? HARNESSES[connected[0]!]!.name + (PLANS[connected[0]!] ? ' · ' + PLANS[connected[0]!] : '') : null;
    body = <>
      <h2>Start the first project</h2>
      <div class="field">
        <label for="setup-goal">Goal</label>
        <textarea id="setup-goal" class={cx('textarea', tried && goalError && 'invalid')} rows={4} value={goal} placeholder="Make a mock for the LGTM Studio phone app" onInput={(e) => setGoal(e.currentTarget.value)} autofocus />
        {tried && goalError && <span class="error" role="alert">{goalError}</span>}
      </div>
      <div class="field">
        <label for="setup-git">Model for commit and pull request messages</label>
        <select id="setup-git" class="select" value={gitModel} onChange={(e) => setGitModel(e.currentTarget.value)}>
          {Object.entries(HARNESSES).map(([id, h]) => <optgroup label={h.name}>{h.models.map((m) => <option value={m}>{m}</option>)}</optgroup>)}
        </select>
      </div>
      <dl class="dl auth-summary">
        <dt>Repository</dt>
        <dd>
          {found ? <><span class="mono">{found.path}</span> <span class="muted">on {found.branch}</span></> : <button type="button" class="link" onClick={() => to(1)}>Add a repository first</button>}
        </dd>
        <dt>Project lead</dt>
        <dd>
          {sub || <>
          <span class="warn-text">No subscription</span>
        </>}
        </dd>
        <dt>Machine</dt>
        <dd>{master.name} <span class="muted">· this computer</span></dd>
      </dl>
      {tried && !found && <p class="error-text auth-need" role="alert">A project needs a repository.</p>}
    </>;
  }

  const waiting = step === 0 && Object.values(signin).includes('waiting');
  return <main class="auth setup">
    <header class="auth-top"><Brand /><button type="button" class="btn ghost" onClick={skipAll}>Skip setup</button></header>
    <form class="auth-card wide" onSubmit={next} novalidate>
      <Steps step={step} reach={reach} go={to} />
      <div class="auth-step" key={step}>{body}</div>
      <div class="auth-actions">
        {step > 0 && <button type="button" class="btn lg" onClick={() => to(step - 1)}><Icon name="arrow-left" size={14} /><span>Back</span></button>}
        <span class="grow" />
        {step === 0 && !connected.length && <button type="button" class="btn lg ghost" disabled={waiting} onClick={() => to(1)}>Skip for now</button>}
        <button type="submit" class="btn primary lg" disabled={(step === 0 && waiting) || (step === 1 && checking)}>
          <span>{step === 2 ? 'Create project' : 'Continue'}</span>
          {step < 2 && <Icon name="arrow-right" size={14} />}
        </button>
      </div>
    </form>
    <p class="auth-under">Step {step + 1} of 3</p>
  </main>;
}

export function seedAuth() {}

// The saved engine steps use this same three-step layout. Discovery is a read, and
// registering a folder is the only operation that establishes whether it is a repository.
function LiveFirstRun({ s }: ScreenProps) {
  // The engine's setup record, as the live website maps it.
  const data = s.setupData as Loose | null | undefined;
  const [view, setView] = useState<number | null>(null), [path, setPath] = useState(''), [found, setFound] = useState<Loose | null>(null);
  const [goal, setGoal] = useState(''), [workspace, setWorkspace] = useState(''), [shell, setShell] = useState('');
  useEffect(() => { void board.loadSetup(); }, []);
  useEffect(() => { if (data) { setWorkspace(data.workspace); setShell(data.shell); } }, [data?.workspace, data?.shell]);
  const step = view == null ? data?.step || 0 : view;
  useEffect(() => { if (data?.completed) go('/home'); }, [data?.completed]);
  const reach = data?.step || 0;
  const working = ['read-setup', 'setup', 'setup-settings', 'setup-repository', 'create-project'].some(board.busy);
  const change = async (next: number) => {
    if (working) return;
    if (next <= reach) { setView(next); return; }
    const reply = await board.advanceSetup(next);
    if (reply) setView(next);
  };
  const next = async (event: SubmitEvent) => {
    event.preventDefault(); if (working) return;
    if (step === 0) {
      if (!workspace.trim() || !shell.trim()) return;
      const reply = await board.saveSetupSettings({ workspaceFolder: workspace.trim(), shell: shell.trim() });
      if (reply) await change(1);
    } else if (step === 1) {
      const reply = found || await board.registerSetupRepository(path);
      if (reply) { setFound(reply); await change(2); }
    } else if (goal.trim() && found) {
      const created = s.ui.setupCreated as string | undefined;
      if (created) {
        const completed = await board.run('setup', () => import('../engine').then((engine) => engine.request('project.setup.complete', { projectId: created })));
        if (completed) go(`/p/${created}/board`);
      } else await board.createProject({ repo: found.repo, goal: goal.trim() }, { setup: true });
    }
  };
  return <main class="auth setup">
    <header class="auth-top"><Brand /><button type="button" class="btn ghost" onClick={refuse}>Skip setup</button></header>
    <form class="auth-card wide" onSubmit={next} novalidate aria-busy={working}>
      <Steps step={step} reach={reach} go={change} />
      <div class="auth-step" key={step}>
        {!data ? <>
          <p class="muted" role="status">Loading setup…</p>
          <button type="button" class="btn" disabled={working} onClick={() => board.loadSetup()}>Retry</button>
        </>
          : step === 0 ? <>
            <h2>Connect a subscription</h2>
            <div class="auth-options" role="radiogroup" aria-label="Harness">
              {data.harnesses.map((h: Loose) => <div class="auth-option" key={h.harness}>
              <div class="auth-option-head">
                <span class="grow">
                  <b>{h.harness}</b>
                  <small>{h.installed ? 'Installed' : 'Not installed'} · {h.configured ? 'Configured' : 'Not configured'}</small>
                </span>
              </div>
              <div class="auth-option-body">
                {h.accounts.map((a: Loose) => <p key={a.accountId}>{a.accountId} · {a.state}</p>)}
                <button type="button" class="btn" onClick={refuse}>Sign in with {h.harness}</button>
              </div>
            </div>)}
            </div>
            {!data.available && <p class="error-text" role="alert">{data.reason}</p>}
            <div class="field">
              <label for="setup-workspace">Workspace folder</label>
              <input id="setup-workspace" class="input mono" value={workspace} disabled={working} onInput={(e) => setWorkspace(e.currentTarget.value)} />
            </div>
            <div class="field">
              <label for="setup-shell">Shell</label>
              <input id="setup-shell" class="input" value={shell} disabled={working} onInput={(e) => setShell(e.currentTarget.value)} />
            </div>
            <button type="button" class="btn" disabled={working} onClick={() => board.loadSetup()}>Refresh discovery</button>
          </>
          : step === 1 ? <>
            <h2>Add a repository</h2>
            <div class="field">
              <label for="setup-path">Repository folder on local</label>
              <input id="setup-path" class="input lg mono" value={path} disabled={working} onInput={(e) => { setPath(e.currentTarget.value); setFound(null); }} />
            </div>
            <div class="auth-check" aria-live="polite">
              {found ? <span class="success-text">Repository registered: {found.path}</span> : ''}
            </div>
            <div class="auth-suggest">
              <span class="label">Found in <span class="mono">{data.workspace}</span></span>
              <ul>
                {data.repositories.map((r: Loose) => <li key={r.path}>
                <button type="button" class="auth-pick" onClick={() => { setPath(r.path); setFound(null); }}>
                  <Icon name="folder-git-2" size={14} />
                  <span class="mono">{r.path}</span>
                </button>
              </li>)}
              </ul>
            </div>
          </>
          : <>
            <h2>Start the first project</h2>
            <div class="field">
              <label for="setup-goal">Goal</label>
              <textarea id="setup-goal" class="textarea" rows={4} value={goal} disabled={working} onInput={(e) => setGoal(e.currentTarget.value)} />
            </div>
            <div class="field">
              <label for="setup-git">Model for commit and pull request messages</label>
              <select id="setup-git" class="select" onChange={refuse}><option value="" /></select>
            </div>
            <dl class="dl auth-summary">
              <dt>Repository</dt>
              <dd>{found?.path || <button type="button" class="link" onClick={() => change(1)}>Add a repository first</button>}</dd>
              <dt>Project lead</dt>
              <dd>
                {data.lead ? `${data.lead.harness} · ${data.lead.account._tag === 'named' ? data.lead.account.accountId : 'Any ready account'}${data.lead.model ? ' · ' + data.lead.model : ''}${data.lead.effort ? ' · ' + data.lead.effort : ''}` : ''}
              </dd>
              <dt>Machine</dt>
              <dd>local</dd>
            </dl>
            {s.ui.setupCreated && <p class="muted">The project exists. Retry to finish setup after its lead is ready.</p>}
          </>}
      </div>
      <div class="auth-actions">
        {step > 0 && <button type="button" class="btn lg" disabled={working} onClick={() => change(step - 1)}>Back</button>}
        <span class="grow" />
        <button type="submit" class="btn primary lg" disabled={working || !data || (step === 0 && (!workspace.trim() || !shell.trim())) || (step === 1 && !path.trim()) || (step === 2 && (!goal.trim() || !found))}>
          {working ? 'Working…' : step === 2 ? s.ui.setupCreated ? 'Finish setup' : 'Create project' : 'Continue'}
        </button>
      </div>
    </form>
    <p class="auth-under">Step {step + 1} of 3</p>
  </main>;
}
