#!/usr/bin/env node
// Picks a free port and a temp db path once, then runs `playwright test`
// as a child process that inherits them via env vars. Doing this here
// (rather than inside playwright.config.ts) avoids re-picking a different
// port every time Playwright re-imports the config in a new process.
import net from 'net';
import os from 'os';
import path from 'path';
import { spawn } from 'child_process';

function getFreePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const address = srv.address();
      const port = typeof address === 'object' && address ? address.port : null;
      srv.close(() => (port ? resolve(port) : reject(new Error('no free port'))));
    });
  });
}

const port = await getFreePort();
const dbPath = path.join(os.tmpdir(), `dewey-e2e-${Date.now()}-${process.pid}.db`);

const child = spawn('npx', ['playwright', 'test', ...process.argv.slice(2)], {
  stdio: 'inherit',
  env: {
    ...process.env,
    DEWEY_E2E_PORT: String(port),
    DEWEY_E2E_DB: dbPath,
  },
});

child.on('exit', (code) => process.exit(code ?? 1));
