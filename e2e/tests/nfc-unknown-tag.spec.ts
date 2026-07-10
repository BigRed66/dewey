import { test, expect } from '@playwright/test';

// Regression test: the "unknown tag" modal's "＋ Buch anlegen" link used to
// go to a blank /books/new, throwing away the tag ID the reader had just
// scanned — forcing the user to scan the same physical book again. The link
// now carries the tag ID, and /books/new prefills + reveals the manual entry
// form straight away instead of requiring an extra "Weiter" click.

test('opening "＋ Buch anlegen" with a known tag id skips re-scanning', async ({ page }) => {
  await page.goto('/books/new?nfc_tag_id=FRESH-TAG-123');

  await expect(page.locator('#nfc_tag_id')).toHaveValue('FRESH-TAG-123');
  // The manual entry form must already be visible, not gated behind
  // clicking "Weiter" again.
  await expect(page.locator('#title')).toBeVisible();
});

test('the unknown-tag modal link includes the scanned uid', async () => {
  // Static check that the link-building code embeds the scanned uid rather
  // than a bare "/books/new" — the actual modal only ever opens from a real
  // SSE tag event, which needs hardware, so this asserts the fix in place
  // rather than driving the SSE path end-to-end.
  const fs = await import('fs');
  const html = fs.readFileSync('internal/web/templates/layout.html', 'utf-8');
  expect(html).toContain('href="/books/new?nfc_tag_id=\' + encodeURIComponent(uid)');
});
