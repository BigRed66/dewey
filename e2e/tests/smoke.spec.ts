import { test, expect } from '@playwright/test';

test('books page lists seeded books', async ({ page }) => {
  await page.goto('/books');
  await expect(page.getByRole('heading', { name: 'Bücher' })).toBeVisible();
  await expect(page.getByText('Harry Potter und der Stein der Weisen')).toBeVisible();
});

test('favicon is bundled and served', async ({ page, request }) => {
  await page.goto('/books');
  await expect(page.locator('link[rel="icon"][type="image/svg+xml"]')).toHaveAttribute('href', '/favicon.svg');

  const ico = await request.get('/favicon.ico');
  expect(ico.ok()).toBe(true);
  expect(ico.headers()['content-type']).toBe('image/x-icon');

  const svg = await request.get('/favicon.svg');
  expect(svg.ok()).toBe(true);
  expect(svg.headers()['content-type']).toBe('image/svg+xml');
});
