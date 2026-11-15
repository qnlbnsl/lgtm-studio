import type { Event, RequestId, QuestionId } from '@lgtm/contracts'
import * as Effect from 'effect/Effect'
import type { Input } from '../commands/internal.ts'
import { answerQuestion } from '../routing/questions.ts'
import { requestedPermissions } from './approvals.ts'

export const permissionSettingClosedNote = 'Permission request closed because the owner changed its setting.'
export const permissionWithdrawnNote = 'Permission request closed because the owner withdrew its grant.'

// The handler calls this only after validating the owner-only project batch.
// Reuse ordinary question closure, answer delivery and queued-delivery withdrawal.
export const closeChangedPermissionQuestions = (input: Input, events: ReadonlyArray<Event>, requestId: RequestId) => Effect.gen(function* () {
  const changes = events.filter((event) => event._tag === 'project.PermissionChanged' || event._tag === 'project.PermissionWithdrawn')
  const closed: Event[] = []
  for (const row of Object.values(input.snapshot.questions)) {
    if (row.status !== 'open' || row.projectApproval === undefined) continue
    const permissions = requestedPermissions(row.projectApproval)
    const change = changes.find((event) => event.projectId === row.projectId && permissions.includes(event.permission))
    if (change === undefined) continue
    const plan = yield* answerQuestion(input, { requestId, kind: 'question', id: row.questionId as QuestionId,
      text: change._tag === 'project.PermissionChanged' ? permissionSettingClosedNote : permissionWithdrawnNote })
    closed.push(...plan.events)
  }
  return closed
})
