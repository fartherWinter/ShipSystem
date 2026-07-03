import type { Page, Route } from '@playwright/test';

type RoleCode = 'super_admin' | 'admin' | 'dispatcher' | 'viewer';

type MockUser = {
  id: number;
  username: string;
  displayName: string;
  status: string;
  role: {
    id: number;
    name: string;
    code: RoleCode;
  };
};

const baseUser: MockUser = {
  id: 1,
  username: 'demo',
  displayName: 'Demo User',
  status: 'active',
  role: {
    id: 2,
    name: '值班员',
    code: 'viewer',
  },
};

export async function installApiMocks(page: Page, roleCode: RoleCode = 'viewer'): Promise<void> {
  const user: MockUser = {
    ...baseUser,
    role: {
      ...baseUser.role,
      code: roleCode,
      name: roleName(roleCode),
    },
  };
  let nextShipID = 102;
  let nextDispatchEventID = 202;
  const ships = [
    {
      id: 101,
      name: '海巡01',
      mmsi: '123456789',
      shipType: 'patrol',
      flag: 'CN',
      lengthM: 120,
      widthM: 18,
      status: 'active',
      createdAt: '2026-07-01T00:00:00Z',
      updatedAt: '2026-07-01T00:00:00Z',
    },
  ];
  const alarms = [
    {
      id: 401,
      shipId: 101,
      ship: ships[0],
      type: 'SPEEDING',
      level: 'CRITICAL',
      title: 'Speed breach alert',
      message: 'Patrol ship exceeded the configured speed corridor.',
      longitude: 121.5123,
      latitude: 31.2245,
      status: 'OPEN',
      ackBy: null,
      ackAt: null,
      createdAt: '2026-07-02T02:00:00Z',
      updatedAt: '2026-07-02T02:00:00Z',
    },
  ];
  const tracksByShip = {
    101: [
      {
        id: 501,
        shipId: 101,
        longitude: 121.49,
        latitude: 31.23,
        speedKnots: 16.5,
        course: 42,
        reportedAt: '2026-07-02T08:00:00Z',
        createdAt: '2026-07-02T08:00:00Z',
      },
      {
        id: 502,
        shipId: 101,
        longitude: 121.505,
        latitude: 31.236,
        speedKnots: 17.1,
        course: 48,
        reportedAt: '2026-07-02T08:05:00Z',
        createdAt: '2026-07-02T08:05:00Z',
      },
    ],
  };
  const dispatchEvents = [
    {
      id: 201,
      title: 'Harbor Patrol Follow-up',
      description: 'Confirm intercept route and handoff window.',
      status: 'NEW',
      priority: 'normal',
      shipId: 101,
      createdAt: '2026-07-02T01:00:00Z',
      updatedAt: '2026-07-02T01:00:00Z',
    },
  ];
  const battleScenario = {
    code: 'open-water-duel',
    name: 'Open Water Duel',
    description: 'Mock replay session for browser regression coverage.',
    originLongitude: 121.49,
    originLatitude: 31.23,
    blueUnits: 1,
    redUnits: 1,
    radarRangeKm: 80,
    weaponRangeKm: 45,
  };
  const battleSessions = [
    {
      id: 301,
      sessionId: 'session-alpha',
      name: 'Replay Session Alpha',
      scenarioCode: battleScenario.code,
      status: 'blue_victory',
      startedAt: '2026-07-02T09:00:00Z',
      stoppedAt: '2026-07-02T09:03:00Z',
      lastScanAt: '2026-07-02T09:03:00Z',
      createdAt: '2026-07-02T09:00:00Z',
      updatedAt: '2026-07-02T09:03:00Z',
    },
  ];
  const battleTimelineBySession = {
    'session-alpha': [
      {
        tick: 0,
        snapshotTime: '2026-07-02T09:00:00Z',
        eventCount: 1,
        events: [
          {
            type: 'RADAR_CONTACT',
            severity: 'INFO',
            message: 'Blue radar contact established',
            sourceUnitId: 'blue-01',
            targetUnitId: 'red-01',
          },
        ],
      },
      {
        tick: 1,
        snapshotTime: '2026-07-02T09:01:00Z',
        eventCount: 1,
        events: [
          {
            type: 'WEAPON_RELEASE',
            severity: 'WARN',
            message: 'Blue escort launched missile volley',
            sourceUnitId: 'blue-01',
            targetUnitId: 'red-01',
          },
        ],
      },
      {
        tick: 2,
        snapshotTime: '2026-07-02T09:02:00Z',
        eventCount: 1,
        events: [
          {
            type: 'TARGET_DESTROYED',
            severity: 'CRITICAL',
            message: 'Red hull breach confirmed',
            sourceUnitId: 'blue-01',
            targetUnitId: 'red-01',
          },
        ],
      },
    ],
  };
  const battleSnapshotsBySession = {
    'session-alpha': [
      createBattleSnapshot(0, '2026-07-02T09:00:00Z'),
      createBattleSnapshot(1, '2026-07-02T09:01:00Z'),
      createBattleSnapshot(2, '2026-07-02T09:02:00Z'),
    ],
  };
  const battleReportsBySession = {
    'session-alpha': {
      session: battleSessions[0],
      winner: 'blue',
      firedCount: 2,
      hitCount: 1,
      destroyedCount: 1,
      damageRanking: [
        {
          unitId: 'red-01',
          name: 'Red Frigate',
          side: 'red',
          maxHp: 100,
          hp: 0,
          damageTaken: 100,
          status: 'destroyed',
        },
        {
          unitId: 'blue-01',
          name: 'Blue Escort',
          side: 'blue',
          maxHp: 100,
          hp: 92,
          damageTaken: 8,
          status: 'active',
        },
      ],
      keyEvents: [
        {
          id: 1,
          sessionId: 'session-alpha',
          eventId: 'event-2',
          type: 'TARGET_DESTROYED',
          severity: 'CRITICAL',
          message: 'Red hull breach confirmed',
          sourceUnitId: 'blue-01',
          targetUnitId: 'red-01',
          longitude: 121.53,
          latitude: 31.235,
          occurredAt: '2026-07-02T09:02:00Z',
          createdAt: '2026-07-02T09:02:00Z',
        },
      ],
    },
  };

  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const method = route.request().method();

    if (path === '/api/v1/auth/login' && method === 'POST') {
      await json(route, 200, {
        token: 'mock-token',
        user,
        menus: [],
      });
      return;
    }

    if (path === '/api/v1/auth/logout' && method === 'POST') {
      await route.fulfill({ status: 204, body: '' });
      return;
    }

    if (path === '/api/v1/ships' && method === 'GET') {
      await json(route, 200, {
        items: ships,
        total: ships.length,
      });
      return;
    }

    if (path === '/api/v1/ships' && method === 'POST') {
      const payload = route.request().postDataJSON() as Record<string, unknown>;
      const created = {
        id: nextShipID++,
        name: String(payload.name ?? ''),
        mmsi: String(payload.mmsi ?? ''),
        shipType: String(payload.shipType ?? ''),
        flag: String(payload.flag ?? 'CN'),
        lengthM: Number(payload.lengthM ?? 0),
        widthM: Number(payload.widthM ?? 0),
        status: String(payload.status ?? 'active'),
        createdAt: '2026-07-02T00:00:00Z',
        updatedAt: '2026-07-02T00:00:00Z',
      };
      ships.unshift(created);
      await json(route, 201, created);
      return;
    }

    if (/^\/api\/v1\/ships\/\d+$/.test(path) && method === 'PUT') {
      const shipID = Number(path.split('/').pop());
      const payload = route.request().postDataJSON() as Record<string, unknown>;
      const target = ships.find((item) => item.id === shipID);
      if (!target) {
        await json(route, 404, { message: 'ship does not exist' });
        return;
      }
      Object.assign(target, {
        name: String(payload.name ?? target.name),
        mmsi: String(payload.mmsi ?? target.mmsi),
        shipType: String(payload.shipType ?? target.shipType),
        flag: String(payload.flag ?? target.flag),
        lengthM: Number(payload.lengthM ?? target.lengthM),
        widthM: Number(payload.widthM ?? target.widthM),
        status: String(payload.status ?? target.status),
        updatedAt: '2026-07-03T00:00:00Z',
      });
      await json(route, 200, target);
      return;
    }

    if (/^\/api\/v1\/ships\/\d+$/.test(path) && method === 'DELETE') {
      const shipID = Number(path.split('/').pop());
      const index = ships.findIndex((item) => item.id === shipID);
      if (index === -1) {
        await json(route, 404, { message: 'ship does not exist' });
        return;
      }
      ships.splice(index, 1);
      await route.fulfill({ status: 204, body: '' });
      return;
    }

    if (/^\/api\/v1\/ships\/\d+\/tracks$/.test(path) && method === 'GET') {
      const shipID = Number(path.split('/')[4]);
      const items = tracksByShip[shipID as keyof typeof tracksByShip] ?? [];
      await json(route, 200, { items });
      return;
    }

    if (path === '/api/v1/alarms' && method === 'GET') {
      const status = url.searchParams.get('status');
      const items = status ? alarms.filter((item) => item.status === status) : alarms;
      await json(route, 200, { items, total: items.length });
      return;
    }

    if (/^\/api\/v1\/alarms\/\d+\/ack$/.test(path) && method === 'PUT') {
      const alarmID = Number(path.split('/')[4]);
      const target = alarms.find((item) => item.id === alarmID);
      if (!target) {
        await json(route, 404, { message: 'alarm does not exist' });
        return;
      }
      Object.assign(target, {
        status: 'ACKED',
        ackBy: user.id,
        ackAt: '2026-07-03T01:00:00Z',
        updatedAt: '2026-07-03T01:00:00Z',
      });
      await json(route, 200, target);
      return;
    }

    if (path === '/api/v1/dispatch-events' && method === 'GET') {
      const status = url.searchParams.get('status');
      const items = status ? dispatchEvents.filter((item) => item.status === status) : dispatchEvents;
      await json(route, 200, { items, total: items.length });
      return;
    }

    if (path === '/api/v1/dispatch-events' && method === 'POST') {
      const payload = route.request().postDataJSON() as Record<string, unknown>;
      const created = {
        id: nextDispatchEventID++,
        title: String(payload.title ?? ''),
        description: String(payload.description ?? ''),
        status: String(payload.status ?? 'NEW'),
        priority: String(payload.priority ?? 'normal'),
        shipId: payload.shipId ? Number(payload.shipId) : null,
        createdAt: '2026-07-03T00:00:00Z',
        updatedAt: '2026-07-03T00:00:00Z',
      };
      dispatchEvents.unshift(created);
      await json(route, 201, created);
      return;
    }

    if (/^\/api\/v1\/dispatch-events\/\d+\/status$/.test(path) && method === 'PUT') {
      const dispatchID = Number(path.split('/')[4]);
      const payload = route.request().postDataJSON() as Record<string, unknown>;
      const target = dispatchEvents.find((item) => item.id === dispatchID);
      if (!target) {
        await json(route, 404, { message: 'dispatch event does not exist' });
        return;
      }
      Object.assign(target, {
        status: String(payload.status ?? target.status),
        updatedAt: '2026-07-03T00:10:00Z',
      });
      await json(route, 200, target);
      return;
    }

    if (path === '/api/v1/rbac/users' && method === 'GET') {
      await json(route, 200, { items: [user] });
      return;
    }

    if (path === '/api/v1/rbac/roles' && method === 'GET') {
      await json(route, 200, {
        items: [
          { id: 1, code: 'super_admin', name: '超级管理员', description: 'super admin' },
          { id: 2, code: 'viewer', name: '值班员', description: 'viewer' },
        ],
      });
      return;
    }

    if (path === '/api/v1/rbac/menus' && method === 'GET') {
      await json(route, 200, { items: [] });
      return;
    }

    if (path === '/api/v1/battle/scenarios' && method === 'GET') {
      await json(route, 200, { items: [battleScenario] });
      return;
    }

    if (path === '/api/v1/battle/sessions' && method === 'GET') {
      await json(route, 200, { items: battleSessions, total: battleSessions.length });
      return;
    }

    if (/^\/api\/v1\/battle\/sessions\/[^/]+\/timeline$/.test(path) && method === 'GET') {
      const sessionId = decodeURIComponent(path.split('/')[5]);
      const items = battleTimelineBySession[sessionId as keyof typeof battleTimelineBySession];
      if (!items) {
        await json(route, 404, { message: 'battle timeline does not exist' });
        return;
      }
      await json(route, 200, { items });
      return;
    }

    if (/^\/api\/v1\/battle\/sessions\/[^/]+\/snapshots$/.test(path) && method === 'GET') {
      const sessionId = decodeURIComponent(path.split('/')[5]);
      const items = battleSnapshotsBySession[sessionId as keyof typeof battleSnapshotsBySession];
      if (!items) {
        await json(route, 404, { message: 'battle snapshots do not exist' });
        return;
      }
      await json(route, 200, { items });
      return;
    }

    if (/^\/api\/v1\/battle\/sessions\/[^/]+\/report$/.test(path) && method === 'GET') {
      const sessionId = decodeURIComponent(path.split('/')[5]);
      const item = battleReportsBySession[sessionId as keyof typeof battleReportsBySession];
      if (!item) {
        await json(route, 404, { message: 'battle report does not exist' });
        return;
      }
      await json(route, 200, item);
      return;
    }

    if (path === '/api/v1/analytics/simulate/start' && method === 'POST') {
      await json(route, 200, {
        success: true,
        requestId: 'mock-analytics-start',
        message: 'simulator started',
      });
      return;
    }

    await json(route, 200, {});
  });

  await installMockWebSocket(page);
}

function createBattleSnapshot(tick: number, snapshotTime: string) {
  const redDestroyed = tick >= 2;
  const redLongitude = 121.55 - tick * 0.01;
  const redLatitude = 31.24 - tick * 0.002;
  return {
    id: tick + 1,
    sessionId: 'session-alpha',
    tick,
    snapshotTime,
    units: [
      {
        sessionId: 'session-alpha',
        unitId: 'blue-01',
        shipId: 1,
        name: 'Blue Escort',
        side: 'blue',
        hp: 92,
        maxHp: 100,
        radarRangeKm: 80,
        weaponRangeKm: 45,
        cooldownSeconds: 8,
        longitude: 121.49 + tick * 0.01,
        latitude: 31.23 + tick * 0.002,
        course: 45,
        speedKnots: 24,
        status: 'active',
        createdAt: '2026-07-02T09:00:00Z',
        updatedAt: snapshotTime,
      },
      {
        sessionId: 'session-alpha',
        unitId: 'red-01',
        shipId: 2,
        name: 'Red Frigate',
        side: 'red',
        hp: redDestroyed ? 0 : 80 - tick * 10,
        maxHp: 100,
        radarRangeKm: 75,
        weaponRangeKm: 42,
        cooldownSeconds: 10,
        longitude: redLongitude,
        latitude: redLatitude,
        course: 225,
        speedKnots: 20,
        status: redDestroyed ? 'destroyed' : 'active',
        createdAt: '2026-07-02T09:00:00Z',
        updatedAt: snapshotTime,
      },
    ],
    projectiles:
      tick === 0
        ? []
        : [
            {
              sessionId: 'session-alpha',
              projectileId: `proj-${tick}`,
              sourceUnitId: 'blue-01',
              targetUnitId: 'red-01',
              side: 'blue',
              longitude: 121.5 + tick * 0.01,
              latitude: 31.232 + tick * 0.001,
              speedKmH: 900,
              status: redDestroyed ? 'hit' : 'flying',
              createdAt: '2026-07-02T09:01:00Z',
              updatedAt: snapshotTime,
            },
          ],
    radarTargets: [
      {
        sessionId: 'session-alpha',
        radarId: 'blue-radar-01',
        targetId: 'red-01',
        side: 'red',
        longitude: redLongitude,
        latitude: redLatitude,
        course: 225,
        speedKnots: 20,
        confidence: 0.97,
        detected: true,
        scanTime: snapshotTime,
        createdAt: snapshotTime,
      },
    ],
    events:
      tick === 0
        ? [
            {
              id: 1,
              sessionId: 'session-alpha',
              eventId: 'event-0',
              type: 'RADAR_CONTACT',
              severity: 'INFO',
              message: 'Blue radar contact established',
              sourceUnitId: 'blue-01',
              targetUnitId: 'red-01',
              longitude: redLongitude,
              latitude: redLatitude,
              occurredAt: snapshotTime,
              createdAt: snapshotTime,
            },
          ]
        : tick === 1
          ? [
              {
                id: 2,
                sessionId: 'session-alpha',
                eventId: 'event-1',
                type: 'WEAPON_RELEASE',
                severity: 'WARN',
                message: 'Blue escort launched missile volley',
                sourceUnitId: 'blue-01',
                targetUnitId: 'red-01',
                longitude: 121.51,
                latitude: 31.233,
                occurredAt: snapshotTime,
                createdAt: snapshotTime,
              },
            ]
          : [
              {
                id: 3,
                sessionId: 'session-alpha',
                eventId: 'event-2',
                type: 'TARGET_DESTROYED',
                severity: 'CRITICAL',
                message: 'Red hull breach confirmed',
                sourceUnitId: 'blue-01',
                targetUnitId: 'red-01',
                longitude: redLongitude,
                latitude: redLatitude,
                occurredAt: snapshotTime,
                createdAt: snapshotTime,
              },
            ],
  };
}

export async function preloadSavedUser(page: Page, roleCode: RoleCode = 'viewer'): Promise<void> {
  const user: MockUser = {
    ...baseUser,
    role: {
      ...baseUser.role,
      code: roleCode,
      name: roleName(roleCode),
    },
  };
  await page.addInitScript((savedUser) => {
    window.localStorage.setItem('shipsystem_user', JSON.stringify(savedUser));
  }, user);
}

async function installMockWebSocket(page: Page): Promise<void> {
  await page.addInitScript(() => {
    class MockWebSocket {
      static OPEN = 1;
      static CLOSED = 3;
      readyState = MockWebSocket.OPEN;
      url: string;
      onopen: ((event: Event) => void) | null = null;
      onclose: ((event: CloseEvent) => void) | null = null;
      onerror: ((event: Event) => void) | null = null;
      onmessage: ((event: MessageEvent<string>) => void) | null = null;

      constructor(url: string) {
        this.url = url;
        window.setTimeout(() => {
          this.onopen?.(new Event('open'));
        }, 0);
      }

      close() {
        this.readyState = MockWebSocket.CLOSED;
        this.onclose?.(new CloseEvent('close'));
      }

      send() {}
    }

    Object.defineProperty(window, 'WebSocket', {
      configurable: true,
      writable: true,
      value: MockWebSocket,
    });
  });
}

function roleName(roleCode: RoleCode): string {
  switch (roleCode) {
    case 'super_admin':
      return '超级管理员';
    case 'admin':
      return '管理员';
    case 'dispatcher':
      return '调度员';
    default:
      return '值班员';
  }
}

async function json(route: Route, status: number, body: unknown): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json; charset=utf-8',
    body: JSON.stringify(body),
  });
}
