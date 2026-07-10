import { test, expect } from '@playwright/test';

// Coverage for the loan checkout/return flow. Checkout has a real manual
// form (type or scan an NFC tag), so it's driven through the UI like a user
// would. Return has no manual UI without physical NFC hardware — the app's
// own frontend triggers it by POSTing to /loans/return (see the
// modal-return-btn handler in layout.html), so hitting that same endpoint
// directly is the accurate way to exercise it without a reader attached.

test.describe.serial('loan checkout', () => {
  test('checking out an available copy creates an active loan', async ({ page }) => {
    await page.goto('/loans/checkout');
    await page.getByLabel('NFC-Tag des Buchs').fill('E2E-4-1'); // Tierlexikon für Kinder, single copy
    await page.getByLabel('Schüler').selectOption({ label: 'Ben Schulz' });
    await page.getByRole('button', { name: 'Ausleihen' }).click();

    await expect(page).toHaveURL(/\/loans$/);
    const row = page.locator('.tbl-row', { hasText: 'Tierlexikon für Kinder' });
    await expect(row).toBeVisible();
    await expect(row).toContainText('Ben Schulz');
  });

  test('checking out a copy that is already on loan is refused', async ({ page }) => {
    await page.goto('/loans/checkout');
    await page.getByLabel('NFC-Tag des Buchs').fill('E2E-3-1'); // Harry Potter copy on active loan (Anna)
    await page.getByLabel('Schüler').selectOption({ label: 'Ben Schulz' });
    await page.getByRole('button', { name: 'Ausleihen' }).click();

    await expect(page.locator('.alert-error')).toContainText('bereits ausgeliehen');
    await expect(page).toHaveURL(/\/loans\/checkout/);
  });

  test('checking out an unknown tag is refused', async ({ page }) => {
    await page.goto('/loans/checkout');
    await page.getByLabel('NFC-Tag des Buchs').fill('does-not-exist');
    await page.getByLabel('Schüler').selectOption({ label: 'Ben Schulz' });
    await page.getByRole('button', { name: 'Ausleihen' }).click();

    await expect(page.locator('.alert-error')).toContainText('Kein Buch mit diesem Tag gefunden');
  });

  test('returning the checked-out copy closes the loan', async ({ page }) => {
    const res = await page.request.post('/loans/return', {
      headers: { Accept: 'application/json' },
      form: { nfc_tag_id: 'E2E-4-1' },
    });
    expect(res.ok()).toBe(true);
    expect(await res.json()).toEqual({ ok: true });

    await page.goto('/loans');
    await expect(page.locator('.tbl-row', { hasText: 'Tierlexikon für Kinder' })).toHaveCount(0);
  });

  test('returning a copy with no open loan is refused', async ({ page }) => {
    const res = await page.request.post('/loans/return', {
      headers: { Accept: 'application/json' },
      form: { nfc_tag_id: 'E2E-6-1' }, // Die drei ??? — never loaned
    });
    expect(res.ok()).toBe(false);
    const body = await res.json();
    expect(body.error).toContain('Kein offener Ausleihvorgang');
  });
});
