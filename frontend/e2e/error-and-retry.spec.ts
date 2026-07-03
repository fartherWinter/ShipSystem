import type { Page } from '@playwright/test';
import { expect, test } from '@playwright/test';
import { installApiMocks, preloadSavedUser } from './fixtures';

test.describe('error and retry recovery', () => {
  test('dashboard recovers after the first summary request fails', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await failOnceAndFallback(page, /\/api\/v1\/ships\?keyword=.*$/, 'dashboard ship summary failed once');

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

    await failOnceAndFallback(page, /\/api\/v1\/dispatch-events\?status=.*$/, 'dispatch list failed once');

    await page.goto('/dispatch');

    const alert = page.locator('.ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('dispatch list failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.locator('tbody tr', { hasText: 'Harbor Patrol Follow-up' })).toBeVisible();
  });

  test('ships page recovers after the first list fetch fails', async ({ page }) => {
    await preloadSavedUser(page, 'admin');
    await installApiMocks(page, 'admin');

    await failOnceAndFallback(page, /\/api\/v1\/ships\?keyword=.*$/, 'ships list failed once');

    await page.goto('/ships');

    const alert = page.locator('.ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('ships list failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.locator('tbody tr', { hasText: '123456789' })).toBeVisible();
  });

  test('tracks query recovers after the first fetch fails', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await failOnceAndFallback(page, /\/api\/v1\/ships\/101\/tracks$/, 'tracks query failed once');

    await page.goto('/tracks');
    await expect(page.locator('.track-map .monitor-map')).toBeVisible();

    await page.locator('.page-toolbar .ant-btn-primary').click();

    const alert = page.locator('.ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('tracks query failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.locator('tbody tr')).toHaveCount(2);
  });

  test('monitor simulator start recovers after the first request fails', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await failOnceAndFallback(
      page,
      /\/api\/v1\/analytics\/simulate\/start$/,
      'monitor simulator start failed once',
      'POST',
    );

    await page.goto('/monitor');
    await expect(page.getByText('connected')).toBeVisible();

    const startButton = page.locator('.side-panel .ant-btn-primary');
    await startButton.click();

    const alert = page.locator('.side-panel .ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('monitor simulator start failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(startButton).not.toHaveClass(/ant-btn-loading/);
  });

  test('battle live tab recovers after the first scenario fetch fails', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await failOnceAndFallback(page, /\/api\/v1\/battle\/scenarios$/, 'battle scenarios failed once');

    await page.goto('/battle');

    const alert = page.locator('.battle-side-panel .ant-alert').first();
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('battle scenarios failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.getByText('Mock replay session for browser regression coverage.')).toBeVisible();
  });

  test('battle replay tab recovers after the first detail fetch fails', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await failOnceAndFallback(
      page,
      /\/api\/v1\/battle\/sessions\/session-alpha\/timeline$/,
      'battle replay timeline failed once',
    );

    await page.goto('/battle');
    await page.locator('.ant-tabs-tab').nth(1).click();

    const alert = page.locator('.battle-side-panel .ant-alert').last();
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('battle replay timeline failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.getByText('Blue radar contact established').first()).toBeVisible();
    await expect(page.getByRole('slider')).toHaveAttribute('aria-valuemax', '2');
  });

  test('alarms page recovers after the first list fetch fails', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await failOnceAndFallback(page, /\/api\/v1\/alarms\?status=.*$/, 'alarms list failed once');

    await page.goto('/alarms');

    const alert = page.locator('.ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('alarms list failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.locator('tbody tr', { hasText: 'Speed breach alert' })).toBeVisible();
  });

  test('rbac page recovers after the first users fetch fails', async ({ page }) => {
    await preloadSavedUser(page, 'super_admin');
    await installApiMocks(page, 'super_admin');

    await failOnceAndFallback(page, /\/api\/v1\/rbac\/users$/, 'rbac users failed once');

    await page.goto('/rbac');

    const alert = page.locator('.ant-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('rbac users failed once');

    await alert.locator('.ant-btn').click();

    await expect(alert).toHaveCount(0);
    await expect(page.getByText('demo').first()).toBeVisible();
    await expect(page.getByText('Dashboard').first()).toBeVisible();
  });
});

async function failOnceAndFallback(page: Page, pattern: RegExp, message: string, method = 'GET'): Promise<void> {
  let failed = false;
  await page.route(pattern, async (route) => {
    if (route.request().method() !== method) {
      await route.fallback();
      return;
    }
    if (!failed) {
      failed = true;
      await route.fulfill({
        status: 500,
        contentType: 'application/json; charset=utf-8',
        body: JSON.stringify({ message }),
      });
      return;
    }
    await route.fallback();
  });
}
