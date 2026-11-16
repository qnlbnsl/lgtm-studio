import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { Page } from '@playwright/test'
import * as Effect from 'effect/Effect'
import { afterEach, expect, it, vi } from 'vitest'
import { engineLists } from '../../engine/src/pieces/lists.ts'
import { measureGitJourney } from '../e2e/support/gitMeasurements.ts'

afterEach(() => { vi.unstubAllEnvs(); vi.restoreAllMocks() })

it('leaves the service and browser unwrapped when measurement is disabled', () => {
  vi.stubEnv('LGTM_GIT_MEASURE_DIR', undefined)
  const page = { on: vi.fn(), off: vi.fn() }, log = vi.spyOn(console, 'log').mockImplementation(() => {})
  const measurement = measureGitJourney(page as unknown as Page), server = Effect.succeed(undefined)
  expect(measurement.lists).toBe(engineLists)
  expect(measurement.server(server)).toBe(server)
  measurement.finish()
  expect(page.on).not.toHaveBeenCalled()
  expect(page.off).not.toHaveBeenCalled()
  expect(log).not.toHaveBeenCalled()
})

it('preserves the journey failure when the measurement cannot be written in finally', () => {
  const directory = mkdtempSync(join(tmpdir(), 'lgtm-git-measurement-'))
  try {
    const blocked = join(directory, 'file')
    writeFileSync(blocked, 'This file cannot be used as a directory.')
    vi.stubEnv('LGTM_GIT_MEASURE_DIR', blocked)
    vi.spyOn(console, 'log').mockImplementation(() => {})
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const measurement = measureGitJourney({ on: vi.fn(), off: vi.fn() } as unknown as Page)
    const failure = new Error('The original journey failure')
    expect(() => { try { throw failure } finally { measurement.finish() } }).toThrow(failure)
    expect(warn.mock.calls).toEqual([['Git journey measurement failed:', 'EEXIST']])
  } finally { rmSync(directory, { recursive: true, force: true }) }
})
