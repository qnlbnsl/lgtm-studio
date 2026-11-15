import type { EventEnvelope } from '@lgtm/contracts'
import { toolText } from '@lgtm/contracts/thread'
import { engineToolTitle } from './engineToolTitles.ts'

type Content = Extract<EventEnvelope['event'], { _tag: 'ThreadItem' }>['content']
export type ToolDetail = { label: string; text: string; kind?: 'diff'; note?: string; copyLabel?: string }
export type ToolRow = { id: string; ts: number; type: 'tools'; summary: string; raw: string; status: string; details: ToolDetail[] }
const bidi = /[\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/g
const label = (text: string) => text.replace(bidi, '').replace(/[\u0000-\u001f\u007f-\u009f\u2028\u2029]/g, ' ').replace(/\s+/g, ' ').trim()
const fileName = (path: string) => label(pathText(path).split(/[\\/]/).at(-1) || pathText(path))
const firstLine = (text: string) => text.split('\n').find((line) => line.trim())?.trim() ?? ''
const unquote = (text: string) => text.trim().replace(/^`+|`+$/g, '')
const pathText = (text: string) => unquote(text.replace(/\s*\(\d+\s*-\s*\d+\)\s*$/, '').replace(/^file\s+/i, '')).replace(/^["']+|["']+$/g, '')
// Written from the display rule: only these exact executable spellings may
// lose their wrapper. This does not resolve a path or evaluate shell text.
const shellExecutables = ['', '/bin/', '/usr/bin/', '/usr/local/bin/'].flatMap(folder => ['sh', 'bash', 'zsh'].map(shell => folder + shell))
function shellCommand(command: string): string | undefined {
  const prefix = shellExecutables.flatMap(shell => [`${shell} -c `, `${shell} -lc `]).find(value => command.startsWith(value))
  if (!prefix) return undefined
  const argument = command.slice(prefix.length), quote = argument[0]
  if (quote !== "'" && quote !== '"') return undefined
  let inner = ''
  for (let i = 1; i < argument.length; i++) {
    const character = argument[i]!
    if (character === quote) return i === argument.length - 1 && inner.trim() ? inner : undefined
    if (quote === '"' && character === '\\') {
      const next = argument[++i]
      if (next === undefined) return undefined
      inner += ['"', '\\', '$', '`'].includes(next) ? next : '\\' + next
    } else inner += character
  }
  return undefined
}
export function commandTitle(command: string): string {
  const inner = shellCommand(command.trim()) ?? command
  const lines = inner.trim().split('\n')
  return lines.length > 1 ? firstLine(inner).replace(/\\\s*$/, '').trimEnd() + '…' : firstLine(inner)
}
const object = (text?: string): Record<string, unknown> => {
  try { const value = JSON.parse(text ?? 'null'); return value && typeof value === 'object' && !Array.isArray(value) ? value : {} } catch { return {} }
}
const field = (value: Record<string, unknown>, ...keys: string[]) => keys.map((key) => value[key]).find((v): v is string => typeof v === 'string')
const verbs: Record<string, string> = { read: 'Read', edit: 'Edited', write: 'Wrote', create: 'Created', delete: 'Deleted', remove: 'Removed', search: 'Searched', grep: 'Searched', glob: 'Searched', list: 'Listed', get: 'Read', set: 'Updated', update: 'Updated', post: 'Posted', ask: 'Asked', orient: 'Read', run: 'Ran', execute: 'Ran', bash: 'Ran', shell: 'Ran', exec_command: 'Ran', apply_patch: 'Edited' }

export function toolRow(base: { id: string; ts: number }, content: Extract<Content, { _tag: 'tool-call' }>, files: ReadonlyMap<string, string> = new Map(), holder?: 'owner' | 'agent'): ToolRow {
  const input = object(content.input)
  const title = label(unquote(firstLine(content.title)))
  const commandValue = input.command ?? input.cmd
  const command = typeof commandValue === 'string' ? commandValue : Array.isArray(commandValue) && commandValue.every((part) => typeof part === 'string') ? commandValue.join(' ') : undefined
  const path = files.keys().next().value ?? field(input, 'file_path', 'path', 'filePath')
  const token = title.toLowerCase().replace(/\(.*$/, '').trim()
  const named = /^(read|edit|write|search|grep|glob|bash|shell|run|execute)\b[: ]*([\s\S]*)/i.exec(title)
  const namedPath = named?.[2] ? pathText(named[2]) : ''
  let summary: string
  if (command) summary = `Ran ${commandTitle(command)}`
  else if (files.size) summary = `${/write|create/i.test(title) ? 'Wrote' : 'Edited'} ${fileName(path ?? '')}${files.size > 1 ? ` and ${files.size - 1} more ${files.size === 2 ? 'file' : 'files'}` : ''}`
  else if (engineToolTitle(title)) summary = engineToolTitle(title, holder)!
  else if (/^List files(?: in )?/i.test(title)) summary = title.replace(/^List/i, 'Listed').replace(/["'`]/g, '')
  else if (/^(?:\S*\/)?(?:sh|bash|zsh) -(?:lc|c) /.test(title)) summary = `Ran ${commandTitle(unquote(content.title))}`
  else if (/^Search(?: for | in |$)/i.test(title)) summary = title.replace(/^Search/i, 'Searched')
    .replace(/^(Searched for |Searched in )(['"])(.*)\2(?= in |$)/, '$1$3').replace(/( in )(['"])(.*)\2$/, '$1$3')
  else if (/^mcp(?:__|\.)/.test(title)) {
    const name = title.startsWith('mcp__') ? title.split('__').slice(2).join('_') : title.split('.').slice(2).join('_')
    const words = name.split(/[._-]+/)
    const action = words.pop()!
    let target = words.filter((word) => words.length === 1 || !['project', 'roles'].includes(word)).join(' ')
    if (action === 'list' && target && !target.endsWith('s')) target += 's'
    summary = action === 'ask' && !target ? 'Asked a question' : verbs[action] ? `${verbs[action]} ${target || (action === 'orient' ? 'role guidance' : action)}` : `Used ${[...words, action].join(' ')}`
  } else if (named || verbs[token]) {
    const verb = verbs[named?.[1]?.toLowerCase() ?? token] ?? 'Used'
    const target = path ? fileName(path) : field(input, 'pattern', 'query') ?? (['Read', 'Edited', 'Wrote'].includes(verb) ? fileName(namedPath) : named?.[2]) ?? ''
    summary = `${verb} ${target || (verb === 'Ran' ? 'command' : verb === 'Searched' ? 'files' : 'file')}`
  } else if (/^(?:pnpm|npm|git|node|python3?|rg|ls|cat|cd)\s/.test(title)) summary = `Ran ${commandTitle(unquote(content.title))}`
  else if (path) summary = `Used ${fileName(path)}`
  else summary = title || 'Used tool'

  const details: ToolDetail[] = []
  if (files.size) {
    const patches = new Map<string, string[]>()
    for (const [file, diff] of files) patches.set(diff, [...(patches.get(diff) ?? []), file])
    for (const [diff, paths] of patches) {
      const isDiff = /^@@(?:\s|$)/m.test(diff)
      details.push({ label: paths.map(label).join(', '), text: diff,
        ...(isDiff ? { kind: 'diff' as const, copyLabel: 'diff' } : { note: 'The earlier text was not recorded.', copyLabel: 'content' }) })
    }
  } else if (content.input) details.push({ label: command ? 'Command' : 'Input', text: command ?? toolText(content.input) })
  if (content.output && !files.size) {
    const output = toolText(content.output)
    if (output && !details.some((detail) => detail.text === output)) details.push({ label: 'Output', text: output })
  }
  if (!content.input && !files.size && namedPath && ['read', 'edit', 'write'].includes(named![1]!.toLowerCase())) details.unshift({ label: 'Path', text: namedPath, copyLabel: 'path' })
  // Keep the complete command, or the uncapped title, in the opened detail.
  if (!content.input && !files.size && summary.startsWith('Ran ') && summary !== 'Ran command') details.unshift({ label: 'Command', text: unquote(content.title) })
  const cleanSummary = label(summary)
  if ((cleanSummary.length > 120 || content.title.includes('\n') && !summary.startsWith('Ran ')) && !details.some((detail) => detail.text === content.title)) details.push({ label: 'Full title', text: content.title.replace(bidi, '') })
  return { ...base, type: 'tools', summary: cleanSummary.length > 120 ? cleanSummary.slice(0, 119) + '…' : cleanSummary,
    raw: /^mcp(?:__|\.)/.test(title) || verbs[token] ? title : named?.[1] ?? (command || summary.startsWith('Ran ') ? 'Command' : ''), status: content.status ?? '', details }
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
    if (e._tag === 'ThreadItem' || ['QuestionAsked', 'PermissionRequested', 'PromptSent', 'DeliverySent', 'SessionOpened', 'TurnEnded', 'AgentStopped', 'AgentInterrupted', 'AgentCrashed', 'AgentStarted'].includes(e._tag)) segments.set(e.agentId, segment + 1)
    return item
  })
}
