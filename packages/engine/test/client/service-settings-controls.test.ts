import { mkdtempSync, rmSync } from 'node:fs'
import * as Effect from 'effect/Effect'
import { expect, it } from 'vitest'
import { call, ownerCredential, success } from '../support/handlerRig.ts'
import { run, read } from './support/readRig.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'

// Each retained write control must reach a durable engine setting.
for (const [label, patch] of [
  ['Stall watch', { stallWatch: false }],
  ['Assigned, not started', { assignedStallMinutes: 7 }],
  ['Worker, no output', { limits: { stallMinutes: 17 } }],
  ['Lead, no output', { leadStallMinutes: 29 }],
  ['Nudge a stalled agent automatically', { autoNudge: false }],
  ['Automatic nudges', { nudgeAttempts: 1 }],
  ['Replace the worker', { replaceAfter: 4 }],
  ['Wait after a usage limit', { limits: { capacityBackoffMinutes: 11 } }],
  ['Try again', { limits: { capacityRetries: 3 } }],
  ['Restart after a crash', { limits: { crashRetries: 0 } }],
  ['Close an idle team', { limits: { teamIdleMinutes: 35 } }],
] as const) {
  it(`Service settings: ${label} changes engine state through roles.settings.set`, async () => run(Effect.gen(function* () {
    const before = yield* read('roles.settings.get')
    const settings = { ...before.settings, ...patch, ...('limits' in patch ? { limits: { ...before.settings.limits, ...patch.limits } } : {}) }
    success(yield* call(ownerCredential, 'roles.settings.set', { settings }))
    const after = yield* read('roles.settings.get')
    expect(after.settings).toMatchObject(patch)
    expect(after.settings).not.toEqual(before.settings)
  }), registerRebuildCheck()))
}
for (const [label, lead] of [
  ['Harness', { harness: 'first', account: { _tag: 'any' }, permission: 'ask' }],
  ['Model', { harness: 'first', account: { _tag: 'any' }, permission: 'ask', model: 'test-model' }],
  ['Effort', { harness: 'first', account: { _tag: 'any' }, permission: 'ask', effort: 'high' }],
  ['Permission', { harness: 'first', account: { _tag: 'any' }, permission: 'read-only' }],
] as const) {
  it(`Service settings: ${label} changes engine state through project.settings.set`, async () => run(Effect.gen(function* () {
    const before = yield* read('project.settings')
    const settings = { workspaceFolder: '/tmp', shell: '/bin/sh', lead }
    success(yield* call(ownerCredential, 'project.settings.set', { settings }))
    expect((yield* read('project.settings')).settings).toMatchObject(settings)
    expect((yield* read('project.settings')).settings).not.toEqual(before.settings)
  }), registerRebuildCheck()))
}
for (const [label, patch] of [
  
  ['Default reviewers', { reviewDefaults: { reviewers: 4, differentVendor: true } }],
  ['Reviewer from another vendor', { reviewDefaults: { reviewers: 2, differentVendor: false } }],
] as const) {
  it(`Service settings: ${label} changes engine state through project.settings.set`, async () => run(Effect.gen(function* () {
    const before = yield* read('project.settings')
    const settings = { workspaceFolder: '/tmp', shell: '/bin/sh', ...patch }
    success(yield* call(ownerCredential, 'project.settings.set', { settings }))
    expect((yield* read('project.settings')).settings).toMatchObject(patch)
    expect((yield* read('project.settings')).settings).not.toEqual(before.settings)
  }), registerRebuildCheck()))
}

it('Service settings: Your overrides and Use the default folder change the engine workflow folder', async () => {
  const folder = mkdtempSync('/tmp/s20-workflows-')
  try {
    await run(Effect.gen(function* () {
      const settings = { workspaceFolder: '/tmp', shell: '/bin/sh', workflowFolder: folder }
      success(yield* call(ownerCredential, 'project.settings.set', { settings }))
      expect((yield* read('project.settings')).settings.workflowFolder).toBe(folder)
      success(yield* call(ownerCredential, 'project.settings.set', { settings: { workspaceFolder: '/tmp', shell: '/bin/sh' }, clearWorkflowFolder: true }))
      expect((yield* read('project.settings')).settings.workflowFolder).toBeUndefined()
    }), registerRebuildCheck())
  } finally { rmSync(folder, { recursive: true, force: true }) }
})

for (const [label, patch] of [
  ['Project terminal', { terminalEnabled: true }],
  ['Web page', { enabled: false }],
  ['Port', { port: 24681 }],
  ['Check-in', { heartbeatSeconds: 37 }],
  ['Behind your HTTPS proxy', { proxyMode: true, publicAddress: 'https://s20.example.test' }],
  ['Public address', { publicAddress: 'https://s20.example.test' }],
] as const) {
  it(`Service settings: ${label} changes engine state through client.web.settings.set`, async () => run(Effect.gen(function* () {
    const { requestId, serverState, ...before } = yield* read('client.web.settings')
    success(yield* call(ownerCredential, 'client.web.settings.set', { settings: { ...before, ...patch } }))
    const { requestId: afterId, serverState: afterState, ...after } = yield* read('client.web.settings')
    expect(after).toMatchObject(patch)
    expect(after).not.toEqual(before)
  }), registerRebuildCheck()))
}
