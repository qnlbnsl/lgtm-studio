import { readdirSync, readFileSync } from 'node:fs'
import type { SignInOutput } from '@lgtm/contracts'
import { randomUUID } from 'node:crypto'
import type { IPty } from '@lydell/node-pty'
import { terminalReasons } from './terminalReasons.ts'
import { signInReasons } from './signinReasons.ts'
export type SignInItem = SignInOutput
type Check = Extract<SignInOutput, { _tag: 'ended' }>['check']
type Start = { kind?: 'project'; cols?: number; rows?: number; allowed?: () => boolean; accountId: string; connectionId: string; executable: string; args: ReadonlyArray<string>; cwd: string; environment: Record<string, string>; check: () => Promise<Check>; onEnd?: () => void }
type Session = { id: string; accountId: string; connectionId: string; child: IPty; queue: SignInItem[]; bytes: number; paused?: boolean; wake?: (() => void) | undefined; kind?: 'project'; timer?: ReturnType<typeof setTimeout>; grace?: ReturnType<typeof setTimeout> | undefined; processExited: boolean; ending: boolean; done: boolean; attached: boolean; check: Start['check']; onEnd: Start['onEnd']; exited: Promise<void> }
// Ephemeral only. No logger, event-store, board or alert dependency is available here.
export class SignInSessions {
  private sessions = new Map<string, Session>()
  private closed = false
  terminalEnabled = false
  terminalGeneration = 0
  private options: { graceMs?: number; limitMs?: number }
  constructor(options: { graceMs?: number; limitMs?: number } = {}) { this.options = options }
  async start(input: Start) {
    const reasons = input.kind === 'project' ? terminalReasons : signInReasons
    let spawn: typeof import('@lydell/node-pty')['spawn']
    try { ({ spawn } = await import('@lydell/node-pty')) } catch { throw new Error(reasons.failed) }
    // Recheck after loading, so concurrent starts cannot exceed either limit.
    if (this.closed) throw new Error(reasons.ended)
    if (input.kind === 'project' && (!this.terminalEnabled || input.allowed?.() === false)) throw new Error(terminalReasons.off)
    if (input.kind === 'project' && [...this.sessions.values()].filter(s => s.kind === 'project' && !s.ending).length >= 8) throw new Error(terminalReasons.busy)
    if (input.kind !== 'project' && [...this.sessions.values()].some(s => s.kind !== 'project' && s.accountId === input.accountId && !s.done)) throw new Error(signInReasons.busy)
    if (input.kind !== 'project' && [...this.sessions.values()].filter(s => s.kind !== 'project').length >= 16) throw new Error(signInReasons.busy)
    let child: IPty
    try { child = spawn(input.executable, [...input.args], { name: 'xterm-256color', cols: input.cols ?? 80, rows: input.rows ?? 24, cwd: input.cwd, env: input.environment, encoding: 'utf8' }) } catch { throw new Error(reasons.failed) }
    let resolveExit!: () => void
    const exited = new Promise<void>(resolve => { resolveExit = resolve })
    const id = randomUUID()
    const session: Session = { id, ...(input.kind ? { kind: input.kind } : {}), accountId: input.accountId, connectionId: input.connectionId, child, queue: [], bytes: 0, processExited: false, ending: false, done: false, attached: false, check: input.check, onEnd: input.onEnd, exited,
      ...(input.kind === 'project' ? {} : { timer: setTimeout(() => this.finish(session, 'The sign-in reached its time limit.'), this.options.limitMs ?? 5 * 60_000) }) }
    session.timer?.unref()
    this.sessions.set(id, session)
    child.onData(text => {
      if (session.ending) return
      if (session.kind !== 'project' && session.bytes + Buffer.byteLength(text) > 64 * 1024) {
        this.finish(session, 'The sign-in produced too much unread output.'); return
      }
      session.bytes += Buffer.byteLength(text); session.queue.push({ _tag: 'output', text })
      // Stop reading the PTY so the kernel applies backpressure to the shell.
      // The queue can exceed the high mark by at most the current native chunk.
      if (session.kind === 'project' && !session.paused && session.bytes >= 128 * 1024) { session.paused = true; child.pause() }
      session.wake?.()
    })
    child.onExit(() => { session.processExited = true; if (session.kind === 'project') this.stopSessionMembers(session); resolveExit(); this.finish(session, session.kind === 'project' ? 'The shell exited.' : 'The sign-in program exited.') })
    // A start whose caller never attaches cannot leave a process behind.
    session.grace = setTimeout(() => this.finish(session, 'The terminal disconnected.'), this.options.graceMs ?? 5000)
    session.grace.unref()
    return { id, pid: child.pid }
  }
  private get(id: string, connection: string, kind?: 'project') { const s = this.sessions.get(id); const reasons = kind === 'project' ? terminalReasons : signInReasons; if (!s || s.kind !== kind) throw new Error(reasons.missing); if (s.connectionId !== connection) throw new Error(reasons.connection); return s }
  private stopSessionMembers(s: Session) {
    try { for (const entry of readdirSync('/proc')) {
      if (!/^\d+$/.test(entry)) continue
      try {
        const stat = readFileSync('/proc/' + entry + '/stat', 'utf8')
        const fields = stat.slice(stat.lastIndexOf(')') + 2).split(' ')
        if (Number(fields[3]) !== s.child.pid) continue
        const pid = Number(entry)
        if (s.processExited && pid === s.child.pid) continue
        // Signal the observed member, not a process-group identifier that may
        // have been reused after the leader exited. Recheck membership first.
        const again = readFileSync('/proc/' + entry + '/stat', 'utf8')
        if (Number(again.slice(again.lastIndexOf(')') + 2).split(' ')[3]) === s.child.pid) process.kill(pid, 'SIGKILL')
      } catch {}
    } } catch {} // No /proc means there is no descendant discovery on this host.
  }
  private finish(s: Session, reason: string) {
    if (s.ending) return
    s.ending = true; clearTimeout(s.timer); clearTimeout(s.grace); s.onEnd?.()
    // /proc session membership survives the leader's exit. Never signal the
    // former leader PID or its former process group after its exit callback.
    if (s.kind === 'project') this.stopSessionMembers(s)
    if (!s.processExited) {
      try { process.kill(-s.child.pid, 'SIGKILL') } catch {}
      try { s.child.kill('SIGKILL') } catch {}
    }
    void s.exited.then(() => s.check()).catch(() => ({ state: 'unavailable' as const })).then(check => { s.queue.push({ _tag: 'ended', reason, check }); s.done = true; s.wake?.(); s.grace = setTimeout(() => this.sessions.delete(s.id), 5000); s.grace.unref() })
  }
  write(id: string, connection: string, text: string, kind?: 'project') { const s = this.get(id, connection, kind); if (s.ending) throw new Error(kind ? terminalReasons.ended : signInReasons.ended); if (Buffer.byteLength(text) > 4096) throw new Error(kind ? terminalReasons.bounds : signInReasons.bounds); s.child.write(text) }
  resize(id: string, connection: string, cols: number, rows: number, kind?: 'project') { const s = this.get(id, connection, kind); if (s.ending) throw new Error(kind ? terminalReasons.ended : signInReasons.ended); if (!Number.isInteger(cols) || !Number.isInteger(rows) || cols < 20 || cols > 240 || rows < 5 || rows > 100) throw new Error(kind ? terminalReasons.bounds : signInReasons.bounds); s.child.resize(cols, rows) }
  stop(id: string, connection: string, kind?: 'project') { this.finish(this.get(id, connection, kind), kind ? 'The owner closed the terminal.' : 'The owner stopped the sign-in.') }
  stopTerminals(reason: string) { this.terminalGeneration++; this.terminalEnabled = false; for (const s of this.sessions.values()) if (s.kind === 'project') this.finish(s, reason) }
  detach(id: string, connection: string) { const s = this.sessions.get(id); if (s && s.connectionId === connection && !s.ending && !s.grace) { s.grace = setTimeout(() => this.finish(s, 'The terminal disconnected.'), this.options.graceMs ?? 5000); s.grace.unref() } }
  disconnect(connection: string) { for (const s of this.sessions.values()) this.detach(s.id, connection) }
  async *output(id: string, connection: string, kind?: 'project'): AsyncGenerator<SignInItem> {
    const s = this.get(id, connection, kind)
    if (s.attached) throw new Error(kind ? terminalReasons.stream : signInReasons.stream)
    s.attached = true; clearTimeout(s.grace); s.grace = undefined
    try { for (;;) { if (!s.queue.length) { if (s.done) return; await new Promise<void>(resolve => { s.wake = resolve }); s.wake = undefined; continue }
      const item = s.queue.shift()!; if (item._tag === 'output') { s.bytes -= Buffer.byteLength(item.text); if (s.paused && !s.ending && s.bytes <= 32 * 1024) { s.paused = false; s.child.resume() } } yield item; if (item._tag === 'ended') return
    } } finally { s.queue = []; s.bytes = 0; if (s.done) this.sessions.delete(id); else this.detach(id, connection) }
  }
  close() { this.closed = true; for (const s of this.sessions.values()) this.finish(s, 'The service stopped.'); this.sessions.clear() }
}
