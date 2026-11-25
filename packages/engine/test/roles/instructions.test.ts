import { execFileSync } from 'node:child_process'
import { promises as fs, linkSync, mkdirSync, mkdtempSync, readFileSync, renameSync, rmSync, symlinkSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { parseRole } from '@lgtm/contracts'
import { InstructionStore, instructionFiles, instructionLimits, instructionName } from '../../src/roles/instructions.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'

registerRebuildCheck({ noLog: true })

let root: string, home: string, folder: string
beforeEach(() => { root = mkdtempSync('/tmp/s22-instructions-'); home = join(root, 'home'); folder = join(home, '.lgtm/roles'); mkdirSync(folder, { recursive: true }) })
afterEach(() => { vi.restoreAllMocks(); rmSync(root, { recursive: true, force: true }) })
export const roleRows = [
  ['lead:project-lead', 'project-lead', ['lead-base', 'project-lead']],
  ['project-lead', 'project-lead', ['lead-base', 'project-lead']],
  ['orchestrator', 'orchestrator', ['lead-base', 'orchestrator']],
  ['builder', 'builder', ['worker-base', 'builder']],
  ['worker:builder', 'builder', ['worker-base', 'builder']],
  ['lead:builder', undefined, ['lead-base']],
  ['worker:build-lead', undefined, ['worker-base']],
  ['chef:builder', undefined, ['worker-base']],
  [':builder', undefined, ['worker-base']],
  ['worker:reviewer', 'reviewer', ['reviewer-base', 'reviewer']],
  ['reviewer', 'reviewer', ['reviewer-base', 'reviewer']],
  ['worker:test-runner', 'test-runner', ['test-runner']],
  ['worker:evaluator', 'evaluator', ['evaluator']],
  ['lead:planning-lead', 'planning-lead', ['lead-base', 'planning-lead']],
  ['lead:design-lead', 'design-lead', ['lead-base', 'design-lead']],
  ['lead:build-lead', 'build-lead', ['lead-base', 'build-lead']],
  ['worker:planner', 'planner', ['worker-base', 'planner']],
  ['worker:designer', 'designer', ['worker-base', 'designer']],
  ['worker:build-init', 'build-init', ['worker-base', 'build-init']],
  ['worker:custom', undefined, ['worker-base']],
  ['lead:custom', undefined, ['lead-base']],
  ['orchestrator:custom', undefined, ['lead-base']],
  ['lead', undefined, ['lead-base']],
  ['worker', undefined, ['worker-base']],
  ['', undefined, ['worker-base']],
  ['chef', undefined, ['worker-base']],
  ['worker:../../builder', undefined, ['worker-base']],
  ['worker:builder:extra', undefined, ['worker-base']],
] as const
it.each(roleRows)('s22: role %j selects %j and %j without changing its parsed authority', (role, name, files) => {
  const authority = parseRole(role)
  expect(instructionName(role)).toBe(name)
  expect(instructionFiles(role)).toEqual(files)
  expect(parseRole(role)).toEqual(authority)
})
it('s22: every bare and correctly prefixed default role selects the same files', () => {
  for (const name of ['orchestrator', 'project-lead', 'planning-lead', 'design-lead', 'build-lead', 'planner', 'designer', 'reviewer', 'builder', 'build-init', 'test-runner', 'evaluator']) {
    expect(instructionName(name)).toBe(name)
    const kind = name === 'orchestrator' ? 'orchestrator' : ['project-lead', 'planning-lead', 'design-lead', 'build-lead'].includes(name) ? 'lead' : 'worker'
    expect(instructionFiles(kind + ':' + name)).toEqual(instructionFiles(name))
  }
})
it('s22: defaults compose in mapping order and owner advice replaces only the role', async () => {
  const store = new InstructionStore(home)
  const text = await store.compose('worker:builder')
  expect(text.indexOf('# Rules for every worker')).toBe(0)
  expect(text.indexOf('# Builder')).toBeGreaterThan(0)
  const overridden = await store.compose('builder', { name: 'Owner', roles: { builder: { advice: 'Owner builder.', limits: {} } } })
  expect(overridden).toContain('# Rules for every worker')
  expect(overridden).toContain('Owner builder.')
  expect(overridden).not.toContain('# Builder')
  const custom = await store.compose('worker:custom', { name: 'Owner', roles: { custom: { advice: 'Custom role.', limits: {} } } })
  expect(custom).toContain('Custom role.')
})
it('s22: each file is independently overridden and unknown paths are never opened', async () => {
  writeFileSync(join(folder, 'worker-base.md'), 'Owner base.')
  writeFileSync(join(folder, 'builder.md'), 'Owner role.')
  const store = new InstructionStore(home)
  expect(await store.compose('builder')).toBe('Owner base.\n\nOwner role.')
  expect(await store.compose('worker:../../outside')).toBe('Owner base.')
  expect(await store.compose('builder', { name: 'Owner', roles: { builder: { advice: 'Event advice.', limits: {} } } })).toBe('Owner base.\n\nEvent advice.')
})
it.each(['file-link', 'hard-link', 'directory', 'fifo', 'oversize', 'parent-link', 'home-link', 'lgtm-link', 'missing'])('s22: %s override falls back with one value-free diagnostic', async (kind) => {
  const target = join(folder, 'builder.md'), outside = join(root, 'private-name-secret')
  writeFileSync(outside, 'UNSAFE CONTENT')
  if (kind === 'file-link') symlinkSync(outside, target)
  if (kind === 'hard-link') linkSync(outside, target)
  if (kind === 'directory') mkdirSync(target)
  if (kind === 'fifo') execFileSync('mkfifo', [target])
  if (kind === 'oversize') writeFileSync(target, 'x'.repeat(instructionLimits.fileBytes + 1))
  if (kind === 'parent-link') { renameSync(folder, folder + '-real'); symlinkSync(folder + '-real', folder); writeFileSync(join(folder, 'builder.md'), 'UNSAFE CONTENT') }
  if (kind === 'home-link') { renameSync(home, home + '-real'); symlinkSync(home + '-real', home) }
  if (kind === 'lgtm-link') { const lgtm = join(home, '.lgtm'); renameSync(lgtm, lgtm + '-real'); symlinkSync(lgtm + '-real', lgtm) }
  const lines: string[] = []
  const text = await new InstructionStore(home, (line) => lines.push(line)).compose('builder')
  expect(text).toContain('# Builder')
  expect(text).not.toContain('UNSAFE CONTENT')
  expect(lines.filter((line) => line.includes('builder'))).toHaveLength(kind === 'missing' ? 0 : 1)
  expect(lines.join('\n')).not.toContain(root)
  expect(lines.join('\n')).not.toContain('private-name-secret')
})
it('s22: held descriptors prevent a pathname swap after open', async () => {
  const target = join(folder, 'builder.md'), outside = join(root, 'outside')
  writeFileSync(target, 'Safe held role.'); writeFileSync(outside, 'Outside role.')
  const open = fs.open.bind(fs); let swapped = false
  vi.spyOn(fs, 'open').mockImplementation(async (...args: Parameters<typeof fs.open>) => {
    const handle = await open(...args)
    if (!swapped && String(args[0]).endsWith('/builder.md')) { swapped = true; renameSync(target, target + '.held'); symlinkSync(outside, target) }
    return handle
  })
  expect(await new InstructionStore(home).compose('builder')).toContain('Safe held role.')
})
it('s22: composed text rejects above 48 KiB, including JSON advice', async () => {
  await expect(new InstructionStore(home).compose('builder', { name: 'Owner', roles: { builder: { advice: 'x'.repeat(instructionLimits.composedBytes), limits: {} } } })).rejects.toMatchObject({ reason: 'Role instructions exceed 48 KiB.' })
})
it('s22: defaults load beside the module in a packaged tree with spaces and no design folder', () => {
  const checkout = join(root, 'package with spaces'); mkdirSync(checkout)
  const source = new URL('../../src/roles/', import.meta.url)
  // Copy the runtime module and shipped resources, as the workflow store test does.
  execFileSync('cp', ['-R', fileURLToPath(new URL('instructions', source)), checkout])
  writeFileSync(join(checkout, 'instructions.ts'), readFileSync(new URL('instructions.ts', source)))
  symlinkSync(fileURLToPath(new URL('../../node_modules', import.meta.url)), join(checkout, 'node_modules'))
  const code = `const { InstructionStore } = await import(${JSON.stringify(new URL('file://' + checkout + '/instructions.ts').href)}); console.log(await new InstructionStore(${JSON.stringify(home)}).compose('builder'))`
  expect(execFileSync(process.execPath, ['--input-type=module', '-e', code], { encoding: 'utf8' })).toContain('# Builder')
})

it('s22: a file that grows after stat is still read with a bounded allocation and ignored', async () => {
  const target = join(folder, 'builder.md'); writeFileSync(target, 'small')
  const open = fs.open.bind(fs); let readBytes = 0
  vi.spyOn(fs, 'open').mockImplementation(async (...args: Parameters<typeof fs.open>) => {
    const handle = await open(...args)
    if (String(args[0]).endsWith('/builder.md')) {
      const stat = handle.stat.bind(handle), read = handle.read.bind(handle)
      vi.spyOn(handle, 'stat').mockImplementationOnce(async () => {
        const result = await stat(); writeFileSync(target, 'x'.repeat(instructionLimits.fileBytes + 100)); return result
      })
      vi.spyOn(handle, 'read').mockImplementation(async (...readArgs: any[]) => {
        const result = await (read as any)(...readArgs); readBytes += result.bytesRead; return result
      })
    }
    return handle
  })
  expect(await new InstructionStore(home).compose('builder')).toContain('# Builder')
  expect(readBytes).toBe(instructionLimits.fileBytes + 1)
})
it('s22: a directory swapped after opening cannot redirect its child read', async () => {
  writeFileSync(join(folder, 'test-runner.md'), 'Safe folder role.')
  const outside = join(root, 'outside'); mkdirSync(outside); writeFileSync(join(outside, 'test-runner.md'), 'Outside role.')
  const open = fs.open.bind(fs); let swapped = false
  vi.spyOn(fs, 'open').mockImplementation(async (...args: Parameters<typeof fs.open>) => {
    const handle = await open(...args)
    if (!swapped && String(args[0]).endsWith('/roles')) { swapped = true; renameSync(folder, folder + '.held'); symlinkSync(outside, folder) }
    return handle
  })
  expect(await new InstructionStore(home).compose('worker:test-runner')).toContain('Safe folder role.')
})
