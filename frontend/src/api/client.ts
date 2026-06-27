import type {
  Alarm,
  AlarmPageResult,
  AnalyticsProxyResponse,
  BattleReport,
  BattleScenarioListResponse,
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
  LoginResponse,
  MenuListResponse,
  LocationReportResponse,
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
    const message = body.message ?? `请求失败：${resp.status}`;
    throw new Error(requestId ? `${message}（请求ID：${requestId}）` : message);
  }
  if (resp.status === 204) {
    return undefined as T;
  }
  return resp.json() as Promise<T>;
}

export const api = {
  login: (username: string, password: string) =>
    request<LoginResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<void>('/auth/logout', { method: 'POST', body: JSON.stringify({}) }),
  ships: (keyword = '') => request<ShipPageResult>(`/ships?keyword=${encodeURIComponent(keyword)}`),
  createShip: (ship: ShipUpsertRequest) => request<Ship>('/ships', { method: 'POST', body: JSON.stringify(ship) }),
  updateShip: (id: number, ship: ShipUpsertRequest) => request<Ship>(`/ships/${id}`, { method: 'PUT', body: JSON.stringify(ship) }),
  deleteShip: (id: number) => request<void>(`/ships/${id}`, { method: 'DELETE' }),
  reportLocation: (shipId: number, payload: ShipLocationReportRequest) =>
    request<LocationReportResponse>(`/ships/${shipId}/locations`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  tracks: (shipId: number) => request<TrackListResponse>(`/ships/${shipId}/tracks`),
  alarms: (status = '') => request<AlarmPageResult>(`/alarms?status=${encodeURIComponent(status)}`),
  ackAlarm: (id: number) => request<Alarm>(`/alarms/${id}/ack`, { method: 'PUT', body: JSON.stringify({}) }),
  dispatchEvents: (status = '') => request<DispatchEventPageResult>(`/dispatch-events?status=${encodeURIComponent(status)}`),
  createDispatchEvent: (event: DispatchEventCreateRequest) =>
    request<DispatchEvent>('/dispatch-events', { method: 'POST', body: JSON.stringify(event) }),
  updateDispatchStatus: (id: number, payload: DispatchStatusUpdateRequest) =>
    request<DispatchEvent>(`/dispatch-events/${id}/status`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  battleScenarios: () => request<BattleScenarioListResponse>('/battle/scenarios'),
  battleSessions: () => request<BattleSessionPageResult>('/battle/sessions'),
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

export async function startSimulator() {
  const payload: SimulationStartRequest = { shipIds: [1, 2] };
  return request<AnalyticsProxyResponse>('/analytics/simulate/start', {
    method: 'POST',
    body: JSON.stringify(payload),
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
