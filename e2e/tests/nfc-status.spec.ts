import { test, expect } from '@playwright/test';

// Regression test: configuring an NFC port that isn't actually reachable
// (reader unplugged, wrong path, ...) used to still show "ready"/"Verbunden"
// everywhere, because the checks only asked "is a reader configured?"
// instead of "is the serial port actually open?".

test('an unreachable NFC port shows disconnected, not ready', async ({ page }) => {
  await page.goto('/settings');
  await page.evaluate(() => {
    const select = document.getElementById('nfc-port-select') as HTMLSelectElement;
    const opt = document.createElement('option');
    opt.value = '/dev/ttyFAKE_DOES_NOT_EXIST';
    opt.textContent = opt.value;
    select.appendChild(opt);
    select.value = opt.value;
  });
  await page.locator('form[action="/settings/nfc"]').getByRole('button', { name: 'Speichern' }).click();

  await expect(page).toHaveURL(/saved=nfc/);
  // serial.Open() on the bogus path fails almost immediately, but it's a
  // background goroutine racing this page load, so poll a couple of reloads.
  await expect
    .poll(async () => {
      await page.reload();
      return page.locator('.pill', { hasText: 'Nicht verbunden' }).count();
    })
    .toBeGreaterThan(0);

  // The nav bar's live SSE status must also settle on "not reachable",
  // never on "bereit" (ready).
  await page.goto('/books');
  await expect(page.locator('#nav-text')).toHaveText('Reader nicht erreichbar', { timeout: 10_000 });
  await expect(page.locator('#nav-dot')).toHaveClass(/error/);

  // Clean up so later tests see NFC disabled again (the bogus port is
  // still selected on reload, so explicitly reset to "— Kein Port —").
  await page.goto('/settings');
  await page.locator('#nfc-port-select').selectOption('');
  await page.locator('form[action="/settings/nfc"]').getByRole('button', { name: 'Speichern' }).click();
  await expect(page.locator('.pill', { hasText: 'Deaktiviert' })).toBeVisible();
});
