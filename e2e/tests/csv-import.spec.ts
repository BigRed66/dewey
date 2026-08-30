import { test, expect } from '@playwright/test';

// These tests share the seeded database with every other spec, so they use
// year-4 classes and names that don't appear in the seed data.
const csv = [
  'Familienname\tVornamen\tKlassenname',
  'Sonnenberg\tMarit\t4d',
  'Sonnenberg\tJonne Piet\t4d',
  'Talvik\tRuben\t4e',
  'Fehler\tMax\t9z', // year outside 1..4 — must be rejected, not fatal
].join('\n');

async function upload(page, contents: string) {
  await page.goto('/import');
  await page.setInputFiles('#file', {
    name: 'schueler.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from(contents, 'utf-8'),
  });
  await page.getByRole('button', { name: 'Importieren' }).click();
}

test.describe('csv import', () => {
  test('importing a pupil list creates the classes and pupils', async ({ page }) => {
    await upload(page, csv);

    await expect(page.locator('.alert-success')).toContainText('3 Schüler angelegt');
    await expect(page.locator('.alert-success')).toContainText('2 Klassen neu erstellt');

    // The malformed row is reported per line instead of failing the import.
    await expect(page.getByText('Nicht übernommene Zeilen')).toBeVisible();
    await expect(page.locator('.tbl-row')).toContainText('liegt nicht zwischen 1 und 4');

    await page.goto('/students');
    await expect(page.getByText('Marit Sonnenberg')).toBeVisible();
    await expect(page.getByText('Jonne Piet Sonnenberg')).toBeVisible();
    await expect(page.getByText('Ruben Talvik')).toBeVisible();
  });

  test('new classes get the placeholder teacher', async ({ page }) => {
    await page.goto('/classes');
    // Jahrgang and Gruppe are separate columns, so 4d renders as "Klasse 4" + "D".
    const imported = page.locator('.tbl-row').filter({ hasText: 'Noch einzutragen' });
    await expect(imported).toHaveCount(2);
    await expect(imported.first()).toContainText('Klasse 4');
  });

  test('re-importing the same file skips pupils that already exist', async ({ page }) => {
    await upload(page, csv);

    await expect(page.locator('.alert-success')).toContainText('0 Schüler angelegt');
    await expect(page.locator('.alert-success')).toContainText('0 Klassen neu erstellt');
    await expect(page.locator('.alert-success')).toContainText('3 bereits vorhanden');
  });

  test('a file without the required columns is rejected', async ({ page }) => {
    await upload(page, 'Vorname;Nachname\nAnna;Meier\n');

    await expect(page.locator('.alert-error')).toContainText('Spalten fehlen');
    await expect(page.locator('.alert-error')).toContainText('Klassenname');
  });
});
