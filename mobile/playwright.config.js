import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: 'test/e2e',
  timeout: 30000,
  retries: 0,
  reporter: 'list',
  snapshotPathTemplate: '{testDir}/goldens/{arg}{ext}',
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:8888',
    ...devices['Pixel 7'],
    locale: 'ru-RU',
    timezoneId: 'Europe/Moscow',
  },
});
