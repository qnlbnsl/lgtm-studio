import type { EventEnvelope } from '@lgtm/contracts'
import { toolText } from '@lgtm/contracts/thread'

type Content = Extract<EventEnvelope['event'], { _tag: 'ThreadItem' }>['content']
export type ToolDetail = { label: string; text: string; kind?: 'diff' }
export type ToolRow = { id: string; ts: number; type: 'tools'; summary: string; raw: string; status: string; details: ToolDetail[] }
const fileName = (path: string) => path.split(/[\\/]/).at(-1) || path
const firstLine = (text: string) => text.split('\n').find((line) => line.trim())?.trim() ?? ''
const object = (text?: string): Record<string, unknown> => {
  try { const value = JSON.parse(text ?? 'null'); return value && typeof value === 'object' && !Array.isArray(value) ? value : {} } catch { return {} }
}
const field = (value: Record<string, unknown>, ...keys: string[]) => keys.map((key) => value[key]).find((v): v is string => typeof v === 'string')
const verbs: Record<string, string> = { read: 'Read', edit: 'Edited', write: 'Wrote', create: 'Created', delete: 'Deleted', remove: 'Removed', search: 'Searched', grep: 'Searched', glob: 'Searched', list: 'Listed', get: 'Read', set: 'Updated', update: 'Updated', post: 'Posted', ask: 'Asked', orient: 'Read', run: 'Ran', execute: 'Ran', bash: 'Ran', shell: 'Ran', exec_command: 'Ran', apply_patch: 'Edited' }

export function toolRow(base: { id: string; ts: number }, content: Extract<Content, { _tag: 'tool-call' }>, files: ReadonlyMap<string, string> = new Map()): ToolRow {
  const input = object(content.input)
  const title = firstLine(content.title)
  const command = field(input, 'command', 'cmd')
  const path = files.keys().next().value ?? field(input, 'file_path', 'path', 'filePath')
  const token = title.toLowerCase().replace(/\(.*$/, '').trim()
  const named = /^(read|edit|write|search|grep|glob|bash|shell|run|execute)\b[: ]*([\s\S]*)/i.exec(title)
  let summary: string
  if (command) summary = `Ran ${firstLine(command)}`
  else if (files.size) summary = `${/write|create/i.test(title) ? 'Wrote' : 'Edited'} ${fileName(path ?? '')}${files.size > 1 ? ` and ${files.size - 1} more ${files.size === 2 ? 'file' : 'files'}` : ''}`
  else if (/^mcp__/.test(title)) {
    const words = title.split('__').at(-1)!.split('_')
    const action = words.pop()!
    const target = words.filter((word) => !['project', 'roles'].includes(word)).join(' ')
    summary = verbs[action] ? `${verbs[action]} ${target || (action === 'orient' ? 'role guidance' : action)}` : `Used ${[...words, action].join(' ')}`
  } else if (named || verbs[token]) {
    const verb = verbs[named?.[1]?.toLowerCase() ?? token] ?? 'Used'
    const target = path ? fileName(path) : field(input, 'pattern', 'query') ?? (['Read', 'Edited', 'Wrote'].includes(verb) ? fileName(named?.[2] ?? '') : named?.[2]) ?? ''
    summary = `${verb} ${target || (verb === 'Ran' ? 'command' : verb === 'Searched' ? 'files' : 'file')}`
  } else if (/^(?:pnpm|npm|git|node|python3?|rg|ls|cat|cd)\s/.test(title)) summary = `Ran ${title}`
  else if (path) summary = `Used ${fileName(path)}`
  else summary = title || 'Used tool'

  const details: ToolDetail[] = []
  if (files.size) {
    // Diff snapshots replace input old/new strings and repeated output snapshots.
    // The path is a single label; patch headers already identify files themselves.
    const patches = new Map<string, string[]>()
    for (const [file, diff] of files) patches.set(diff, [...(patches.get(diff) ?? []), file])
    for (const [diff, paths] of patches) {
      const missing = paths.filter((file) => !diff.includes(file))
      details.push({ label: missing.join(', ') || 'Diff', text: diff, kind: 'diff' })
    }
  } else if (content.input) details.push({ label: command ? 'Command' : 'Input', text: toolText(content.input) })
  if (content.output && !files.size) {
    const output = toolText(content.output)
    if (output && !details.some((detail) => detail.text === output)) details.push({ label: 'Output', text: output })
  }
  if (!content.input && !files.size && named?.[2] && ['read', 'edit', 'write'].includes(named[1]!.toLowerCase())) details.unshift({ label: 'Path', text: named[2] })
  // Titles can contain a multiline command when the harness omitted rawInput.
  if (!content.input && !files.size && content.title !== summary && (content.title !== title || /^Ran (?!command$)/.test(summary))) details.unshift({ label: 'Command', text: content.title })
  return { ...base, type: 'tools', summary, raw: /^mcp__/.test(title) || verbs[token] ? title : named?.[1] ?? (command || summary.startsWith('Ran ') ? 'Command' : ''), status: content.status ?? '', details }
}

// Text chunks belong to a contiguous message. Reusing a harness message id across
// a tool or a question must not move later words above that recorded action.
export function orderedThread(items: ReadonlyArray<EventEnvelope>): EventEnvelope[] {
  const segments = new Map<string, number>()
  const seen = new Set<number>()
  return [...items].sort((a, b) => a.sequence - b.sequence).filter((item) => {
    if (seen.has(item.sequence)) return false
    seen.add(item.sequence); return true
  }).map((item) => {
    const e = item.event
    if (!('agentId' in e)) return item
    const segment = segments.get(e.agentId) ?? 0
    if (e._tag === 'ThreadItem' && e.content._tag === 'text') return { ...item, event: { ...e, itemId: `${segment}:${e.itemId}` } }
    if (e._tag === 'ThreadItem' || ['QuestionAsked', 'PermissionRequested', 'PromptSent', 'DeliverySent', 'SessionOpened', 'TurnEnded', 'AgentStopped'].includes(e._tag)) segments.set(e.agentId, segment + 1)
    return item
  })
}
