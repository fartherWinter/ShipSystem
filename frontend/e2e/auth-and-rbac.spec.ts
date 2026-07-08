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

  test('viewer dashboard shows aggregated counts and latest alarm details', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/dashboard');

    await expect(page.locator('.ant-statistic')).toHaveCount(3);
    await expect(page.locator('.ant-statistic-content-value').nth(0)).toContainText('1');
    await expect(page.locator('.ant-statistic-content-value').nth(1)).toContainText('1');
    await expect(page.locator('.ant-statistic-content-value').nth(2)).toContainText('1');
    await expect(page.locator('tbody tr').first()).toContainText('SPEEDING');
    await expect(page.locator('tbody tr').first()).toContainText('Patrol ship exceeded the configured speed corridor.');
  });

  test('viewer is redirected away from rbac page', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/rbac');

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.getByText('权限管理')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: '首页态势' })).toBeVisible();
  });

  test('admin can create edit and delete a ship from the table', async ({ page }) => {
    await preloadSavedUser(page, 'admin');
    await installApiMocks(page, 'admin');

    await page.goto('/ships');

    await page.locator('.page-toolbar .ant-btn-primary').click();
    const modal = page.locator('.ant-modal').last();
    await modal.locator('input').nth(0).fill('E2E Patrol 01');
    await modal.locator('input').nth(1).fill('987654321');
    await modal.locator('.ant-btn-primary').click();

    const createdRow = page.locator('tbody tr', { hasText: 'E2E Patrol 01' });
    await expect(createdRow).toBeVisible();

    await createdRow.locator('button').first().click();
    const editModal = page.locator('.ant-modal').last();
    await editModal.locator('input').nth(0).fill('E2E Patrol 01 Updated');
    await editModal.locator('.ant-btn-primary').click();

    const updatedRow = page.locator('tbody tr', { hasText: 'E2E Patrol 01 Updated' });
    await expect(updatedRow).toBeVisible();

    await updatedRow.locator('button').nth(1).click();
    await page.locator('.ant-popconfirm .ant-btn-primary').click();
    await expect(page.locator('tbody tr', { hasText: 'E2E Patrol 01 Updated' })).toHaveCount(0);
  });

  test('super admin can open rbac page and inspect user role menu tables', async ({ page }) => {
    await preloadSavedUser(page, 'super_admin');
    await installApiMocks(page, 'super_admin');

    await page.goto('/rbac');

    await expect(page).toHaveURL(/\/rbac$/);
    await expect(page.getByText('demo').first()).toBeVisible();
    await expect(page.getByText('super_admin').first()).toBeVisible();
    await expect(page.getByText('viewer').first()).toBeVisible();
    await expect(page.getByText('Dashboard').first()).toBeVisible();
    await expect(page.getByText('/dashboard').first()).toBeVisible();
  });

  test('logout clears saved user and returns to the login page', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/dashboard');
    await expect(page.getByText('Demo User')).toBeVisible();

    await page.locator('.app-header button').click();

    await expect(page.locator('input').first()).toBeVisible();
    await expect
      .poll(async () => page.evaluate(() => window.localStorage.getItem('shipsystem_user')))
      .toBeNull();
  });
});
