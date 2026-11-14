import type { SignInOutput } from '@lgtm/contracts'
import { randomUUID } from 'node:crypto'
import type { IPty } from '@lydell/node-pty'
import { signInReasons } from './signinReasons.ts'
export type SignInItem = SignInOutput
type Check = Extract<SignInOutput, { _tag: 'ended' }>['check']
type Start = { accountId: string; connectionId: string; executable: string; args: ReadonlyArray<string>; cwd: string; environment: Record<string, string>; check: () => Promise<Check>; onEnd?: () => void }
type Session = { id: string; accountId: string; connectionId: string; child: IPty; queue: SignInItem[]; bytes: number; wake?: (() => void) | undefined; timer: ReturnType<typeof setTimeout>; grace?: ReturnType<typeof setTimeout> | undefined; processExited: boolean; ending: boolean; done: boolean; attached: boolean; check: Start['check']; onEnd: Start['onEnd']; exited: Promise<void> }
// Ephemeral only. No logger, event-store, board or alert dependency is available here.
export class SignInSessions {
  private sessions = new Map<string, Session>()
  private closed = false
  private options: { graceMs?: number; limitMs?: number }
  constructor(options: { graceMs?: number; limitMs?: number } = {}) { this.options = options }
  async start(input: Start) {
    let spawn: typeof import('@lydell/node-pty')['spawn']
    try { ({ spawn } = await import('@lydell/node-pty')) } catch { throw new Error(signInReasons.failed) }
    // Recheck after loading, so concurrent starts cannot exceed either limit.
    if (this.closed) throw new Error(signInReasons.ended)
    if ([...this.sessions.values()].some(s => s.accountId === input.accountId && !s.done)) throw new Error(signInReasons.busy)
    if (this.sessions.size >= 16) throw new Error(signInReasons.busy)
    let child: IPty
    try { child = spawn(input.executable, [...input.args], { name: 'xterm-256color', cols: 80, rows: 24, cwd: input.cwd, env: input.environment, encoding: 'utf8' }) } catch { throw new Error(signInReasons.failed) }
    let resolveExit!: () => void
    const exited = new Promise<void>(resolve => { resolveExit = resolve })
    const id = randomUUID()
    const session: Session = { id, accountId: input.accountId, connectionId: input.connectionId, child, queue: [], bytes: 0, processExited: false, ending: false, done: false, attached: false, check: input.check, onEnd: input.onEnd, exited,
      timer: setTimeout(() => this.finish(session, 'The sign-in reached its time limit.'), this.options.limitMs ?? 5 * 60_000) }
    session.timer.unref()
    this.sessions.set(id, session)
    child.onData(text => { if (session.ending) return; if (session.bytes + Buffer.byteLength(text) > 64 * 1024) this.finish(session, 'The sign-in produced too much unread output.'); else { session.bytes += Buffer.byteLength(text); session.queue.push({ _tag: 'output', text }); session.wake?.() } })
    child.onExit(() => { session.processExited = true; resolveExit(); this.finish(session, 'The sign-in program exited.') })
    // A start whose caller never attaches cannot leave a process behind.
    session.grace = setTimeout(() => this.finish(session, 'The terminal disconnected.'), this.options.graceMs ?? 5000)
    session.grace.unref()
    return { id, pid: child.pid }
  }
  private get(id: string, connection: string) { const s = this.sessions.get(id); if (!s) throw new Error(signInReasons.missing); if (s.connectionId !== connection) throw new Error(signInReasons.connection); return s }
  private finish(s: Session, reason: string) {
    if (s.ending) return
    s.ending = true; clearTimeout(s.timer); clearTimeout(s.grace); s.onEnd?.()
    // Signal only while the PTY child is alive; an exited PID may be reused.
    if (!s.processExited) {
      try { process.kill(-s.child.pid, 'SIGKILL') } catch {}
      try { s.child.kill('SIGKILL') } catch {}
    }
    void s.exited.then(() => s.check()).catch(() => ({ state: 'unavailable' as const })).then(check => { s.queue.push({ _tag: 'ended', reason, check }); s.done = true; s.wake?.(); s.grace = setTimeout(() => this.sessions.delete(s.id), 5000); s.grace.unref() })
  }
  write(id: string, connection: string, text: string) { const s = this.get(id, connection); if (s.ending) throw new Error(signInReasons.ended); if (Buffer.byteLength(text) > 4096) throw new Error(signInReasons.bounds); s.child.write(text) }
  resize(id: string, connection: string, cols: number, rows: number) { const s = this.get(id, connection); if (s.ending) throw new Error(signInReasons.ended); if (!Number.isInteger(cols) || !Number.isInteger(rows) || cols < 20 || cols > 240 || rows < 5 || rows > 100) throw new Error(signInReasons.bounds); s.child.resize(cols, rows) }
  stop(id: string, connection: string) { this.finish(this.get(id, connection), 'The owner stopped the sign-in.') }
  detach(id: string, connection: string) { const s = this.sessions.get(id); if (s && s.connectionId === connection && !s.ending && !s.grace) { s.grace = setTimeout(() => this.finish(s, 'The terminal disconnected.'), this.options.graceMs ?? 5000); s.grace.unref() } }
  disconnect(connection: string) { for (const s of this.sessions.values()) this.detach(s.id, connection) }
  async *output(id: string, connection: string): AsyncGenerator<SignInItem> {
    const s = this.get(id, connection)
    if (s.attached) throw new Error(signInReasons.stream)
    s.attached = true; clearTimeout(s.grace); s.grace = undefined
    try { for (;;) { if (!s.queue.length) { if (s.done) return; await new Promise<void>(resolve => { s.wake = resolve }); s.wake = undefined; continue }
      const item = s.queue.shift()!; if (item._tag === 'output') s.bytes -= Buffer.byteLength(item.text); yield item; if (item._tag === 'ended') return
    } } finally { s.queue = []; s.bytes = 0; if (s.done) this.sessions.delete(id); else this.detach(id, connection) }
  }
  close() { this.closed = true; for (const s of this.sessions.values()) this.finish(s, 'The service stopped.'); this.sessions.clear() }
}
