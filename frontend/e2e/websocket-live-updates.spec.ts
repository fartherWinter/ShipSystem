import { expect, test } from '@playwright/test';
import { emitMockWsMessage, installApiMocks, preloadSavedUser } from './fixtures';

test.describe('websocket live updates', () => {
  test('dispatcher dispatch page applies dispatch_event_updated without manual refresh', async ({ page }) => {
    await preloadSavedUser(page, 'dispatcher');
    await installApiMocks(page, 'dispatcher');

    await page.goto('/dispatch');
    await expect(page.locator('tbody tr', { hasText: 'Harbor Patrol Follow-up' })).toBeVisible();

    await emitMockWsMessage(page, {
      type: 'dispatch_event_updated',
      data: {
        id: 999,
        title: 'WS Dispatch 01',
        description: 'Inserted from websocket event',
        status: 'PROCESSING',
        priority: 'high',
        shipId: 101,
        createdAt: '2026-07-03T09:00:00Z',
        updatedAt: '2026-07-03T09:05:00Z',
      },
    });

    const row = page.locator('tbody tr', { hasText: 'WS Dispatch 01' });
    await expect(row).toBeVisible();
    await expect(row).toContainText('PROCESSING');
    await expect(row).toContainText('high');
  });

  test('viewer monitor page applies ship_location_updated into latest locations list', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/monitor');
    await expect(page.getByText('connected')).toBeVisible();

    await emitMockWsMessage(page, {
      type: 'ship_location_updated',
      data: {
        id: 8801,
        shipId: 301,
        longitude: 123.4567,
        latitude: 32.1234,
        speedKnots: 18.2,
        course: 90,
        reportedAt: '2026-07-03T09:10:00Z',
        createdAt: '2026-07-03T09:10:00Z',
      },
    });

    await expect(page.getByText('#301 123.4567, 32.1234 / 18.2')).toBeVisible();
  });

  test('viewer battle page applies battle_state_updated without starting a local session', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/battle');
    await expect(page.getByText('connected')).toHaveCount(0);

    await emitMockWsMessage(page, {
      type: 'battle_state_updated',
      data: {
        sessionId: 'ws-battle-01',
        status: 'running',
        updatedAt: '2026-07-03T09:20:00Z',
        units: [
          {
            sessionId: 'ws-battle-01',
            unitId: 'blue-ws-01',
            shipId: 1,
            name: 'Blue WS Escort',
            side: 'blue',
            hp: 100,
            maxHp: 100,
            radarRangeKm: 80,
            weaponRangeKm: 45,
            cooldownSeconds: 8,
            longitude: 121.49,
            latitude: 31.23,
            course: 45,
            speedKnots: 24,
            status: 'active',
            createdAt: '2026-07-03T09:20:00Z',
            updatedAt: '2026-07-03T09:20:00Z',
          },
        ],
        projectiles: [],
        events: [],
        radarTargets: [],
      },
    });

    await expect(page.getByText('ws-battle-01')).toBeVisible();
    await expect(page.getByText('running').first()).toBeVisible();
  });
});
