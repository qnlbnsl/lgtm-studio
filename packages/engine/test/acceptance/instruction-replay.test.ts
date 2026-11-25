import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { afterEach, expect } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { eventsUntil, isTag, peekEcho, promptsOf, requestId, scenario, since, stopAllServices, tagged, waitFor } from '../support/acceptance.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { withRoleInstructions } from '../support/instructionPrompt.ts'

registerRebuildCheck({ noLog: true })
afterEach(async () => { await stopAllServices() })
for (const accepted of [false, true]) scenario(`s22 r2: owner first prompt crash ${accepted ? 'after' : 'before'} acceptance preserves base replay rules even with saved engine instructions`, (w) => Effect.gen(function* () {
  const gate = join(w.root.root, 'first-turn-gate')
  const worker = yield* w.start(w.owner, { role: 'worker:builder', prompt: 'Owner first work.', script: { turns: [[
    ...(accepted ? [{ step: 'plan', entries: [] }] : []), { step: 'wait_for_file', path: gate },
  ]] } })
  const initial = yield* eventsUntil(w.owner, 0, 'the first prompt to reach the chosen crash point', isTag(accepted ? 'DeliveryAccepted' : 'DeliverySent', worker.agentId))
  const sent = tagged(initial, 'DeliverySent', worker.agentId)[0]!.event as {deliveryId: string}
  const before = yield* w.token(worker)
  const received = yield* waitFor('the recorded first prompt', () => promptsOf(peekEcho(worker.echo)!)[0])
  const listing = yield* w.owner.accounts({ requestId: requestId(), operation: 'list' } as never)
  const account = listing.accounts.find((row) => row.id === worker.accountId)!
  const script = JSON.parse(account.environment!.FAKE_AGENT_SCRIPT!)
  yield* w.owner.accounts({ requestId: requestId(), operation: 'set', account: { ...account, environment: {
    ...account.environment, FAKE_AGENT_SCRIPT: JSON.stringify({ ...script, history: [{ role: 'user', text: received }] }),
  } } } as never)
  const mark = yield* w.watermark()
  yield* w.kill('SIGKILL'); yield* w.boot(); yield* w.token(worker, before.pid)
  yield* waitFor('the first post-restart prompt', () => promptsOf(peekEcho(worker.echo)!).length > 0 ? true : undefined)
  writeFileSync(gate, 'finish')
  yield* eventsUntil(w.owner, mark, 'the resumed turn to end', isTag('TurnEnded', worker.agentId))
  const rows = yield* since(w.owner, mark)
  const same = (tag: string) => tagged(rows, tag, worker.agentId).filter((row) => (row.event as {deliveryId?: string}).deliveryId === sent.deliveryId)
  expect(same('DeliveryRequeued')).toHaveLength(accepted ? 0 : 1)
  expect(same('DeliverySent')).toHaveLength(accepted ? 0 : 1)
  const carries = tagged(rows, 'DeliveryQueued').filter((row) => {
    const delivery = (row.event as {delivery:{target:string;content:{_tag:string}}}).delivery
    return delivery.target === worker.agentId && delivery.content._tag === 'carry-on'
  })
  expect(carries).toHaveLength(accepted ? 1 : 0)
  const prompts = promptsOf(peekEcho(worker.echo)!)
  expect(prompts).toHaveLength(1)
  // Review round 1 of unit f2-lifecycle, item 6: the accepted first turn never
  // succeeded, so by the one rule the runner and the log share
  // (src/roles/instructionObligation.ts) the instructions are still owed and
  // the carry-on carries them, as a retry after an in-memory cancel does.
  if (accepted) {
    const carry = (carries[0]!.event as {delivery:{id:string;content:{text:string}}}).delivery
    expect(carry.content.text).toMatch(/Carry on/)
    expect(prompts[0]).toBe(withRoleInstructions(prompts[0]!, carry.id, carry.content.text, ['worker-base', 'builder']))
  }
  // Issue 1235: the unaccepted first turn still owes the role instructions, so its retry carries them again.
  else expect(prompts[0]).toBe(withRoleInstructions(prompts[0]!, sent.deliveryId, 'Owner first work.', ['worker-base', 'builder']))
  expect(JSON.stringify(rows)).not.toContain('# Rules for every worker')
}))
