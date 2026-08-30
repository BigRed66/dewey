import { test, expect } from '@playwright/test';

// Regression test: the filtered list used to be loaded by a query that didn't
// join the classes table, so .Class was nil and rendering "Klasse {{.Class.Year}}"
// blew up with a nil pointer as soon as a class chip was clicked.

test('filtering pupils by class shows that class only, with its class and teacher', async ({ page }) => {
  await page.goto('/students');

  await page.getByRole('link', { name: 'Klasse 1A', exact: true }).click();
  await expect(page).toHaveURL(/\/students\?class=\d+/);

  // Earlier specs delete pupils from the shared seeded database, so assert on
  // the shape of the filtered rows rather than an absolute count.
  const rows = page.locator('.tbl-row');
  expect(await rows.count()).toBeGreaterThan(0);
  for (const row of await rows.all()) {
    await expect(row).toContainText('Klasse 1A');
    await expect(row).toContainText('Frau Bauer');
  }

  // A pupil from another class must not leak into the filtered view.
  await expect(page.getByText('David Wolf')).toHaveCount(0);
});

// The box on the pupils list used to filter the class chips, so typing a
// pupil's name hid every chip and left the list untouched — it looked broken.
test('searching filters the pupil list by name', async ({ page }) => {
  await page.goto('/students');

  const visibleRows = page.locator('.tbl-row[data-name]:visible');
  const total = await visibleRows.count();
  expect(total).toBeGreaterThan(0);

  // Imported by csv-import.spec.ts; two pupils share this surname.
  await page.fill('#student-search', 'Sonnenberg');
  await expect(visibleRows).toHaveCount(2);
  for (const row of await visibleRows.all()) {
    await expect(row).toContainText('Sonnenberg');
  }
  await expect(page.locator('#student-search-count')).toContainText(`2 von ${total}`);

  // No match shows an empty state rather than a silently blank table.
  await page.fill('#student-search', 'gibtesnicht');
  await expect(visibleRows).toHaveCount(0);
  await expect(page.locator('#student-search-empty')).toBeVisible();

  await page.fill('#student-search', '');
  await expect(visibleRows).toHaveCount(total);
});

test('searching also matches the class name', async ({ page }) => {
  await page.goto('/students');
  await page.fill('#student-search', '1A');

  const visibleRows = page.locator('.tbl-row[data-name]:visible');
  expect(await visibleRows.count()).toBeGreaterThan(0);
  for (const row of await visibleRows.all()) {
    await expect(row).toContainText('Klasse 1A');
  }
});
