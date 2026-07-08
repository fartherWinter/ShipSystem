import { readFile } from 'node:fs/promises';
import { expect, test } from '@playwright/test';
import { installApiMocks, preloadSavedUser } from './fixtures';

test.describe('dispatch and battle replay flows', () => {
  test('dispatcher can create a dispatch event and advance its status', async ({ page }) => {
    await preloadSavedUser(page, 'dispatcher');
    await installApiMocks(page, 'dispatcher');

    await page.goto('/dispatch');

    await page.locator('.page-toolbar .ant-btn-primary').click();
    const modal = page.locator('.ant-modal').last();
    await modal.locator('input').nth(0).fill('Dispatch E2E 01');
    await modal.locator('textarea').fill('Create and advance dispatch event');
    await modal.locator('.ant-btn-primary').click();

    const row = page.locator('tbody tr', { hasText: 'Dispatch E2E 01' });
    await expect(row).toBeVisible();
    await expect(row).toContainText('NEW');

    await row.locator('.ant-select').click();
    await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({ hasText: 'PROCESSING' }).click();
    await expect(row).toContainText('PROCESSING');
  });

  test('dispatcher can start and stop a live battle session', async ({ page }) => {
    await preloadSavedUser(page, 'dispatcher');
    await installApiMocks(page, 'dispatcher');

    await page.goto('/battle');

    const startButton = page.locator('.battle-side-panel .ant-btn-primary').first();
    await expect(startButton).toBeVisible();
    await startButton.click();

    await expect(page.getByText('session-live-302')).toBeVisible();
    await expect(page.getByText('running').first()).toBeVisible();

    const stopButton = page.locator('.battle-side-panel .ant-btn-dangerous').first();
    await expect(stopButton).toBeEnabled();
    await stopButton.click();

    await expect(page.getByText('stopped').first()).toBeVisible();
  });

  test('battle replay loads a session and can step to the next snapshot', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/battle');
    await page.locator('.ant-tabs-tab').nth(1).click();

    const sessionButton = page.getByRole('button', { name: /Replay Session Alpha/i });
    await expect(sessionButton).toBeVisible();
    await sessionButton.click();

    await expect(page.getByText('Blue radar contact established').first()).toBeVisible();
    const slider = page.getByRole('slider');
    await expect(slider).toHaveAttribute('aria-valuemax', '2');
    await expect(slider).toHaveAttribute('aria-valuenow', '0');

    await page.locator('button:has(svg.lucide-chevron-right)').click();
    await expect(slider).toHaveAttribute('aria-valuenow', '1');
    await expect(page.getByText('Blue escort launched missile volley').first()).toBeVisible();
  });

  test('battle replay can export report downloads in json csv and html', async ({ page }, testInfo) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/battle');
    await page.locator('.ant-tabs-tab').nth(1).click();

    const sessionButton = page.getByRole('button', { name: /Replay Session Alpha/i });
    await expect(sessionButton).toBeVisible();
    await sessionButton.click();
    await expect(page.getByRole('button', { name: /JSON/i })).toBeVisible();

    const cases = [
      {
        trigger: /JSON/i,
        filename: 'battle-session-alpha-report.json',
        expectedSnippet: '"sessionId": "session-alpha"',
      },
      {
        trigger: /CSV/i,
        filename: 'battle-session-alpha-report.csv',
        expectedSnippet: 'summary,session_id,session-alpha',
      },
      {
        trigger: /HTML/i,
        filename: 'battle-session-alpha-report.html',
        expectedSnippet: '<!doctype html>',
      },
    ] as const;

    for (const item of cases) {
      const [download] = await Promise.all([
        page.waitForEvent('download'),
        page.getByRole('button', { name: item.trigger }).click(),
      ]);

      expect(download.suggestedFilename()).toBe(item.filename);

      const outputPath = testInfo.outputPath(item.filename);
      await download.saveAs(outputPath);
      const text = await readFile(outputPath, 'utf8');
      expect(text).toContain(item.expectedSnippet);
    }
  });
});
