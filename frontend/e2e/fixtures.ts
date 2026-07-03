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

    if (path === '/api/v1/alarms' && method === 'GET') {
      await json(route, 200, { items: [], total: 0 });
      return;
    }

    if (path === '/api/v1/dispatch-events' && method === 'GET') {
      await json(route, 200, { items: [], total: 0 });
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
      await json(route, 200, { items: [] });
      return;
    }

    if (path === '/api/v1/battle/sessions' && method === 'GET') {
      await json(route, 200, { items: [], total: 0 });
      return;
    }

    await json(route, 200, {});
  });

  await installMockWebSocket(page);
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
