import type {
  Alarm,
  AlarmPageResult,
  AnalyticsProxyResponse,
  BattleReport,
  BattleScenarioListResponse,
  BattleSession,
  BattleSessionCreateRequest,
  BattleSessionCreateResponse,
  BattleSessionPageResult,
  BattleSimulationStartRequest,
  BattleSimulationStopRequest,
  BattleSnapshotListResponse,
  BattleState,
  BattleTimelineResponse,
  DispatchEvent,
  DispatchEventCreateRequest,
  DispatchEventPageResult,
  DispatchStatusUpdateRequest,
  LocationReportResponse,
  LoginResponse,
  MenuListResponse,
  RoleListResponse,
  Ship,
  ShipLocationReportRequest,
  ShipPageResult,
  ShipUpsertRequest,
  SimulationStartRequest,
  TrackListResponse,
  UserListResponse,
} from '../types';

const API_BASE = import.meta.env.VITE_API_BASE ?? '/api/v1';

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set('Content-Type', 'application/json');
  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers, credentials: 'include' });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}));
    const requestId = typeof body.requestId === 'string' ? body.requestId : resp.headers.get('X-Request-ID');
    const message = typeof body.message === 'string' ? body.message : `请求失败：${resp.status}`;
    throw new Error(requestId ? `${message}（请求 ID：${requestId}）` : message);
  }
  if (resp.status === 204) {
    return undefined as T;
  }
  return resp.json() as Promise<T>;
}

type TrackQuery = {
  start?: string;
  end?: string;
};

type PaginationQuery = {
  page?: number;
  size?: number;
};

type PageResult<T> = {
  items: T[];
  total: number;
};

function appendPagination(params: URLSearchParams, pagination?: PaginationQuery) {
  if (!pagination) {
    return;
  }
  if (pagination.page !== undefined) {
    params.set('page', String(pagination.page));
  }
  if (pagination.size !== undefined) {
    params.set('size', String(pagination.size));
  }
}

export const api = {
  login: (username: string, password: string) =>
    request<LoginResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<void>('/auth/logout', { method: 'POST', body: JSON.stringify({}) }),
  ships: (keyword = '', pagination?: PaginationQuery) => {
    const params = new URLSearchParams();
    if (keyword.trim()) params.set('keyword', keyword.trim());
    appendPagination(params, pagination);
    const query = params.toString();
    return request<ShipPageResult>(`/ships${query ? `?${query}` : ''}`);
  },
  createShip: (ship: ShipUpsertRequest) => request<Ship>('/ships', { method: 'POST', body: JSON.stringify(ship) }),
  updateShip: (id: number, ship: ShipUpsertRequest) => request<Ship>(`/ships/${id}`, { method: 'PUT', body: JSON.stringify(ship) }),
  deleteShip: (id: number) => request<void>(`/ships/${id}`, { method: 'DELETE' }),
  reportLocation: (shipId: number, payload: ShipLocationReportRequest) =>
    request<LocationReportResponse>(`/ships/${shipId}/locations`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  tracks: (shipId: number, filters: TrackQuery = {}) => {
    const params = new URLSearchParams();
    if (filters.start) params.set('start', filters.start);
    if (filters.end) params.set('end', filters.end);
    const query = params.toString();
    return request<TrackListResponse>(`/ships/${shipId}/tracks${query ? `?${query}` : ''}`);
  },
  alarms: (status = '', pagination?: PaginationQuery) => {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    appendPagination(params, pagination);
    const query = params.toString();
    return request<AlarmPageResult>(`/alarms${query ? `?${query}` : ''}`);
  },
  ackAlarm: (id: number) => request<Alarm>(`/alarms/${id}/ack`, { method: 'PUT', body: JSON.stringify({}) }),
  dispatchEvents: (status = '', pagination?: PaginationQuery) => {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    appendPagination(params, pagination);
    const query = params.toString();
    return request<DispatchEventPageResult>(`/dispatch-events${query ? `?${query}` : ''}`);
  },
  createDispatchEvent: (event: DispatchEventCreateRequest) =>
    request<DispatchEvent>('/dispatch-events', { method: 'POST', body: JSON.stringify(event) }),
  updateDispatchStatus: (id: number, payload: DispatchStatusUpdateRequest) =>
    request<DispatchEvent>(`/dispatch-events/${id}/status`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  battleScenarios: () => request<BattleScenarioListResponse>('/battle/scenarios'),
  battleSessions: (pagination?: PaginationQuery) => {
    const params = new URLSearchParams();
    appendPagination(params, pagination);
    const query = params.toString();
    return request<BattleSessionPageResult>(`/battle/sessions${query ? `?${query}` : ''}`);
  },
  createBattleSession: (payload: BattleSessionCreateRequest) =>
    request<BattleSessionCreateResponse>('/battle/sessions', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  battleState: (sessionId: string) => request<BattleState>(`/battle/sessions/${encodeURIComponent(sessionId)}/state`),
  battleTimeline: (sessionId: string) => request<BattleTimelineResponse>(`/battle/sessions/${encodeURIComponent(sessionId)}/timeline`),
  battleSnapshots: (sessionId: string, from?: number, to?: number) => {
    const params = new URLSearchParams();
    if (from !== undefined) params.set('from', String(from));
    if (to !== undefined) params.set('to', String(to));
    const query = params.toString();
    return request<BattleSnapshotListResponse>(`/battle/sessions/${encodeURIComponent(sessionId)}/snapshots${query ? `?${query}` : ''}`);
  },
  battleReport: (sessionId: string) => request<BattleReport>(`/battle/sessions/${encodeURIComponent(sessionId)}/report`),
  stopBattleSession: (sessionId: string) =>
    request<BattleState>(`/battle/sessions/${encodeURIComponent(sessionId)}/stop`, {
      method: 'POST',
      body: JSON.stringify({}),
    }),
  users: () => request<UserListResponse>('/rbac/users'),
  roles: () => request<RoleListResponse>('/rbac/roles'),
  menus: () => request<MenuListResponse>('/rbac/menus'),
};

export async function getSimulatorStatus() {
  return request<AnalyticsProxyResponse>('/analytics/simulate/status');
}

export async function listAllShips(keyword = '') {
  return collectAllPages<Ship>((page, size) => api.ships(keyword, { page, size }));
}

export async function listAllDispatchEvents(status = '') {
  return collectAllPages<DispatchEvent>((page, size) => api.dispatchEvents(status, { page, size }));
}

export async function listAllBattleSessions() {
  return collectAllPages<BattleSession>((page, size) => api.battleSessions({ page, size }));
}

async function collectAllPages<T>(loadPage: (page: number, size: number) => Promise<PageResult<T>>) {
  const size = 100;
  let page = 1;
  let total = 0;
  const items: T[] = [];

  do {
    const data = await loadPage(page, size);
    items.push(...data.items);
    total = data.total;
    if (data.items.length === 0) {
      break;
    }
    page += 1;
  } while (items.length < total);

  return items;
}

export async function startSimulator(shipIds: number[]) {
  const payload: SimulationStartRequest = { shipIds };
  return request<AnalyticsProxyResponse>('/analytics/simulate/start', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export async function stopSimulator() {
  return request<AnalyticsProxyResponse>('/analytics/simulate/stop', {
    method: 'POST',
    body: JSON.stringify({}),
  });
}

export async function startBattleSimulator(payload: BattleSimulationStartRequest) {
  return request<AnalyticsProxyResponse>('/analytics/simulate/battle/start', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export async function stopBattleSimulator(sessionId: string) {
  const payload: BattleSimulationStopRequest = { sessionId };
  return request<AnalyticsProxyResponse>('/analytics/simulate/battle/stop', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
