import { existsSync, readdirSync, readFileSync } from 'node:fs'
import type { SignInOutput } from '@lgtm/contracts'
import { randomUUID } from 'node:crypto'
import type { IPty } from '@lydell/node-pty'
import { terminalReasons } from './terminalReasons.ts'
import { signInReasons } from './signinReasons.ts'
import { TerminalHistory } from './terminalHistory.ts'
export type SignInItem = SignInOutput
type Check = Extract<SignInOutput, { _tag: 'ended' }>['check']
type Start = { ownerSessionId?: string | undefined; kind?: 'project'; projectId?: string; worktreeId?: string; cols?: number; rows?: number; allowed?: () => boolean; accountId: string; connectionId: string; executable: string; args: ReadonlyArray<string>; cwd: string; environment: Record<string, string>; check: () => Promise<Check>; onEnd?: () => void }
type Session = { ownerSessionId: string | undefined; attachedOwnerSessionId?: string | undefined; history: TerminalHistory; reader: number; everAttached: boolean; detached: boolean; id: string; accountId: string; connectionId: string; child: IPty; queue: SignInItem[]; bytes: number; paused?: boolean; draining?: boolean; exitWatch?: ReturnType<typeof setInterval> | undefined; wake?: (() => void) | undefined; kind?: 'project'; projectId?: string; worktreeId?: string; timer?: ReturnType<typeof setTimeout>; grace?: ReturnType<typeof setTimeout> | undefined; processExited: boolean; ending: boolean; done: boolean; attached: boolean; check: Start['check']; onEnd: Start['onEnd']; exited: Promise<void> }
// Ephemeral only. No logger, event-store, board or alert dependency is available here.
export class SignInSessions {
  private sessions = new Map<string, Session>()
  private closed = false
  private revokedOwners = new Set<string>()
  terminalEnabled = false
  terminalGeneration = 0
  private options: { graceMs?: number; limitMs?: number }
  constructor(options: { graceMs?: number; limitMs?: number } = {}) { this.options = options }
  async start(input: Start) {
    const reasons = input.kind === 'project' ? terminalReasons : signInReasons
    let spawn: typeof import('@lydell/node-pty')['spawn']
    try { ({ spawn } = await import('@lydell/node-pty')) } catch { throw new Error(reasons.failed) }
    // Recheck after loading, so concurrent starts cannot exceed either limit.
    if (this.closed || input.ownerSessionId !== undefined && this.revokedOwners.has(input.ownerSessionId) || input.kind !== 'project' && input.allowed?.() === false) throw new Error(reasons.ended)
    if (input.kind === 'project' && (!this.terminalEnabled || input.allowed?.() === false)) throw new Error(terminalReasons.off)
    if (input.kind === 'project' && [...this.sessions.values()].filter(s => s.kind === 'project' && !s.ending).length >= 8) throw new Error(terminalReasons.busy)
    if (input.kind !== 'project' && [...this.sessions.values()].some(s => s.kind !== 'project' && s.accountId === input.accountId && !s.done)) throw new Error(signInReasons.busy)
    if (input.kind !== 'project' && [...this.sessions.values()].filter(s => s.kind !== 'project').length >= 16) throw new Error(signInReasons.busy)
    let child: IPty
    try { child = spawn(input.executable, [...input.args], { name: 'xterm-256color', cols: input.cols ?? 80, rows: input.rows ?? 24, cwd: input.cwd, env: input.environment, encoding: 'utf8' }) } catch { throw new Error(reasons.failed) }
    // The pinned Unix node-pty exposes native waitpid completion here before
    // its public onExit (which follows socket close). Fail closed if it changes.
    if (input.kind === 'project' && typeof (child as IPty & { _boundClose?: boolean })._boundClose !== 'boolean') {
      try { child.kill('SIGKILL') } catch {}
      throw new Error(reasons.failed)
    }
    let resolveExit!: () => void
    const exited = new Promise<void>(resolve => { resolveExit = resolve })
    const id = randomUUID()
    const session: Session = { ownerSessionId: input.ownerSessionId, history: new TerminalHistory(), reader: 0, everAttached: false, detached: false, id, ...(input.kind ? { kind: input.kind } : {}), ...(input.worktreeId === undefined ? {} : { worktreeId: input.worktreeId }), ...(input.projectId === undefined ? {} : { projectId: input.projectId }), accountId: input.accountId, connectionId: input.connectionId, child, queue: [], bytes: 0, processExited: false, ending: false, done: false, attached: false, check: input.check, onEnd: input.onEnd, exited,
      ...(input.kind === 'project' ? {} : { timer: setTimeout(() => this.finish(session, 'The sign-in reached its time limit.'), this.options.limitMs ?? 5 * 60_000) }) }
    session.timer?.unref()
    this.sessions.set(id, session)
    child.onData(text => {
      if (session.ending) return
      if (session.kind !== 'project' && session.bytes + Buffer.byteLength(text) > 64 * 1024) {
        this.finish(session, 'The sign-in produced too much unread output.'); return
      }
      if (session.kind === 'project') {
        // Retain at most 64 KiB of UTF-8 output, in this object only.
        const turnedOver = session.history.append(text)
        if (session.detached) {
          if (turnedOver) this.pause(session)
          return
        }
      }
      session.bytes += Buffer.byteLength(text); session.queue.push({ _tag: 'output', text })
      // Stop reading the PTY so the kernel applies backpressure to the shell.
      // The queue can exceed the high mark by at most the current native chunk.
      if (session.kind === 'project' && session.bytes >= 128 * 1024) this.pause(session)
      session.wake?.()
    })
    child.onExit(() => { this.processExit(session); resolveExit(); this.finish(session, session.kind === 'project' ? 'The shell exited.' : 'The sign-in program exited.') })
    // A start whose caller never attaches cannot leave a process behind.
    session.grace = setTimeout(() => this.finish(session, 'The terminal disconnected.'), this.graceMs(session))
    session.grace.unref()
    return { id, pid: child.pid }
  }
  private graceMs(s: Session) { return this.options.graceMs ?? (s.kind === 'project' && s.ownerSessionId !== undefined ? 5 * 60_000 : 5000) }
  private pause(s: Session) {
    if (s.paused || s.draining || s.ending) return
    s.paused = true; s.child.pause()
    // node-pty destroys an unread socket 200 ms after native exit. Resume
    // before that deadline and drain the finite tail without pausing again.
    s.exitWatch = setInterval(() => {
      if ((s.child as IPty & { _boundClose: boolean })._boundClose) this.processExit(s)
    }, 15)
    s.exitWatch.unref()
  }
  private resume(s: Session) {
    if (!s.paused) return
    s.paused = false; clearInterval(s.exitWatch); s.exitWatch = undefined; s.child.resume()
  }
  private get(id: string, connection: string, kind?: 'project', ownerSessionId?: string) {
    const s = this.sessions.get(id), reasons = kind === 'project' ? terminalReasons : signInReasons;
    if (!s || s.kind !== kind) throw new Error(reasons.missing);
    const allowed = kind === 'project' && s.ownerSessionId !== undefined
      ? ownerSessionId !== undefined && !this.revokedOwners.has(ownerSessionId)
      : s.connectionId === connection && s.ownerSessionId === ownerSessionId;
    if (!allowed) throw new Error(reasons.connection);
    return s;
  }
  private processExit(s: Session) {
    if (s.processExited) return
    s.processExited = true; clearInterval(s.exitWatch); s.exitWatch = undefined
    if (s.kind === 'project' && !s.ending) this.stopSessionMembers(s)
    if (s.paused) { s.paused = false; s.draining = true; s.child.resume() }
  }
  private stopSessionMembers(s: Session) {
    // A fork during one pass is discovered by the next. The bound keeps
    // shutdown finite even if a job forks continuously or remains a zombie.
    for (let pass = 0; pass < 5; pass++) {
      if (s.processExited && existsSync('/proc/' + s.child.pid)) return
      let found = false
      try { for (const entry of readdirSync('/proc')) {
        if (!/^\d+$/.test(entry)) continue
        try {
          const member = () => { const stat = readFileSync('/proc/' + entry + '/stat', 'utf8'); return Number(stat.slice(stat.lastIndexOf(')') + 2).split(' ')[3]) === s.child.pid }
          if (!member()) continue
          const pid = Number(entry)
          if (s.processExited && pid === s.child.pid) continue
          // A reappearing former leader is a reused identifier. Signal neither
          // it nor any members of that new session, including between passes.
          if (s.processExited && existsSync('/proc/' + s.child.pid)) return
          if (member()) { found = true; process.kill(pid, 'SIGKILL') }
        } catch {}
      } } catch { return } // No /proc means no descendant discovery.
      if (!found) return
    }
  }
  private finish(s: Session, reason: string) {
    if (s.ending) return
    s.ending = true; clearTimeout(s.timer); clearTimeout(s.grace); clearInterval(s.exitWatch); s.exitWatch = undefined; s.onEnd?.()
    // /proc session membership survives the leader's exit. Never signal the
    // former leader PID or its former process group after its exit callback.
    if (s.kind === 'project' && !s.processExited) this.stopSessionMembers(s)
    if (!s.processExited) {
      try { process.kill(-s.child.pid, 'SIGKILL') } catch {}
      try { s.child.kill('SIGKILL') } catch {}
    }
    void s.exited.then(() => s.check()).catch(() => ({ state: 'unavailable' as const })).then(check => { s.queue.push({ _tag: 'ended', reason, check }); s.done = true; s.wake?.(); s.grace = setTimeout(() => this.sessions.delete(s.id), 5000); s.grace.unref() })
  }
  write(id: string, connection: string, text: string, kind?: 'project', ownerSessionId?: string) { const s = this.get(id, connection, kind, ownerSessionId); if (s.ending) throw new Error(kind ? terminalReasons.ended : signInReasons.ended); if (Buffer.byteLength(text) > 4096) throw new Error(kind ? terminalReasons.bounds : signInReasons.bounds); s.child.write(text) }
  resize(id: string, connection: string, cols: number, rows: number, kind?: 'project', ownerSessionId?: string) { const s = this.get(id, connection, kind, ownerSessionId); if (s.ending) throw new Error(kind ? terminalReasons.ended : signInReasons.ended); if (!Number.isInteger(cols) || !Number.isInteger(rows) || cols < 20 || cols > 240 || rows < 5 || rows > 100) throw new Error(kind ? terminalReasons.bounds : signInReasons.bounds); s.child.resize(cols, rows) }
  stop(id: string, connection: string, kind?: 'project', ownerSessionId?: string) { this.finish(this.get(id, connection, kind, ownerSessionId), kind ? 'The owner closed the terminal.' : 'The owner stopped the sign-in.') }
  hasProjectTerminal(worktrees: ReadonlySet<string>) { return [...this.sessions.values()].some(s => s.kind === 'project' && !s.ending && s.worktreeId !== undefined && worktrees.has(s.worktreeId)) }
  stopTerminals(reason: string) { this.terminalGeneration++; this.terminalEnabled = false; for (const s of this.sessions.values()) if (s.kind === 'project') this.finish(s, reason) }
  list(ownerSessionId?: string, connection?: string) {
    return [...this.sessions.values()].filter(s => s.kind === 'project' && !s.ending && (ownerSessionId === undefined
      ? s.ownerSessionId === undefined && s.connectionId === connection
      : !this.revokedOwners.has(ownerSessionId) && s.ownerSessionId !== undefined))
      .map(s => ({ sessionId: s.id, projectId: s.projectId!, worktreeId: s.worktreeId! }));
  }
  signOut(ownerSessionId: string, remaining: ReadonlySet<string> = new Set()) {
    this.revokedOwners.add(ownerSessionId);
    for (const s of this.sessions.values()) {
      if (s.ownerSessionId === undefined) continue;
      if (s.kind === 'project') {
        if (!remaining.size) { s.history.clear(); this.finish(s, 'The owner signed out.'); }
        else {
          if (s.ownerSessionId === ownerSessionId) s.ownerSessionId = s.attachedOwnerSessionId !== undefined && remaining.has(s.attachedOwnerSessionId) ? s.attachedOwnerSessionId : remaining.values().next().value!;
          if (s.attachedOwnerSessionId === ownerSessionId) this.detach(s.id, s.connectionId);
        }
      } else if (s.ownerSessionId === ownerSessionId) this.finish(s, 'The owner signed out.');
    }
  }
  detach(id: string, connection: string, reader?: number) {
    const s = this.sessions.get(id)
    if (!s || s.connectionId !== connection || s.ending || reader !== undefined && s.reader !== reader) return
    if (s.grace && (s.kind !== 'project' || s.detached)) return
    clearTimeout(s.grace)
    if (s.kind === 'project') {
      s.attached = false; s.attachedOwnerSessionId = undefined; s.detached = true; s.reader++; s.queue = []; s.bytes = 0; s.wake?.()
      this.resume(s)
    }
    s.grace = setTimeout(() => this.finish(s, 'The terminal disconnected.'), this.graceMs(s)); s.grace.unref()
  }
  disconnect(connection: string) { for (const s of this.sessions.values()) this.detach(s.id, connection) }
  async *output(id: string, connection: string, kind?: 'project', onAttached?: (reader: number) => void, ownerSessionId?: string): AsyncGenerator<SignInItem> {
    const s = this.get(id, connection, kind, ownerSessionId)
    if (s.attached) {
      if (kind !== 'project' || ownerSessionId === undefined) throw new Error(kind ? terminalReasons.stream : signInReasons.stream);
      s.reader++; s.wake?.();
    }
    const replaying = kind === 'project' && (s.everAttached || s.detached) && !s.ending
    if (replaying) {
      const replay = s.history.replay();
      s.queue = replay ? [{ _tag: 'output', text: replay }] : []; s.bytes = Buffer.byteLength(replay)
    }
    s.connectionId = connection; s.detached = false; s.everAttached = true
    const reader = ++s.reader
    s.attached = true; s.attachedOwnerSessionId = ownerSessionId; onAttached?.(reader); clearTimeout(s.grace); s.grace = undefined
    // A replay is below the high mark, including an empty escape-safe replay.
    // Resume only after clearing detached, so new data enters the live queue.
    if (replaying) this.resume(s)
    try { for (;;) {
      if (s.reader !== reader) {
        if (kind === 'project' && s.attached && !s.ending) yield { _tag: 'ended', reason: terminalReasons.takeover, check: { state: 'unavailable' } };
        return;
      }
      if (!s.queue.length) { if (s.done) return; let wake!: () => void; await new Promise<void>(resolve => { wake = resolve; s.wake = resolve }); if (s.wake === wake) s.wake = undefined; continue }
      const item = s.queue.shift()!; if (item._tag === 'output') { s.bytes -= Buffer.byteLength(item.text); if (!s.ending && s.bytes <= 32 * 1024) this.resume(s) } yield item; if (item._tag === 'ended') return
    } } finally { if (s.reader === reader) { s.attached = false; s.queue = []; s.bytes = 0; if (s.done) this.sessions.delete(id); else this.detach(id, connection) } }
  }
  close() { this.closed = true; for (const s of this.sessions.values()) this.finish(s, 'The service stopped.'); this.sessions.clear() }
}
