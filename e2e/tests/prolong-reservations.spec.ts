import { test, expect } from '@playwright/test';

// Coverage for the two new features: prolonging a loan ("Verlängern", with a
// chosen due date) and placing a reservation ("Vormerkung") for another
// student who wants the book next — which should then block prolonging
// until the reservation is cancelled. All three required entry points are
// exercised: the loans list, the book detail page, and the student detail
// page. Prolong and reserve are deliberately separate modals per row.
//
// Fixtures relied on (from internal/seed/seed.go, ids are deterministic on a
// freshly seeded db): student 1 = Anna Meier, has an active loan on book 3
// = "Harry Potter und der Stein der Weisen"; student 2 = Ben Schulz, free.

test.describe.serial('prolong and reservations', () => {
  test('loans list: prolonging to a chosen date succeeds', async ({ page }) => {
    await page.goto('/loans');
    const row = page.locator('.tbl-row', { hasText: 'Harry Potter' });
    await row.locator('.row-menu-btn').click();
    await row.getByRole('button', { name: 'Verlängern' }).click();

    const modal = page.locator('#prolong-modal-overlay');
    await expect(modal).toBeVisible();
    await expect(modal.locator('#prolong-modal-title')).toContainText('Harry Potter und der Stein der Weisen');

    await modal.locator('#prolong-modal-date').fill('2027-01-15');
    await modal.getByRole('button', { name: 'Verlängern' }).click();

    await expect(page.locator('.alert-success')).toContainText('Ausleihe verlängert bis 15.01.2027');
  });

  test('loans list: reserving the book for another student blocks further prolonging', async ({ page }) => {
    await page.goto('/loans');
    let row = page.locator('.tbl-row', { hasText: 'Harry Potter' });
    await row.locator('.row-menu-btn').click();
    await row.getByRole('button', { name: 'Vormerken' }).click();

    const reserveModal = page.locator('#reserve-modal-overlay');
    await expect(reserveModal).toBeVisible();
    await reserveModal.locator('#reserve-modal-student').selectOption({ label: 'Ben Schulz' });
    await reserveModal.getByRole('button', { name: 'Vormerken' }).click();

    await expect(page.locator('.alert-success')).toContainText('Vormerkung gespeichert');
    row = page.locator('.tbl-row', { hasText: 'Harry Potter' });
    await expect(row).toContainText('1 Vormerkung');

    await row.locator('.row-menu-btn').click();
    await row.getByRole('button', { name: 'Verlängern' }).click();
    const prolongModal = page.locator('#prolong-modal-overlay');
    await expect(prolongModal.locator('#prolong-modal-blocked')).toBeVisible();
    await expect(prolongModal.locator('#prolong-modal-form')).toBeHidden();

    await prolongModal.locator('button:has-text("Schließen")').click();
    await row.locator('.row-menu-btn').click();
    await row.getByRole('button', { name: 'Vormerken' }).click();
    await expect(reserveModal.locator('#reserve-modal-waiting')).toContainText('Ben Schulz');
  });

  test('book detail: the reservation shows in the waiting list and can be cancelled there', async ({ page }) => {
    await page.goto('/books/3');
    await expect(page.locator('body')).toContainText('Vormerkungen (1)');
    await expect(page.locator('form[action^="/reservations/"] button[title="Vormerkung stornieren"]')).toBeVisible();

    await page.locator('form[action^="/reservations/"] button[title="Vormerkung stornieren"]').click();
    await expect(page.locator('.alert-success')).toContainText('Vormerkung storniert');
    await expect(page.locator('body')).not.toContainText('Vormerkungen (1)');
  });

  test('loans list: prolonging works again once the reservation is gone', async ({ page }) => {
    await page.goto('/loans');
    const row = page.locator('.tbl-row', { hasText: 'Harry Potter' });
    await row.locator('.row-menu-btn').click();
    await row.getByRole('button', { name: 'Verlängern' }).click();

    const modal = page.locator('#prolong-modal-overlay');
    await expect(modal.locator('#prolong-modal-form')).toBeVisible();
    await expect(modal.locator('#prolong-modal-blocked')).toBeHidden();
    await modal.getByRole('button', { name: 'Verlängern' }).click();
    await expect(page.locator('.alert-success')).toContainText('Ausleihe verlängert bis');
  });

  test('prolonging to a date in the past is refused', async ({ page }) => {
    await page.goto('/loans');
    const row = page.locator('.tbl-row', { hasText: 'Harry Potter' });
    await row.locator('.row-menu-btn').click();
    await row.getByRole('button', { name: 'Verlängern' }).click();

    const modal = page.locator('#prolong-modal-overlay');
    await modal.locator('#prolong-modal-date').fill('2020-01-01');
    await modal.getByRole('button', { name: 'Verlängern' }).click();

    await expect(page.locator('.alert-error')).toContainText('muss in der Zukunft liegen');
  });

  test('student detail: reserving a book for this student, then cancelling it', async ({ page }) => {
    await page.goto('/students/1'); // Anna Meier
    await page.getByRole('button', { name: 'Buch vormerken' }).click();
    await page.locator('#reserve-book').selectOption({ label: 'Gregs Tagebuch' });
    await page.locator('#reserve-overlay').getByRole('button', { name: 'Vormerken' }).click();

    await expect(page.locator('.alert-success')).toContainText('Vormerkung gespeichert');
    await expect(page.locator('form[action^="/reservations/"] button[title="Vormerkung stornieren"]')).toBeVisible();

    await page.locator('form[action^="/reservations/"] button[title="Vormerkung stornieren"]').click();
    await expect(page.locator('.alert-success')).toContainText('Vormerkung storniert');
  });

  test('reserving the same book twice for the same student is refused', async ({ page }) => {
    await page.goto('/books/3');
    await page.getByRole('button', { name: 'Vormerken' }).click();
    await page.locator('#reserve-student').selectOption({ label: 'Ben Schulz' });
    await page.locator('#reserve-overlay').getByRole('button', { name: 'Vormerken' }).click();
    await expect(page.locator('.alert-success')).toContainText('Vormerkung gespeichert');

    await page.getByRole('button', { name: 'Vormerken' }).click();
    await page.locator('#reserve-student').selectOption({ label: 'Ben Schulz' });
    await page.locator('#reserve-overlay').getByRole('button', { name: 'Vormerken' }).click();
    await expect(page.locator('.alert-error')).toContainText('bereits vorgemerkt');

    // clean up so this reservation doesn't linger for other tests
    await page.locator('form[action^="/reservations/"] button[title="Vormerkung stornieren"]').click();
  });
});
