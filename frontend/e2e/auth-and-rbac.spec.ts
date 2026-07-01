import { expect, test } from '@playwright/test';
import { installApiMocks, preloadSavedUser } from './fixtures';

test.describe('auth and role routing', () => {
  test('login succeeds and dashboard renders', async ({ page }) => {
    await installApiMocks(page, 'viewer');

    await page.goto('/');
    await page.locator('input').nth(1).press('Enter');

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.getByText('Demo User')).toBeVisible();
    const shipCountCard = page.locator('.ant-statistic').first();
    await expect(shipCountCard).toContainText('船舶数量');
    await expect(shipCountCard).toContainText('1');
  });

  test('viewer is redirected away from rbac page', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/rbac');

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.getByText('权限管理')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: '首页态势' })).toBeVisible();
  });
});
