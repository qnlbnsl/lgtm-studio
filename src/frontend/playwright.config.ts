import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.{test,spec}.{ts,tsx}',
  globalSetup: './e2e/support/repository-server.ts',
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 2,
  timeout: 45_000,
  use: {
    browserName: 'chromium',
    headless: true,
    serviceWorkers: 'block',
    actionTimeout: 6000,
  },
  projects: [
    { name: 'desktop', use: { viewport: { width: 1440, height: 900 } } },
    { name: 'phone', use: { viewport: { width: 390, height: 844 } } },
  ],
});
