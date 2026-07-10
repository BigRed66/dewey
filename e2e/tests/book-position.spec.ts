import { test, expect } from '@playwright/test';

// Coverage for the shelf position (row + column) on books: seeded on
// "Die kleine Raupe Nimmersatt" (Reihe A, Spalte 1), editable via the book
// detail page, and settable when registering a brand new book.

test('a seeded book shows its shelf position on the detail page', async ({ page }) => {
  await page.goto('/books/1'); // Die kleine Raupe Nimmersatt — Row A / Column 1
  await expect(page.locator('body')).toContainText('Reihe A');
  await expect(page.locator('body')).toContainText('Spalte 1');
});

test('editing a book updates its shelf position', async ({ page }) => {
  await page.goto('/books/1');
  await page.getByRole('link', { name: 'Bearbeiten' }).click();
  await page.locator('#edit-shelf-row').fill('Z');
  await page.locator('#edit-shelf-column').fill('9');
  await page.getByRole('button', { name: 'Speichern' }).click();

  await expect(page).toHaveURL(/\/books\/1$/);
  await expect(page.locator('body')).toContainText('Reihe Z');
  await expect(page.locator('body')).toContainText('Spalte 9');
});

test('registering a new book can set a shelf position', async ({ page }) => {
  await page.goto('/books/new?nfc_tag_id=SHELF-TEST-1');
  await page.locator('#title').fill('Testbuch für Regal');
  await page.locator('#shelf_row').fill('D');
  await page.locator('#shelf_column').fill('5');
  await page.getByRole('button', { name: 'Speichern' }).click();

  await expect(page).toHaveURL(/\/books$/);
  await page.locator('.tbl-row', { hasText: 'Testbuch für Regal' }).click();
  await expect(page.locator('body')).toContainText('Reihe D');
  await expect(page.locator('body')).toContainText('Spalte 5');
});
