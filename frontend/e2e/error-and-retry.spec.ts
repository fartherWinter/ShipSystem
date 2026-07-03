import { expect, test } from '@playwright/test';
import { installApiMocks, preloadSavedUser } from './fixtures';

test.describe('error and retry recovery', () => {
  test('dashboard recovers after the first summary request fails', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    let shipFailures = 0;
    await page.route(/\/api\/v1\/ships\?keyword=.*$/, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      if (shipFailures === 0) {
        shipFailures += 1;
        await route.fulfill({
          status: 500,
          contentType: 'application/json; charset=utf-8',
          body: JSON.stringify({ message: 'dashboard ship summary failed once' }),
        });
        return;
      }
      await route.fallback();
    });

    await page.goto('/dashboard');

    const alert = page.locator('.ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('dashboard ship summary failed once');

    await alert.locator('.ant-btn').click();

    const shipCountCard = page.locator('.ant-statistic').first();
    await expect(alert).toHaveCount(0);
    await expect(shipCountCard).toContainText('1');
    await expect(page.locator('tbody tr').first()).toContainText('SPEEDING');
  });

  test('dispatch list recovers after the first fetch fails', async ({ page }) => {
    await preloadSavedUser(page, 'dispatcher');
    await installApiMocks(page, 'dispatcher');

    let dispatchFailures = 0;
    await page.route(/\/api\/v1\/dispatch-events\?status=.*$/, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      if (dispatchFailures === 0) {
        dispatchFailures += 1;
        await route.fulfill({
          status: 500,
          contentType: 'application/json; charset=utf-8',
          body: JSON.stringify({ message: 'dispatch list failed once' }),
        });
        return;
      }
      await route.fallback();
    });

    await page.goto('/dispatch');

    const alert = page.locator('.ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('dispatch list failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.locator('tbody tr', { hasText: 'Harbor Patrol Follow-up' })).toBeVisible();
  });
});
