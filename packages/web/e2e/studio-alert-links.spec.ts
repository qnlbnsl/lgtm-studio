import { test } from '@playwright/test'
import { exerciseStudioAlertLinks } from './support/studioAlertLinks.ts'

test('every notification kind opens its existing subject route through the real engine', async ({ page }) => {
  test.setTimeout(90_000)
  await exerciseStudioAlertLinks(page)
})
