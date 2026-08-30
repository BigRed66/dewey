import { test, expect } from '@playwright/test';

// Regression test for the "…" row action menu being clipped/hidden on the
// last row of a table (the dropdown used to be position:absolute inside a
// .card with overflow:hidden, so it got cut off exactly when there was no
// room left below it in the card).
//
// Each test scrolls the row into view before opening its menu: the menu
// closes on any scroll event, so letting Playwright auto-scroll as part of
// the click would race with opening it once the list is long enough to need
// scrolling. A user scrolls first and then clicks, which is what this mirrors.

test('last row action menu opens and its edit link is clickable on Bücher', async ({ page }) => {
  await page.goto('/books');
  const lastRow = page.locator('#books-body .tbl-row').last();
  await lastRow.scrollIntoViewIfNeeded();
  await lastRow.locator('.row-menu-btn').click();
  const editLink = lastRow.locator('.row-menu-drop a.mitem-edit');
  await expect(editLink).toBeVisible();
  await editLink.click();
  await expect(page).toHaveURL(/\/books\/\d+\?edit=true/);
});

test('last row action menu opens and its edit link is clickable on Schüler', async ({ page }) => {
  await page.goto('/students');
  const lastRow = page.locator('.tbl-row').last();
  await lastRow.scrollIntoViewIfNeeded();
  await lastRow.locator('.row-menu-btn').click();
  const editLink = lastRow.locator('.row-menu-drop a.mitem-edit');
  await expect(editLink).toBeVisible();
  await editLink.click();
  await expect(page).toHaveURL(/\/students\?edit=\d+/);
});

test('last row action menu opens and its edit link is clickable on Klassen', async ({ page }) => {
  await page.goto('/classes');
  const lastRow = page.locator('.tbl-row').last();
  await lastRow.scrollIntoViewIfNeeded();
  await lastRow.locator('.row-menu-btn').click();
  const editLink = lastRow.locator('.row-menu-drop a.mitem-edit');
  await expect(editLink).toBeVisible();
  await editLink.click();
  await expect(page).toHaveURL(/\/classes\?edit=\d+/);
});
