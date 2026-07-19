import { defineConfig, devices } from '@playwright/test'

const localExecutable = process.env.PLAYWRIGHT_EXECUTABLE_PATH

export default defineConfig({
  testDir: './e2e',
  testMatch: 'desktop-docs-navigation.spec.ts',
  forbidOnly: Boolean(process.env.CI),
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://127.0.0.1:4174',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'desktop-chromium',
      use: {
        ...devices['Desktop Chrome'],
        launchOptions: localExecutable ? { executablePath: localExecutable } : undefined,
      },
    },
  ],
  webServer: {
    command: 'npm run dev -- --mode desktop --port 4174',
    url: 'http://127.0.0.1:4174/#/',
    reuseExistingServer: false,
    timeout: 120_000,
  },
})
