import { afterEach, beforeAll, expect, it, vi } from 'vitest'

let kit: any, lib: any, ui: any
beforeAll(async () => {
  globalThis.matchMedia ??= ((query: string) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} })) as unknown as typeof matchMedia
  kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  lib = await import('../../../design/mock/js/lib.js' as string)
  ui = await import('../../../design/mock/js/ui.js' as string)
})
afterEach(() => { lib.live.on = false; vi.useRealTimers() })
it('shared help names its button, describes the setting, pins on click and dismisses on Escape', async () => {
  const host = document.createElement('div'); document.body.append(host)
  const settle = () => new Promise((resolve) => setTimeout(resolve, 0))
  try {
    kit.render(kit.h(ui.SettingHelp, { label: 'Base branch' }, 'Finished work merges here.'), host)
    const button = host.querySelector<HTMLButtonElement>('button')!
    const tip = document.getElementById(button.getAttribute('aria-describedby')!)!
    expect(button.type).toBe('button')
    expect(button.getAttribute('aria-label')).toBe('About Base branch')
    expect(tip.textContent).toContain('Finished work merges here.')
    expect(tip.hidden).toBe(true)
    button.focus(); await settle()
    expect(tip.hidden).toBe(false)
    button.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await settle()
    expect(tip.hidden).toBe(true)
    button.click(); button.blur(); await settle()
    expect(tip.hidden).toBe(false)
    expect(button.getAttribute('aria-expanded')).toBe('true')
    expect(tip.querySelector('button')).toBeNull()
    const close = host.querySelector<HTMLButtonElement>('.setting-help-close')!
    button.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); button.blur(); await settle()
    button.focus(); await settle(); close.focus(); await settle()
    expect(tip.hidden).toBe(false)
    close.click(); await settle()
    expect(tip.hidden).toBe(true)
    expect(button.getAttribute('aria-expanded')).toBe('false')
  } finally { kit.render(null, host); host.remove() }
})
it('the sample Project settings keeps the full design and replaces every hint line with shared help', async () => {
  const data = await import('../../../design/mock/js/data.js' as string)
  const screen = await import('../../../design/mock/js/screens/service.js' as string)
  const state = { ...data.seed(), ui: {}, toasts: [] }; screen.seedService(state); lib.store.state = state
  const host = document.createElement('div'); document.body.append(host)
  try {
    kit.render(kit.h(screen.ProjectSettings, { s: state, project: state.projects[0] }), host)
    expect(host.querySelector('.svc-label small')).toBeNull()
    for (const selector of ['#ps-base', '#ps-sub', '[name=ps-perm]', '[aria-label="Write tests first"]', '[aria-label="Agent review"]', '[aria-label="Remove agent credits when merging"]', '#ps-all', '.svc-assign']) expect(host.querySelector(selector), selector).not.toBeNull()
    for (const label of ['Name', 'Goal', 'Runs on', 'Base branch', 'Lead subscription', 'Permission', 'Write tests first', 'Agent review', 'Remove agent credits when merging', 'Archive this project', 'Project planning', 'Starting work']) {
      const button = host.querySelector<HTMLButtonElement>(`[aria-label="About ${label}"]`)!
      expect(button, label).not.toBeNull()
      expect(document.getElementById(button.getAttribute('aria-describedby')!)?.textContent?.length, label).toBeGreaterThan(0)
    }
    expect([...host.querySelectorAll('button')].some((b) => b.textContent?.trim() === 'Delete project')).toBe(false)
    state.projects[0].archived = true
    kit.render(kit.h(screen.ProjectSettings, { s: state, project: state.projects[0] }), host)
    expect([...host.querySelectorAll('button')].some((b) => b.textContent?.trim() === 'Delete project')).toBe(true)
    expect(host.querySelector('[aria-label="About Delete this project"]')).not.toBeNull()
  } finally { kit.render(null, host); host.remove() }
})
