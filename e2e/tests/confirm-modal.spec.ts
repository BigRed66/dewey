import { test, expect } from '@playwright/test';

// Regression test: delete actions used to trigger the native browser
// confirm() dialog. They should now show the app's own styled modal.

test('delete uses the in-app confirm modal, not window.confirm', async ({ page }) => {
  let nativeDialogFired = false;
  page.on('dialog', () => { nativeDialogFired = true; });

  await page.goto('/classes');
  await page.getByLabel('Jahrgang').selectOption('1');
  await page.getByLabel('Gruppe').fill('Z');
  await page.getByLabel('Lehrkraft').fill('Testlehrer');
  await page.getByRole('button', { name: 'Hinzufügen' }).click();

  const row = page.locator('.tbl-row', { hasText: 'Testlehrer' });
  await expect(row).toBeVisible();
  await row.locator('.row-menu-btn').click();
  await row.locator('.row-menu-drop button.mitem-del').click();

  const modal = page.locator('#confirm-modal-overlay');
  await expect(modal).toBeVisible();
  await expect(modal.locator('#confirm-modal-title')).toHaveText('Klasse löschen?');

  // Cancel keeps the row.
  await modal.getByRole('button', { name: 'Abbrechen' }).click();
  await expect(modal).toBeHidden();
  await expect(row).toBeVisible();
  expect(nativeDialogFired).toBe(false);

  // Confirming actually deletes it.
  await row.locator('.row-menu-btn').click();
  await row.locator('.row-menu-drop button.mitem-del').click();
  await page.locator('#confirm-modal-ok').click();
  await expect(page).toHaveURL(/\/classes$/);
  await expect(page.locator('.tbl-row', { hasText: 'Testlehrer' })).toHaveCount(0);
  expect(nativeDialogFired).toBe(false);
});
