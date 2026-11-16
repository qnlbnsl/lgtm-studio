import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'
import { recoverySource } from '../scripts/recoverySource.ts'
import { heldPermissionSource } from '../scripts/heldPermissionSource.ts'
import { alertLinksSource } from '../scripts/alertLinksSource.ts'

for (const [name, rewrite, path, marker] of [
  ['crash status', recoverySource, 'js/data.js', "if (a.crashed) return { dot: 'failed', word: a.activity || 'Crashed', tone: 'error' };"],
  ['crash composer', recoverySource, 'js/ui.js', "a.crashed ? a.name + ': ' + a.activity : a.name + ' is stopped.'"],
  ['held permission index', heldPermissionSource, 'js/data.js', "live.on && r.kind === 'approval' && r.canAnswer"],
  ['crash Messages lane', recoverySource, 'js/screens/messages.js', "if (a.crashed) return { lane: 'needs', why: a.activity || 'Crashed', tone: 'error' };"],
  ['crash Workflow waiting group', recoverySource, 'js/screens/workflow.js', "a.crashed || a.state === 'waiting' || a.state === 'failed'"],
  ['crash Workflow idle group', recoverySource, 'js/screens/workflow.js', "!a.crashed && (a.state === 'idle' || a.state === 'stopped')"],
  ['notification login', alertLinksSource, 'js/screens/auth.js', "live.on ? alertsLive.afterSignIn(s.session.firstRun) : s.session.firstRun ? '/setup' : '/home'"],
] as const) {
  const source = readFileSync(`${process.cwd()}/design/mock/${path}`, 'utf8')
  it(`s21 ${name} guard accepts exactly one anchor`, () => { expect(() => rewrite(path, source)).not.toThrow() })
  it(`s21 ${name} guard rejects a missing anchor`, () => { expect(() => rewrite(path, source.replace(marker, ''))).toThrow() })
  it(`s21 ${name} guard rejects a duplicate anchor`, () => { expect(() => rewrite(path, source + '\n' + marker)).toThrow() })
}

for (const path of ['js/app.js', 'js/screens/home.js', 'js/ui.js']) {
  const source = readFileSync(`${process.cwd()}/design/mock/${path}`, 'utf8')
  const marker = path === 'js/ui.js' ? "liveMode.on && r.kind === 'approval' && r.canAnswer" : 'answerableRequests('
  it(`s21 ${path} held controls and audiences accept their exact anchors`, () => { expect(() => heldPermissionSource(path, source)).not.toThrow() })
  it(`s21 ${path} held controls and audiences reject one missing anchor`, () => { expect(() => heldPermissionSource(path, source.replace(marker, ''))).toThrow() })
  it(`s21 ${path} held controls and audiences reject one extra anchor`, () => { expect(() => heldPermissionSource(path, source + marker)).toThrow() })
}
