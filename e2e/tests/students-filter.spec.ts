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
