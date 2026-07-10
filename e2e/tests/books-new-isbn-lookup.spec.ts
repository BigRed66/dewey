import { test, expect } from '@playwright/test';

// Regression guard for the /books/new tag-prefill fix: landing there with a
// scanned tag id must not short-circuit the normal "type an ISBN, click
// Weiter" lookup path — that still has to run and can still switch the page
// over to the "book already exists, just add a copy" view.

test('entering an ISBN and clicking Weiter still looks it up after a tag prefill', async ({ page }) => {
  await page.goto('/books/new?nfc_tag_id=NEWTAG-1');
  await expect(page.locator('#nfc_tag_id')).toHaveValue('NEWTAG-1');
  await expect(page.locator('#title')).toBeVisible(); // blank manual form from the tag alone

  await page.getByLabel('ISBN').fill('978-3-1234-0006-6'); // seeded "Die drei ???"
  await page.getByRole('button', { name: 'Weiter' }).click();

  await expect(page.getByRole('heading', { name: 'Buch bereits vorhanden — Exemplar hinzufügen' })).toBeVisible();
  await expect(page.locator('body')).toContainText('Die drei ???');
  await expect(page.getByRole('button', { name: 'Exemplar hinzufügen' })).toBeVisible();
});
