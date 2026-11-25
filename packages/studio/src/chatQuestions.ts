import type { EventEnvelope } from '@lgtm/contracts'

type Question = Extract<EventEnvelope['event'], { _tag: 'QuestionAsked' }>
type Asked = EventEnvelope & { event: Question }
const normalize = (value: string) => value.trim().replace(/^#{1,6}\s+/, '').replace(/\*\*(.*?)\*\*/g, '$1').replace(/\s+/g, ' ')

// The lead sometimes announces an ask in its own reply despite the tool's
// instruction. Only a title at the start of a same-turn message identifies it.
export function questionReply(text: string, questions: ReadonlyArray<Question>): string {
  const paragraphs = text.trim().split(/\r?\n\s*\r?\n/)
  const question = questions.find(q => q.title && normalize(paragraphs[0]!) === normalize(q.title))
  if (!question) return text
  let end = 1
  // A recommendation is kept if the authoritative card has none to show.
  if (question.recommendation && /^Recommendation:\s*/.test(normalize(paragraphs[end] ?? ''))) end++
  // This is the lead's own sign-off, not engine delivery text. Ordinary
  // messages and sign-offs separated from the announcement remain untouched.
  if ((paragraphs[end] ?? '').trim() === 'Sent through LGTM Studio.') end++
  return paragraphs.slice(end).join('\n\n')
}

// Each sequence points at its turn's shared array. Finishing the pass makes
// questions available to text chunks anywhere in that turn, including history.
export function questionTurns(items: ReadonlyArray<EventEnvelope>, agentId: string): Map<number, Asked[]> {
  const turns = new Map<number, Asked[]>()
  let questions: Asked[] = []
  for (const item of items) {
    const e = item.event
    if (!('agentId' in e) || e.agentId !== agentId) continue
    turns.set(item.sequence, questions)
    if (e._tag === 'QuestionAsked') questions.push(item as Asked)
    if (['TurnEnded', 'SessionOpened', 'AgentStopped', 'AgentInterrupted', 'AgentCrashed', 'AgentStarted'].includes(e._tag)) questions = []
  }
  return turns
}

export function askHolder(input: string | undefined, sequence: number, questions: ReadonlyArray<Asked>): 'owner' | 'agent' | undefined {
  let arguments_: Record<string, unknown> = {}
  try {
    const value = JSON.parse(input ?? '{}')
    if (value && typeof value === 'object') arguments_ = value.arguments ?? value
  } catch { /* Missing or partial adapter input leaves the title generic. */ }
  const text = typeof arguments_?.text === 'string' ? arguments_.text : undefined
  const title = typeof arguments_?.title === 'string' ? arguments_.title : undefined
  const match = text || title ? questions.find(q => (!text || q.event.text === text) && (!title || q.event.title === title))
    : questions.find(q => q.sequence >= sequence)
  return match?.event.holder._tag
}
