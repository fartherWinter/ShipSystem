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
        items: [
          {
            id: 101,
            name: '海巡01',
            mmsi: '123456789',
            shipType: 'patrol',
            flag: 'CN',
            lengthM: 120,
            widthM: 18,
            status: 'active',
          },
        ],
        total: 1,
      });
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
