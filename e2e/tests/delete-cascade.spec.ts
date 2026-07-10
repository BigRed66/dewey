import { test, expect } from '@playwright/test';

// Regression tests for the FK-cascade delete bugs:
// deleting a book/class/student used to hard-fail with a raw SQLite
// "FOREIGN KEY constraint failed" error whenever any loan history (even
// long-returned) referenced it. It should now:
//   - succeed and clean up the loan history when nothing is on active loan
//   - be refused with a friendly message when something is still on loan

async function confirmDelete(page: import('@playwright/test').Page, ok: boolean) {
  const modal = page.locator('#confirm-modal-overlay');
  await expect(modal).toBeVisible();
  if (ok) {
    await page.locator('#confirm-modal-ok').click();
  } else {
    await page.getByRole('button', { name: 'Abbrechen' }).click();
  }
}

test.describe.serial('cascade deletes', () => {
  test('book with only returned loan history deletes cleanly', async ({ page }) => {
    await page.goto('/books');
    const row = page.locator('.tbl-row', { hasText: 'Die kleine Raupe Nimmersatt' });
    await expect(row).toBeVisible();
    await row.locator('.row-menu-btn').click();
    await row.locator('.row-menu-drop button.mitem-del').click();
    await confirmDelete(page, true);

    await expect(page).toHaveURL(/\/books$/);
    await expect(page.locator('.tbl-row', { hasText: 'Die kleine Raupe Nimmersatt' })).toHaveCount(0);
  });

  test('book with an active loan cannot be deleted', async ({ page }) => {
    await page.goto('/books');
    await page.locator('.tbl-row', { hasText: 'Harry Potter' }).click();
    await expect(page.getByRole('heading', { name: /Harry Potter/ })).toBeVisible();

    await page.getByRole('button', { name: 'Löschen' }).click();
    await confirmDelete(page, true);

    await expect(page.locator('body')).toContainText('Es sind noch Exemplare ausgeliehen');

    await page.goto('/books');
    await expect(page.locator('.tbl-row', { hasText: 'Harry Potter' })).toBeVisible();
  });

  test('student with only returned loan history deletes cleanly, without touching classmates', async ({ page }) => {
    await page.goto('/students');
    const row = page.locator('.tbl-row', { hasText: 'Clara Fischer' });
    await expect(row).toBeVisible();
    await row.locator('.row-menu-btn').click();
    await row.locator('.row-menu-drop button.mitem-del').click();
    await confirmDelete(page, true);

    await expect(page).toHaveURL(/\/students$/);
    await expect(page.locator('.tbl-row', { hasText: 'Clara Fischer' })).toHaveCount(0);
    // Anna is in the same class and must be unaffected.
    await expect(page.locator('.tbl-row', { hasText: 'Anna Meier' })).toBeVisible();
  });

  test('student with an active loan cannot be deleted', async ({ page }) => {
    await page.goto('/students');
    const row = page.locator('.tbl-row', { hasText: 'Anna Meier' });
    await row.locator('.row-menu-btn').click();
    await row.locator('.row-menu-drop button.mitem-del').click();
    await confirmDelete(page, true);

    await expect(page.locator('.alert-error')).toContainText('Es sind noch Bücher ausgeliehen');
    await expect(page.locator('.tbl-row', { hasText: 'Anna Meier' })).toBeVisible();
  });

  test('class with a student on active loan cannot be deleted', async ({ page }) => {
    await page.goto('/classes');
    const row = page.locator('.tbl-row', { hasText: 'Frau Bauer' }); // Anna's class
    await row.locator('.row-menu-btn').click();
    await row.locator('.row-menu-drop button.mitem-del').click();
    await confirmDelete(page, true);

    await expect(page.locator('.alert-error')).toContainText('Schüler haben noch ausgeliehene Bücher');
    await expect(page.locator('.tbl-row', { hasText: 'Frau Bauer' })).toBeVisible();
  });

  test('class where students only have returned loan history deletes cleanly', async ({ page }) => {
    await page.goto('/classes');
    const row = page.locator('.tbl-row', { hasText: 'Frau Vogel' }); // Felix's class
    await row.locator('.row-menu-btn').click();
    await row.locator('.row-menu-drop button.mitem-del').click();
    await confirmDelete(page, true);

    await expect(page).toHaveURL(/\/classes$/);
    await expect(page.locator('.tbl-row', { hasText: 'Frau Vogel' })).toHaveCount(0);

    await page.goto('/students');
    await expect(page.locator('.tbl-row', { hasText: 'Felix Krause' })).toHaveCount(0);
    await expect(page.locator('.tbl-row', { hasText: 'Greta Braun' })).toHaveCount(0);
  });
});
