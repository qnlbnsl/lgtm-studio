import { create } from '@bufbuild/protobuf';
import { Code } from '@connectrpc/connect';
import { ClientSchema, ClientState } from '@lgtm/api/clients';
import { expect, type Page } from '@playwright/test';
import {
  observeRepositorySession, serveRepositorySession, serveWebsite, sessionProcedure, sessionRegistry, test,
} from './support/repository-server';

const firstHost = 'first.lgtm.test';
const otherHost = 'other.lgtm.test';
const worker = (id: string, name: string, state = ClientState.ONLINE) => create(ClientSchema, {
  id, name, state, address: '10.2.3.4', agentLimit: 2, harnessesScanned: true,
  harnesses: [{ harness: id === 'old-worker' ? 'gemini' : 'codex', version: '1.2.3' }],
});
const firstWorker = worker('first-worker', 'First worker');
const nextWorker = worker('next-worker', 'Next worker');
const otherWorker = worker('other-worker', 'Other worker');
const oldWorker = worker('old-worker', 'Old sample worker');
const joining = create(ClientSchema, {
  id: 'waiting-worker', name: 'waiting-worker', state: ClientState.WAITING_FOR_APPROVAL, tokenId: 'pair-1',
});
type Listener = Awaited<ReturnType<typeof serveRepositorySession>>;

const sessionTest = test.extend<{
  network: 'site' | 'loopback';
  rig: { web: string; first: Listener; next: Listener; other: Listener; shared: ReturnType<typeof sessionRegistry> };
}>({
  network: ['site', { option: true }],
  rig: async ({ website, network }, use) => {
    const host = network === 'site' ? firstHost : 'localhost';
    const other = network === 'site' ? otherHost : '127.0.0.1';
    const web = await serveWebsite(website, host);
    const listeners: Listener[] = [];
    const shared = sessionRegistry();
    const open = async (hostname: string, listener: string, registry: typeof shared, clients: (typeof firstWorker)[], sample = false) => {
      const fixture = await serveRepositorySession({ pageOrigin: web.origin, hostname, listener, registry, clients, sample });
      listeners.push(fixture);
      return fixture;
    };
    try {
      const first = await open(host, 'first', shared, network === 'site' ? [firstWorker, joining] : [oldWorker], network === 'loopback');
      const next = await open(host, 'next', shared, [nextWorker]);
      const independent = await open(other, 'other', sessionRegistry(), [otherWorker]);
      await use({ web: web.origin, first, next, other: independent, shared });
    } finally {
      await Promise.all([web.stop(), ...listeners.map((listener) => listener.stop())]);
    }
  },
});

sessionTest.use({ launchOptions: {
  args: [`--host-resolver-rules=MAP ${firstHost} 127.0.0.1, MAP ${otherHost} 127.0.0.1`, '--no-proxy-server'],
} });

function navigate(page: Page, route: string) {
  return page.evaluate((hash) => { location.hash = hash; }, route);
}

function choose(page: Page, address: string) {
  return page.evaluate((value) => { void window.repositorySessionProbe.bridge!.connect(value); }, address);
}

function signIn(page: Page) {
  return page.evaluate(() => { void window.lgtm.act.signIn('browser fixture input'); });
}

async function submitLogin(page: Page) {
  await page.getByLabel('Password', { exact: true }).fill('browser fixture input');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
}

function signOut(page: Page) {
  return page.evaluate(() => { void window.lgtm.act.signOut(); });
}

function startCookieWriter(page: Page, writer: 'signIn' | 'signOut') {
  return page.evaluateHandle((method) => ({
    done: method === 'signIn' ? window.lgtm.act.signIn('browser fixture input') : window.lgtm.act.signOut(),
  }), writer);
}

async function chooseInLogin(page: Page, listener: Listener) {
  await page.getByRole('button', { name: /^Change the server\./ }).click();
  await page.getByLabel('Server address', { exact: true }).fill(listener.origin);
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
}

function selected(page: Page) {
  return page.evaluate(() => {
    const s = window.lgtm.store.state;
    return {
      address: s.server.address, remembered: localStorage.getItem('lgtm:server'), server: s.server.status,
      auth: s.session.status, hash: location.hash, machines: s.machines.map((m) => m.id),
    };
  });
}

function ready(page: Page, listener: Listener, auth: string, machines: string[] = []) {
  return expect.poll(() => selected(page)).toMatchObject({
    address: listener.origin, remembered: listener.origin, server: 'ready', auth, machines,
  });
}

function observed(listener: Listener, method: ReturnType<Listener['calls']>[number]['method'], cookie: string) {
  return expect.poll(() => listener.calls().filter((call) => call.method === method).at(-1)?.cookie).toBe(cookie);
}

function browserCount(page: Page, listener: Listener, method: keyof typeof sessionProcedure) {
  return page.evaluate((url) => window.repositorySessionProbe.requests.filter((request) => request === url).length,
    listener.origin + sessionProcedure[method]);
}

async function settle(page: Page) {
  // Let the delivered response and its queued promise continuations reach the application.
  await page.evaluate(() => new Promise<void>((done) => {
    const channel = new MessageChannel();
    channel.port1.onmessage = () => { channel.port1.close(); channel.port2.close(); done(); };
    channel.port2.postMessage(null);
  }));
}

async function holdBrowserReply(page: Page, listener: Listener, method: keyof typeof sessionProcedure) {
  const held = await page.evaluateHandle(({ origin, match }) => window.repositorySessionProbe.hold(origin, match),
    { origin: listener.origin, match: sessionProcedure[method] });
  return {
    ready: () => expect.poll(async () => {
      await page.clock.runFor(16);
      return held.evaluate((reply) => reply.ready);
    }).toBe(true),
    release: async () => {
      await held.evaluate((reply) => reply.release());
      await settle(page);
      await held.dispose();
    },
  };
}

async function stageSessionRecords(page: Page) {
  await page.getByRole('button', { name: 'Show details for First worker', exact: true }).click();
  await page.evaluate(() => {
    const { state } = window.lgtm.store;
    state.drafts['thread:departing-record'] = 'A draft from the departing session.';
    state.ui.panelAgent = 'departing-record';
    window.lgtm.act.setPref('theme', 'dark');
    window.lgtm.act.toast('Notice from the departing session.');
    window.lgtm.act.openDialog({ type: 'rename', title: 'Old dialog', value: 'First worker' });
  });
  await expect(page.getByRole('dialog', { name: 'Old dialog' })).toBeVisible();
}

function sessionRecords(page: Page) {
  return page.evaluate(() => {
    const s = window.lgtm.store.state;
    return {
      machines: s.machines.length, joining: s.joining.length, synced: s.server.synced,
      expanded: Boolean(s.expanded['machine:first-worker']), panel: s.ui.panelAgent ?? null,
      dialog: s.dialog?.type ?? null, toasts: s.toasts.length, drafts: Object.keys(s.drafts).length, theme: s.prefs.theme,
    };
  });
}

const emptySession = { machines: 0, joining: 0, synced: false, expanded: false, panel: null, dialog: null, toasts: 0, drafts: 0, theme: 'dark' };

async function enter(page: Page, rig: { web: string; first: Listener; next: Listener; other: Listener }) {
  const trace = await observeRepositorySession(page, [rig.web, rig.first.origin, rig.next.origin, rig.other.origin]);
  await page.goto(rig.web + '/#/machines');
  await expect.poll(() => page.evaluate(() => typeof window.repositorySessionProbe.bridge?.connect)).toBe('function');
  return trace;
}

async function firstSession(page: Page, first: Listener) {
  await choose(page, first.origin);
  await ready(page, first, 'out');
  await signIn(page);
  await ready(page, first, 'in', [firstWorker.id]);
  await observed(first, 'watch', 'accepted');
}

async function proveThreeListeners(page: Page, rig: { first: Listener; next: Listener; other: Listener }) {
  await firstSession(page, rig.first);
  await choose(page, rig.next.origin);
  await ready(page, rig.next, 'in', [nextWorker.id]);
  await observed(rig.next, 'watch', 'accepted');
  await choose(page, rig.other.origin);
  await ready(page, rig.other, 'out');
  await observed(rig.other, 'discovery', 'missing');
  await signIn(page);
  await ready(page, rig.other, 'in', [otherWorker.id]);
  await observed(rig.other, 'watch', 'accepted');
  await choose(page, rig.first.origin);
  await ready(page, rig.first, 'in', [firstWorker.id]);
  await observed(rig.first, 'watch', 'accepted');
}

async function stageScan(page: Page, first: Listener) {
  const reply = await holdBrowserReply(page, first, 'scan');
  await scan(page);
  await reply.ready();
  await observed(first, 'scan', 'accepted');
  return reply;
}

async function scan(page: Page) {
  await page.getByRole('button', { name: 'Actions for First worker', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Scan harnesses again', exact: true }).click();
}

async function removeFirstWorker(page: Page) {
  await page.getByRole('button', { name: 'Actions for First worker', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Remove machine', exact: true }).click();
  await page.getByRole('dialog', { name: 'Remove First worker?' })
    .getByRole('button', { name: 'Remove machine', exact: true }).click();
}

sessionTest.describe('session boundaries discard late replies', () => {
  for (const boundary of ['watch expiry', 'protected call expiry', 'stalled sign-out', 'server change'] as const) {
    sessionTest(`${boundary} drops session records before an obsolete action completes`, async ({ page, rig }) => {
      const { first, other } = rig;
      const trace = await enter(page, rig);
      await firstSession(page, first);
      const oldAction = await stageScan(page, first);
      let protectedCallAborted = false;
      if (boundary === 'server change') {
        const pending = first.hold('scan');
        await scan(page);
        await pending.arrived;
        void pending.aborted.then(() => { protectedCallAborted = true; });
      }
      await stageSessionRecords(page);
      expect(await sessionRecords(page)).toEqual({
        machines: 1, joining: 1, synced: true, expanded: true, panel: 'departing-record',
        dialog: 'rename', toasts: 1, drafts: 1, theme: 'dark',
      });

      if (boundary === 'watch expiry' || boundary === 'protected call expiry') {
        if (boundary === 'watch expiry') first.send(Code.Unauthenticated);
        else {
          first.refuse('create', Code.Unauthenticated);
          await page.evaluate(() => window.lgtm.act.openDialog({ type: 'pair-machine' }));
          await expect.poll(async () => {
            await page.clock.runFor(16);
            return first.calls().some(({ method }) => method === 'create');
          }).toBe(true);
        }
        await expect.soft.poll(async () => (await selected(page)).auth, { timeout: 1200 }).toBe('expired');
        expect.soft(await sessionRecords(page), 'Expiry clears records and dialogs.').toEqual(emptySession);
        expect.soft(await page.evaluate(() => window.repositorySessionProbe.watchSignals[0]?.signal?.aborted),
          'Expiry aborts the browser watch scope.').toBe(true);
        if ((await selected(page)).auth === 'expired') {
          await expect(page.getByLabel('Password', { exact: true })).toBeVisible();
          await signIn(page);
          await ready(page, first, 'in', [firstWorker.id]);
        }
      } else if (boundary === 'stalled sign-out') {
        const logout = first.hold('signOut');
        await signOut(page);
        await logout.arrived;
        await expect.poll(() => first.watchAborts).toContain(1);
        expect.soft(await sessionRecords(page), 'Sign-out clears records while the server still holds its response.').toEqual(emptySession);
        await expect.soft(page.getByRole('dialog', { name: 'Old dialog' })).toHaveCount(0, { timeout: 1000 });
        logout.release();
        await ready(page, first, 'out');
        await signIn(page);
        await ready(page, first, 'in', [firstWorker.id]);
      } else {
        const nextDiscovery = other.hold('discovery');
        await choose(page, other.origin);
        await nextDiscovery.arrived;
        await expect.poll(() => first.watchAborts).toContain(1);
        await expect.soft.poll(() => protectedCallAborted, { timeout: 1200 }).toBe(true);
        expect.soft(await sessionRecords(page), 'Selecting a server clears records while its discovery is pending.').toEqual(emptySession);
        await expect(page.getByRole('dialog', { name: 'Old dialog' })).toHaveCount(0);
        nextDiscovery.release();
        await ready(page, other, 'out');
        await signIn(page);
        await ready(page, other, 'in', [otherWorker.id]);
        await observed(other, 'watch', 'accepted');
      }

      await navigate(page, '/machines?view=current');
      const current = await selected(page);
      expect(current.hash).toBe('#/machines?view=current');
      await oldAction.release();
      expect.soft(await selected(page)).toEqual(current);
      expect.soft(await page.evaluate(() => window.lgtm.store.state.toasts.map((toast) => toast.text)),
        'A stale protected action leaves no notice in the current session.').toEqual([]);
      expect(trace.errors).toEqual([]);
    });
  }

  for (const writer of ['signIn', 'signOut'] as const) {
    sessionTest(`same hostname discovery observes the cookie after ${writer}`, async ({ page, rig }) => {
      const { first, next, shared } = rig;
      await enter(page, rig);
      await proveThreeListeners(page, rig);
      if (writer === 'signIn') {
        await signOut(page);
        await ready(page, first, 'out');
      }
      const start = shared.calls.length;
      const beforeDiscovery = await browserCount(page, next, 'discovery');
      const response = first.hold(writer);
      await (writer === 'signIn' ? signIn(page) : signOut(page));
      await response.arrived;
      await choose(page, next.origin);
      await settle(page);
      expect.soft(await browserCount(page, next, 'discovery'), 'Discovery stays queued while the cookie writer is held.').toBe(beforeDiscovery);
      response.release();

      const cookie = writer === 'signIn' ? 'accepted' : 'missing';
      await ready(page, next, writer === 'signIn' ? 'in' : 'out', writer === 'signIn' ? [nextWorker.id] : []);
      expect(shared.calls.slice(start).filter(({ method }) => method === writer || method === 'discovery'),
        'Discovery dispatch follows the same-host cookie writer and carries the resulting cookie.').toEqual([
        { listener: 'first', method: writer, cookie: writer === 'signIn' ? 'missing' : 'accepted' },
        { listener: 'next', method: 'discovery', cookie },
      ]);
      if (writer === 'signIn') await observed(next, 'watch', 'accepted');
    });

    sessionTest(`another hostname progresses while ${writer} is held and discards its late completion`, async ({ page, rig }) => {
      const { first, other } = rig;
      const trace = await enter(page, rig);
      await proveThreeListeners(page, rig);
      if (writer === 'signIn') {
        await signOut(page);
        await ready(page, first, 'out');
      }
      const response = first.hold(writer);
      const oldWriter = await startCookieWriter(page, writer);
      await response.arrived;
      await choose(page, other.origin);
      await ready(page, other, 'in', [otherWorker.id]);
      await observed(other, 'discovery', 'accepted');
      await observed(other, 'watch', 'accepted');
      await navigate(page, '/machines?view=independent');
      const current = await selected(page);
      const beforeWatch = await browserCount(page, other, 'watch');
      response.release();
      await page.evaluate(async ({ done }) => { await done; }, oldWriter);
      await settle(page);

      expect.soft(await selected(page), 'The independent workspace survives the old cookie writer.').toEqual(current);
      expect.soft(await browserCount(page, other, 'watch')).toBe(beforeWatch);
      expect(trace.errors).toEqual([]);
    });

    sessionTest(`a held ${writer} has a six-second network deadline`, async ({ page, rig }) => {
      const { first } = rig;
      await enter(page, rig);
      await firstSession(page, first);
      if (writer === 'signIn') {
        await signOut(page);
        await ready(page, first, 'out');
      }
      const response = first.hold(writer);
      await (writer === 'signIn' ? signIn(page) : signOut(page));
      await response.arrived;
      let aborted = false;
      void response.aborted.then(() => { aborted = true; });
      await page.clock.runFor(5999);
      expect(aborted).toBe(false);
      await page.clock.runFor(1);
      await expect.poll(() => aborted, { timeout: 1500 }).toBe(true);
    });
  }

  sessionTest('a queued sign-in follows sign-out and opens an authenticated watch', async ({ page, rig }) => {
    const { first, shared } = rig;
    await enter(page, rig);
    await firstSession(page, first);
    const start = shared.calls.length;
    const signIns = await browserCount(page, first, 'signIn');
    const logout = first.hold('signOut');
    const oldWriter = await startCookieWriter(page, 'signOut');
    await logout.arrived;
    const newWriter = await startCookieWriter(page, 'signIn');
    await settle(page);
    expect.soft(await browserCount(page, first, 'signIn'), 'The next sign-in stays queued until sign-out responds.').toBe(signIns);
    logout.release();
    await page.evaluate(async ([out, into]) => { await Promise.all([out!.done, into!.done]); }, [oldWriter, newWriter]);
    await expect.soft.poll(() => shared.calls.slice(start), { timeout: 1200 }).toEqual([
      { listener: 'first', method: 'signOut', cookie: 'accepted' },
      { listener: 'first', method: 'signIn', cookie: 'missing' },
      { listener: 'first', method: 'watch', cookie: 'accepted' },
    ]);
    await ready(page, first, 'in', [firstWorker.id]);
  });

  sessionTest('a superseded queued discovery is never sent', async ({ page, rig }) => {
    const { first, next, other } = rig;
    await enter(page, rig);
    await proveThreeListeners(page, rig);
    const before = await browserCount(page, next, 'discovery');
    const logout = first.hold('signOut');
    const oldWriter = await startCookieWriter(page, 'signOut');
    await logout.arrived;
    await choose(page, next.origin);
    await choose(page, other.origin);
    await ready(page, other, 'in', [otherWorker.id]);
    const current = await selected(page);
    logout.release();
    await page.evaluate(async ({ done }) => { await done; }, oldWriter);
    await settle(page);
    expect.soft(await browserCount(page, next, 'discovery'), 'The obsolete queued server received no discovery.').toBe(before);
    expect(await selected(page)).toEqual(current);
  });

  for (const outcome of ['answer', 'refusal'] as const) {
    sessionTest(`a delivered discovery ${outcome} cannot replace a newer address within the same mount`, async ({ page, rig }) => {
      const { first, other } = rig;
      await enter(page, rig);
      await proveThreeListeners(page, rig);
      if (outcome === 'refusal') first.refuse('discovery', Code.Unavailable);
      const oldDiscovery = await holdBrowserReply(page, first, 'discovery');
      const pending = await page.evaluateHandle((address) => ({
        done: window.repositorySessionProbe.bridge!.connect(address),
      }), first.origin);
      await oldDiscovery.ready();
      await choose(page, other.origin);
      await ready(page, other, 'in', [otherWorker.id]);
      await navigate(page, '/machines?view=selected');
      const current = await selected(page);
      await oldDiscovery.release();
      await page.evaluate(({ done }) => done, pending);
      expect(await selected(page)).toEqual(current);
    });
  }
});

sessionTest.describe('stale machine actions cannot replace a watch', () => {
  for (const ordering of ['watch first', 'approval first'] as const) {
    sessionTest(`the watch owns the row and approval cannot close a replacement dialog, ${ordering}`, async ({ page, rig }) => {
      const { first } = rig;
      const trace = await enter(page, rig);
      await firstSession(page, first);
      const pairing = await holdBrowserReply(page, first, 'create');
      await page.getByRole('button', { name: 'Add machine', exact: true }).click();
      await pairing.ready();
      await pairing.release();
      const pair = page.getByRole('dialog', { name: 'Add a machine', exact: true });
      await pair.getByLabel('Name', { exact: true }).fill('Newly approved');
      const approval = await holdBrowserReply(page, first, 'approve');
      await pair.getByRole('button', { name: 'Confirm Newly approved', exact: true }).click();
      await approval.ready();

      await page.evaluate(() => window.lgtm.act.openDialog({ type: 'rename', title: 'Replacement dialog', value: 'First worker' }));
      const replacement = page.getByRole('dialog', { name: 'Replacement dialog' });
      await expect(replacement).toBeVisible();
      if (ordering === 'approval first') {
        await approval.release();
        expect.soft((await selected(page)).machines, 'Approval cannot insert its own row.').toEqual([firstWorker.id]);
      }
      const changed = worker(joining.id, 'Name from the watch');
      first.send({ case: 'saved', value: changed });
      await expect(page.getByText(changed.name, { exact: true })).toBeVisible();
      first.send({ case: 'removedId', value: joining.id });
      await expect(page.getByText(changed.name, { exact: true })).toHaveCount(0);
      if (ordering === 'watch first') await approval.release();

      await expect.soft(replacement, 'A completed approval leaves the replacement dialog open.').toBeVisible({ timeout: 1000 });
      expect.soft((await selected(page)).machines, 'A removed row stays absent after approval.').toEqual([firstWorker.id]);
      expect.soft(await page.evaluate((id) => ({
        expanded: Boolean(window.lgtm.store.state.expanded['machine:' + id]),
        toasts: window.lgtm.store.state.toasts.length,
        joining: window.lgtm.store.state.joining.length,
      }), joining.id)).toEqual({ expanded: false, toasts: 0, joining: 0 });
      expect(trace.errors).toEqual([]);
    });
  }

  sessionTest('a late removal refusal and a late pairing reply do not touch the next server', async ({ page, rig }) => {
    const { first, next } = rig;
    const trace = await enter(page, rig);
    await firstSession(page, first);
    first.refuse('remove', Code.FailedPrecondition);
    const removal = await holdBrowserReply(page, first, 'remove');
    await removeFirstWorker(page);
    await removal.ready();
    const token = await holdBrowserReply(page, first, 'create');
    await page.getByRole('button', { name: 'Add machine', exact: true }).click();
    await token.ready();
    const pairing = page.getByRole('dialog', { name: 'Add a machine' });
    await pairing.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(pairing).toHaveCount(0);
    await choose(page, next.origin);
    await ready(page, next, 'in', [nextWorker.id]);
    await observed(next, 'watch', 'accepted');
    await expect.poll(() => first.watchAborts).toContain(1);
    await expect(pairing).toHaveCount(0);
    const before = await selected(page);
    const revokes = await browserCount(page, next, 'revoke');

    await removal.release();
    await token.release();
    expect.soft(await selected(page)).toEqual(before);
    expect.soft(await page.evaluate(() => window.lgtm.store.state.toasts.map((toast) => toast.text)),
      'A refusal from the departed server leaves no toast.').toEqual([]);
    expect.soft(await browserCount(page, next, 'revoke'),
      'The next server never receives cleanup for the old pairing operation.').toBe(revokes);
    expect(trace.errors).toEqual([]);
  });

  sessionTest('a current removal leaves sample edits alone and waits for the watch', async ({ page, rig }) => {
    const { first } = rig;
    const trace = await enter(page, rig);
    await firstSession(page, first);
    const removal = await holdBrowserReply(page, first, 'remove');
    await removeFirstWorker(page);
    await removal.ready();
    // Seed the records at the reply boundary so each sample removal branch can run on the old code.
    const subscription = 'fixture-subscription';
    await page.evaluate(({ id, machine }) => {
      const state = window.lgtm.store.state;
      state.subscriptions.push({ id, machine, harness: 'codex', name: 'Fixture subscription', enabled: false, usage: null });
      state.projects.push({
        id: 'fixture-project', name: 'Fixture project', machine, repo: '', prefix: 'FIX', goal: 'A fixture goal.',
        stage: 'Build', phase: '', branch: '', folder: '', lead: 'fixture-agent', createdAt: 0,
      });
      state.agents.push({
        id: 'fixture-agent', name: 'Fixture agent', project: 'fixture-project', team: null, role: 'project-lead',
        harness: 'codex', subscription: id, model: '', effort: null, permission: '', state: 'working', activity: 'Working',
        task: null, context: 0, turns: 0, tokens: 0, startedAt: 0, workState: 'working',
      });
    }, { id: subscription, machine: firstWorker.id });

    await removal.release();
    expect.soft(await page.evaluate(() => {
      const state = window.lgtm.store.state;
      return { subscriptions: state.subscriptions.map(({ id }) => id), agents: state.agents.map(({ state, activity }) => ({ state, activity })) };
    }), 'A real removal does not apply the sample subscription and agent edits.')
      .toEqual({ subscriptions: [subscription], agents: [{ state: 'working', activity: 'Working' }] });
    expect.soft((await selected(page)).machines, 'The unary reply does not remove a watched machine.').toEqual([firstWorker.id]);
    first.send({ case: 'removedId', value: firstWorker.id });
    await ready(page, first, 'in');
    expect(trace.errors).toEqual([]);
  });
});

sessionTest.describe('session boundaries discard late replies from sample', () => {
  sessionTest.use({ network: 'loopback' });

  sessionTest('post-1b 1: late discovery cannot redirect the next sign-in or machine watch', async ({ page, rig }) => {
    const { first, next } = rig;
    const trace = await enter(page, rig);
    const response = await holdBrowserReply(page, first, 'discovery');
    const oldView = await page.evaluateHandle((address) => ({
      done: window.repositorySessionProbe.bridge!.connect(address),
    }), first.origin);
    await response.ready();
    await page.evaluate(() => window.repositorySessionProbe.bridge!.remount());
    await page.getByLabel('Server address', { exact: true }).fill(next.origin);
    await page.getByRole('button', { name: 'Connect', exact: true }).click();
    await ready(page, next, 'out');
    await response.release();
    await page.evaluate(({ done }) => done, oldView);
    await ready(page, next, 'out');

    await submitLogin(page);
    await expect.poll(() => page.evaluate(() => window.lgtm.store.state.machines.length)).toBe(1);
    expect.soft((await selected(page)).machines, 'The displayed worker belongs to the selected workspace.').toEqual([nextWorker.id]);
    expect.soft(next.calls().map(({ method, cookie }) => ({ method, cookie }))).toEqual([
      { method: 'discovery', cookie: 'missing' },
      { method: 'signIn', cookie: 'missing' },
      { method: 'watch', cookie: 'accepted' },
    ]);
    expect.soft(first.calls().map(({ method }) => method)).toEqual(['discovery']);
    expect(trace.errors).toEqual([]);
  });

  sessionTest('post-1b 3: late sample sign-in leaves the selected signed-out workspace alone', async ({ page, rig }) => {
    const { first, other } = rig;
    const trace = await enter(page, rig);
    await choose(page, first.origin);
    await ready(page, first, 'out');
    const response = await holdBrowserReply(page, first, 'signIn');
    await submitLogin(page);
    await response.ready();

    await chooseInLogin(page, other);
    await ready(page, other, 'out');
    await navigate(page, '/workflows');
    const current = await selected(page);
    expect(current.hash).toBe('#/workflows');
    await response.release();

    expect.soft(await selected(page), 'The departing sign-in cannot change the selected address, hash or authentication.').toEqual(current);
    expect.soft(await browserCount(page, other, 'watch'), 'The new server never starts a watch for an old sign-in.').toBe(0);
    expect(trace.errors).toEqual([]);
  });
});
