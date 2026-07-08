import { expect, test } from '@playwright/test';
import { closeMockWsConnections, emitMockWsMessage, installApiMocks, preloadSavedUser, readMockWsStats } from './fixtures';

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

    await emitMockWsMessage(page, {
      type: 'dispatch_event_updated',
      data: {
        id: 999,
        title: 'WS Dispatch 01',
        description: 'Replaced by websocket upsert',
        status: 'COMPLETED',
        priority: 'normal',
        shipId: 101,
        createdAt: '2026-07-03T09:00:00Z',
        updatedAt: '2026-07-03T09:15:00Z',
      },
    });

    await expect(page.locator('tbody tr', { hasText: 'WS Dispatch 01' })).toHaveCount(1);
    await expect(row).toContainText('COMPLETED');
    await expect(row).toContainText('normal');
    await expect(row).not.toContainText('PROCESSING');
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

    await emitMockWsMessage(page, {
      type: 'ship_location_updated',
      data: {
        id: 8802,
        shipId: 301,
        longitude: 123.5567,
        latitude: 32.2234,
        speedKnots: 19.4,
        course: 110,
        reportedAt: '2026-07-03T09:12:00Z',
        createdAt: '2026-07-03T09:12:00Z',
      },
    });

    await expect(page.locator('.side-panel .ant-list-item')).toHaveCount(1);
    await expect(page.getByText('#301 123.5567, 32.2234 / 19.4')).toBeVisible();
    await expect(page.getByText('#301 123.4567, 32.1234 / 18.2')).toHaveCount(0);
  });

  test('viewer monitor page reconnects after websocket disconnect', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/monitor');
    await expect(page.getByText('connected')).toBeVisible();
    const initialStats = await readMockWsStats(page);

    await closeMockWsConnections(page);
    await expect(page.getByText('disconnected')).toBeVisible();
    await expect(page.getByText('connected')).toBeVisible({ timeout: 3000 });
    await expect.poll(async () => {
      const stats = await readMockWsStats(page);
      return stats.open === 1 && stats.totalCreated > initialStats.totalCreated;
    }).toBe(true);

    await emitMockWsMessage(page, {
      type: 'ship_location_updated',
      data: {
        id: 8810,
        shipId: 404,
        longitude: 120.1234,
        latitude: 30.5678,
        speedKnots: 15.2,
        course: 75,
        reportedAt: '2026-07-03T09:30:00Z',
        createdAt: '2026-07-03T09:30:00Z',
      },
    });

    await expect(page.getByText('#404 120.1234, 30.5678 / 15.2')).toBeVisible();
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

  test('viewer battle page applies radar projectile and event websocket updates for the active session', async ({ page }) => {
    await preloadSavedUser(page, 'viewer');
    await installApiMocks(page, 'viewer');

    await page.goto('/battle');

    await emitMockWsMessage(page, {
      type: 'battle_state_updated',
      data: {
        sessionId: 'ws-battle-02',
        status: 'running',
        updatedAt: '2026-07-03T09:25:00Z',
        units: [
          {
            sessionId: 'ws-battle-02',
            unitId: 'blue-ws-02',
            shipId: 1,
            name: 'Blue WS 02',
            side: 'blue',
            hp: 96,
            maxHp: 100,
            radarRangeKm: 80,
            weaponRangeKm: 45,
            cooldownSeconds: 8,
            longitude: 121.49,
            latitude: 31.23,
            course: 45,
            speedKnots: 24,
            status: 'active',
            createdAt: '2026-07-03T09:25:00Z',
            updatedAt: '2026-07-03T09:25:00Z',
          },
          {
            sessionId: 'ws-battle-02',
            unitId: 'red-ws-02',
            shipId: 2,
            name: 'Red WS 02',
            side: 'red',
            hp: 84,
            maxHp: 100,
            radarRangeKm: 75,
            weaponRangeKm: 42,
            cooldownSeconds: 10,
            longitude: 121.56,
            latitude: 31.24,
            course: 225,
            speedKnots: 20,
            status: 'active',
            createdAt: '2026-07-03T09:25:00Z',
            updatedAt: '2026-07-03T09:25:00Z',
          },
        ],
        projectiles: [],
        events: [],
        radarTargets: [],
      },
    });

    const battleStats = page.locator('.battle-stat-grid .ant-statistic-content-value');
    await expect(battleStats.nth(0)).toContainText('0');
    await expect(battleStats.nth(1)).toContainText('0');

    await emitMockWsMessage(page, {
      type: 'radar_scan_updated',
      data: {
        sessionId: 'ws-battle-02',
        radarId: 'blue-radar-02',
        scanTime: '2026-07-03T09:25:20Z',
        targets: [
          {
            sessionId: 'ws-battle-02',
            radarId: 'blue-radar-02',
            targetId: 'red-ws-02',
            side: 'red',
            longitude: 121.56,
            latitude: 31.24,
            course: 225,
            speedKnots: 20,
            confidence: 0.97,
            detected: true,
            scanTime: '2026-07-03T09:25:20Z',
            createdAt: '2026-07-03T09:25:20Z',
          },
        ],
      },
    });
    await expect(battleStats.nth(0)).toContainText('1');

    await emitMockWsMessage(page, {
      type: 'projectile_updated',
      data: {
        sessionId: 'ws-battle-02',
        projectileId: 'ws-proj-02',
        sourceUnitId: 'blue-ws-02',
        targetUnitId: 'red-ws-02',
        side: 'blue',
        longitude: 121.53,
        latitude: 31.235,
        speedKmH: 900,
        status: 'flying',
        createdAt: '2026-07-03T09:25:30Z',
        updatedAt: '2026-07-03T09:25:30Z',
      },
    });
    await expect(battleStats.nth(1)).toContainText('1');

    await emitMockWsMessage(page, {
      type: 'projectile_updated',
      data: {
        sessionId: 'ws-battle-02',
        projectileId: 'ws-proj-02',
        sourceUnitId: 'blue-ws-02',
        targetUnitId: 'red-ws-02',
        side: 'blue',
        longitude: 121.56,
        latitude: 31.24,
        speedKmH: 900,
        status: 'hit',
        createdAt: '2026-07-03T09:25:30Z',
        updatedAt: '2026-07-03T09:25:35Z',
      },
    });
    await expect(battleStats.nth(1)).toContainText('0');

    await emitMockWsMessage(page, {
      type: 'battle_event_created',
      data: {
        id: 9801,
        sessionId: 'ws-battle-02',
        eventId: 'ws-event-02',
        type: 'WEAPON_RELEASE',
        severity: 'WARN',
        message: 'WS missile launch confirmed',
        sourceUnitId: 'blue-ws-02',
        targetUnitId: 'red-ws-02',
        longitude: 121.53,
        latitude: 31.235,
        occurredAt: '2026-07-03T09:25:40Z',
        createdAt: '2026-07-03T09:25:40Z',
      },
    });

    await expect(page.getByText('WS missile launch confirmed')).toBeVisible();

    await emitMockWsMessage(page, {
      type: 'battle_event_created',
      data: {
        id: 9802,
        sessionId: 'ws-battle-02',
        eventId: 'ws-event-02',
        type: 'WEAPON_RELEASE',
        severity: 'WARN',
        message: 'WS missile launch corrected',
        sourceUnitId: 'blue-ws-02',
        targetUnitId: 'red-ws-02',
        longitude: 121.54,
        latitude: 31.236,
        occurredAt: '2026-07-03T09:25:41Z',
        createdAt: '2026-07-03T09:25:41Z',
      },
    });

    await expect(page.getByText('WS missile launch corrected')).toBeVisible();
    await expect(page.getByText('WS missile launch confirmed')).toHaveCount(0);
  });
});
