import { test } from '@playwright/test'
import { exerciseStudioChatRows } from './support/studioChatRows.ts'

test('live chat rows: one file edit, command output and question in engine order', async ({ page }) => exerciseStudioChatRows(page))
