import { test } from '@playwright/test'
import { exerciseStartup } from './support/studioStartup.ts'

test('website loads and signs in during slow recovery, then continues automatically', async ({ page }) => exerciseStartup(page, 'startup'))
test('agent and channel drafts survive reload in this tab and clear after send', async ({ page }) => exerciseStartup(page, 'drafts'))
test('one plan row updates live and replays the latest plan of each turn', async ({ page }) => exerciseStartup(page, 'plans'))
test('sample mock keeps composer drafts across reload without engine data', async ({ page }) => exerciseStartup(page, 'mock-drafts'))
test('unknown subscription preserves account controls and model choices', async ({ page }) => exerciseStartup(page, 'mock-unknown'))
