import { Bell, LayoutDashboard, Map, Radar, Route as RouteIcon, Send, ShipWheel, Users } from 'lucide-react';
import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { api } from './api/client';
import type { BattleState, DispatchEvent, LoginResponse, ShipLocation, User, WsMessage } from './types';

const WS_RECONNECT_INITIAL_DELAY_MS = 1000;
const WS_RECONNECT_MAX_DELAY_MS = 30000;
const WS_STABLE_CONNECTION_MS = 30000;

const AppShell = lazy(() => import('./components/AppShell'));
const LoginPage = lazy(() => import('./pages/LoginPage'));
const AlarmsPage = lazy(() => import('./pages/AlarmsPage'));
const BattlePage = lazy(() => import('./pages/BattlePage'));
const DashboardPage = lazy(() => import('./pages/DashboardPage'));
const DispatchPage = lazy(() => import('./pages/DispatchPage'));
const MonitorPage = lazy(() => import('./pages/MonitorPage'));
const RbacPage = lazy(() => import('./pages/RbacPage'));
const ShipsPage = lazy(() => import('./pages/ShipsPage'));
const TracksPage = lazy(() => import('./pages/TracksPage'));

type RoleCode = 'super_admin' | 'admin' | 'dispatcher' | 'viewer';

const routes = [
  { path: '/dashboard', label: '首页态势', icon: <LayoutDashboard size={18} />, roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] as RoleCode[] },
  { path: '/ships', label: '船舶管理', icon: <ShipWheel size={18} />, roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] as RoleCode[] },
  { path: '/monitor', label: '实时监控', icon: <Map size={18} />, roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] as RoleCode[] },
  { path: '/battle', label: '雷达对战', icon: <Radar size={18} />, roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] as RoleCode[] },
  { path: '/tracks', label: '轨迹回放', icon: <RouteIcon size={18} />, roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] as RoleCode[] },
  { path: '/alarms', label: '告警中心', icon: <Bell size={18} />, roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] as RoleCode[] },
  { path: '/dispatch', label: '调度事件', icon: <Send size={18} />, roles: ['super_admin', 'admin', 'dispatcher'] as RoleCode[] },
  { path: '/rbac', label: '权限管理', icon: <Users size={18} />, roles: ['super_admin'] as RoleCode[] },
];

export default function App() {
  const [user, setUser] = useState<User | null>(readSavedUser);
  const [wsStatus, setWsStatus] = useState<'connecting' | 'connected' | 'disconnected'>('disconnected');
  const [latestMap, setLatestMap] = useState<Record<number, ShipLocation>>({});
  const [battleState, setBattleState] = useState<BattleState | null>(null);
  const [dispatchEvents, setDispatchEvents] = useState<DispatchEvent[]>([]);
  const navigate = useNavigate();
  const location = useLocation();

  const latestLocations = useMemo(() => Object.values(latestMap), [latestMap]);
  const roleCode = user?.role?.code as RoleCode | undefined;
  const allowedRoutes = useMemo(() => routes.filter((item) => roleCode && item.roles.includes(roleCode)), [roleCode]);
  const navItems = useMemo(
    () => allowedRoutes.map((item) => ({ key: item.path, label: item.label, icon: item.icon })),
    [allowedRoutes],
  );
  const fallbackPath = allowedRoutes[0]?.path ?? '/dashboard';

  useEffect(() => {
    if (!user) {
      setWsStatus('disconnected');
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const wsBase = import.meta.env.VITE_WS_BASE ?? `${protocol}://${window.location.host}/ws/monitor`;
    let socket: WebSocket | null = null;
    let reconnectTimer: number | undefined;
    let retryAttempt = 0;
    let connectedAt = 0;
    let closedByEffect = false;

    function scheduleReconnect() {
      if (closedByEffect || reconnectTimer !== undefined) {
        return;
      }
      const delay = Math.min(WS_RECONNECT_MAX_DELAY_MS, WS_RECONNECT_INITIAL_DELAY_MS * 2 ** retryAttempt);
      retryAttempt += 1;
      reconnectTimer = window.setTimeout(() => {
        reconnectTimer = undefined;
        connect();
      }, delay);
    }

    function connect() {
      if (closedByEffect) {
        return;
      }
      setWsStatus('connecting');
      const nextSocket = new WebSocket(wsBase);
      socket = nextSocket;

      nextSocket.onopen = () => {
        if (closedByEffect || socket !== nextSocket) {
          return;
        }
        connectedAt = Date.now();
        setWsStatus('connected');
      };
      nextSocket.onclose = () => {
        if (closedByEffect || socket !== nextSocket) {
          return;
        }
        if (connectedAt > 0 && Date.now() - connectedAt >= WS_STABLE_CONNECTION_MS) {
          retryAttempt = 0;
        }
        connectedAt = 0;
        setWsStatus('disconnected');
        scheduleReconnect();
      };
      nextSocket.onerror = () => {
        if (closedByEffect || socket !== nextSocket) {
          return;
        }
        setWsStatus('disconnected');
        nextSocket.close();
      };
      nextSocket.onmessage = (event) => {
        const data = parseWsMessage(event.data);
        if (!data) {
          return;
        }
        handleWsMessage(data);
      };
    }

    function handleWsMessage(data: WsMessage) {
      if (data.type === 'ship_location_updated') {
        setLatestMap((prev) => ({ ...prev, [data.data.shipId]: data.data }));
      }
      if (data.type === 'alarm_created') {
        showWarning(data.data.title);
      }
      if (data.type === 'dispatch_event_updated') {
        setDispatchEvents((prev) => upsertDispatchEvent(prev, data.data));
      }
      if (data.type === 'radar_scan_updated') {
        setBattleState((prev) =>
          prev && prev.sessionId === data.data.sessionId
            ? { ...prev, radarTargets: data.data.targets, updatedAt: data.data.scanTime }
            : prev,
        );
      }
      if (data.type === 'projectile_updated') {
        setBattleState((prev) => {
          if (!prev || prev.sessionId !== data.data.sessionId) {
            return prev;
          }
          const next = prev.projectiles.filter((item) => item.projectileId !== data.data.projectileId);
          return { ...prev, projectiles: [data.data, ...next], updatedAt: new Date().toISOString() };
        });
      }
      if (data.type === 'battle_event_created') {
        setBattleState((prev) => {
          if (!prev || prev.sessionId !== data.data.sessionId) {
            return prev;
          }
          const next = prev.events.filter((item) => item.eventId !== data.data.eventId);
          return { ...prev, events: [data.data, ...next].slice(0, 80), updatedAt: data.data.occurredAt };
        });
      }
      if (data.type === 'battle_state_updated') {
        setBattleState(data.data);
      }
    }

    connect();

    return () => {
      closedByEffect = true;
      if (reconnectTimer !== undefined) {
        window.clearTimeout(reconnectTimer);
      }
      socket?.close();
    };
  }, [user]);

  function onLogin(data: LoginResponse) {
    localStorage.setItem('shipsystem_user', JSON.stringify(data.user));
    setUser(data.user);
    navigate('/dashboard', { replace: true });
  }

  async function logout() {
    await api.logout().catch(() => undefined);
    localStorage.removeItem('shipsystem_user');
    setUser(null);
    navigate('/dashboard', { replace: true });
  }

  if (!user) {
    return (
      <Suspense fallback={<PageLoading />}>
        <LoginPage onLogin={onLogin} />
      </Suspense>
    );
  }

  const canAccessCurrent = allowedRoutes.some((item) => item.path === location.pathname);

  return (
    <Suspense fallback={<PageLoading />}>
      <AppShell currentPath={location.pathname} onNavigate={navigate} onLogout={logout} user={user} wsStatus={wsStatus} items={navItems}>
        {!canAccessCurrent ? (
          <Navigate to={fallbackPath} replace />
        ) : (
          <Suspense fallback={<PageLoading compact />}>
            <Routes>
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/ships" element={<ShipsPage />} />
              <Route path="/monitor" element={<MonitorPage locations={latestLocations} wsStatus={wsStatus} />} />
              <Route path="/battle" element={<BattlePage state={battleState} onStateChange={setBattleState} />} />
              <Route path="/tracks" element={<TracksPage />} />
              <Route path="/alarms" element={<AlarmsPage />} />
              <Route path="/dispatch" element={<DispatchPage items={dispatchEvents} onItemsChange={setDispatchEvents} />} />
              <Route path="/rbac" element={<RbacPage />} />
              <Route path="*" element={<Navigate to={fallbackPath} replace />} />
            </Routes>
          </Suspense>
        )}
      </AppShell>
    </Suspense>
  );
}

function readSavedUser(): User | null {
  const savedUser = localStorage.getItem('shipsystem_user');
  if (!savedUser) {
    return null;
  }
  try {
    return JSON.parse(savedUser) as User;
  } catch {
    localStorage.removeItem('shipsystem_user');
    return null;
  }
}

function showWarning(content: string) {
  void import('antd').then(({ message }) => message.warning(content));
}

function parseWsMessage(payload: unknown): WsMessage | null {
  if (typeof payload !== 'string') {
    return null;
  }
  try {
    return JSON.parse(payload) as WsMessage;
  } catch {
    return null;
  }
}

function upsertDispatchEvent(items: DispatchEvent[], event: DispatchEvent) {
  const index = items.findIndex((item) => item.id === event.id);
  if (index === -1) {
    return [event, ...items];
  }
  const next = [...items];
  next[index] = event;
  return next;
}

function PageLoading({ compact = false }: { compact?: boolean }) {
  return <div className={compact ? 'route-loading route-loading-compact' : 'route-loading'}>加载中...</div>;
}
