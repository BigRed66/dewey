import { defineConfig, devices } from '@playwright/test';

const port = Number(process.env.DEWEY_E2E_PORT);
const dbPath = process.env.DEWEY_E2E_DB;
if (!port || !dbPath) {
  throw new Error('Run tests via `npm run test:e2e` (it provides DEWEY_E2E_PORT / DEWEY_E2E_DB)');
}

const baseURL = `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: './e2e/tests',
  fullyParallel: false, // all tests share one seeded database
  workers: 1,
  reporter: 'list',
  use: {
    baseURL,
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: 'go run .',
    cwd: process.cwd(),
    port,
    env: {
      DEWEY_ADDR: `127.0.0.1:${port}`,
      DEWEY_DB_PATH: dbPath,
      DEWEY_SEED: '1',
    },
    reuseExistingServer: false,
    timeout: 30_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
