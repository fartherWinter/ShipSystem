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
});
