import { expect, test } from '@playwright/test';
import { installApiMocks, preloadSavedUser } from './fixtures';

test.describe('monitor alarms and tracks flows', () => {
  test('viewer can acknowledge an alarm from the table', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/alarms');

    const row = page.locator('tbody tr', { hasText: 'Speed breach alert' });
    await expect(row).toBeVisible();
    await expect(row).toContainText('OPEN');

    await row.getByRole('button').click();
    await expect(row).toContainText('ACKED');
    await expect(row.getByRole('button')).toHaveCount(0);
  });

  test('viewer can query ship tracks and render the track table', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/tracks');
    await expect(page.locator('.track-map .monitor-map')).toBeVisible();

    await page.locator('.page-toolbar .ant-btn-primary').click();

    await expect(page.locator('tbody tr')).toHaveCount(2);
    await expect(page.locator('tbody tr').first()).toContainText('2026-07-02T08:00:00Z');
    await expect(page.locator('tbody tr').last()).toContainText('2026-07-02T08:05:00Z');
  });

  test('viewer can open monitor page and trigger simulator start', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/monitor');

    await expect(page.locator('.map-panel .monitor-map')).toBeVisible();
    await expect(page.getByText('connected')).toBeVisible();

    const startButton = page.locator('.side-panel .ant-btn-primary');
    await startButton.click();
    await expect(startButton).not.toHaveClass(/ant-btn-loading/);
    await expect(page.locator('.side-panel .ant-alert')).toHaveCount(0);
  });
});
