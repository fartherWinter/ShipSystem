import { Alert, Button, Card, List, Select, Space, Statistic, Tag, Typography, message } from 'antd';
import { Play, Radio, RefreshCw, Square } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { getSimulatorStatus, listAllShips, startSimulator, stopSimulator } from '../api/client';
import MonitorMap from '../components/MonitorMap';
import type { AnalyticsProxyResponse, Ship, ShipLocation } from '../types';

type Props = {
  locations: ShipLocation[];
  wsStatus: string;
};

type DeliveryMetrics = {
  successCount: number;
  failureCount: number;
  retryCount: number;
  droppedCount: number;
  lastSuccessAt: string | null;
  lastFailureAt: string | null;
  lastError: string;
};

type BattleSessionSummary = {
  runningCount: number;
  totalCount: number;
};

const emptyDeliveryMetrics: DeliveryMetrics = {
  successCount: 0,
  failureCount: 0,
  retryCount: 0,
  droppedCount: 0,
  lastSuccessAt: null,
  lastFailureAt: null,
  lastError: '',
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function readNumber(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function readString(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function formatDateTime(value?: string | null) {
  if (!value) {
    return '-';
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function readRunningFlag(payload: AnalyticsProxyResponse) {
  return typeof payload.running === 'boolean' ? payload.running : false;
}

function readResponseMessage(payload: AnalyticsProxyResponse) {
  return typeof payload.message === 'string' ? payload.message.trim() : '';
}

function readRequestId(payload: AnalyticsProxyResponse) {
  return typeof payload.requestId === 'string' ? payload.requestId.trim() : '';
}

function readDeliveryMetrics(payload: AnalyticsProxyResponse): DeliveryMetrics {
  if (!isRecord(payload.delivery)) {
    return emptyDeliveryMetrics;
  }
  return {
    successCount: readNumber(payload.delivery.successCount),
    failureCount: readNumber(payload.delivery.failureCount),
    retryCount: readNumber(payload.delivery.retryCount),
    droppedCount: readNumber(payload.delivery.droppedCount),
    lastSuccessAt: readString(payload.delivery.lastSuccessAt) || null,
    lastFailureAt: readString(payload.delivery.lastFailureAt) || null,
    lastError: readString(payload.delivery.lastError),
  };
}

function readBattleSessionSummary(payload: AnalyticsProxyResponse): BattleSessionSummary {
  if (!isRecord(payload.battleSessions)) {
    return { runningCount: 0, totalCount: 0 };
  }

  let runningCount = 0;
  let totalCount = 0;

  Object.values(payload.battleSessions).forEach((item) => {
    if (!isRecord(item)) {
      return;
    }
    totalCount += 1;
    if (item.running === true) {
      runningCount += 1;
    }
  });

  return { runningCount, totalCount };
}

export default function MonitorPage({ locations, wsStatus }: Props) {
  const [ships, setShips] = useState<Ship[]>([]);
  const [selectedShipIds, setSelectedShipIds] = useState<number[]>([]);
  const [shipsLoading, setShipsLoading] = useState(false);
  const [shipsError, setShipsError] = useState('');
  const [simulatorRunning, setSimulatorRunning] = useState(false);
  const [statusLoading, setStatusLoading] = useState(false);
  const [statusError, setStatusError] = useState('');
  const [statusNote, setStatusNote] = useState('');
  const [statusUpdatedAt, setStatusUpdatedAt] = useState('');
  const [deliveryMetrics, setDeliveryMetrics] = useState<DeliveryMetrics>(emptyDeliveryMetrics);
  const [battleSessionSummary, setBattleSessionSummary] = useState<BattleSessionSummary>({ runningCount: 0, totalCount: 0 });
  const [starting, setStarting] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [startError, setStartError] = useState('');

  const shipNameById = useMemo(() => new Map(ships.map((ship) => [ship.id, ship.name])), [ships]);
  const selectedShipNames = selectedShipIds.map((id) => shipNameById.get(id)).filter(Boolean) as string[];
  const recentLocations = useMemo(
    () =>
      [...locations].sort((left, right) => {
        const leftTime = Date.parse(left.reportedAt);
        const rightTime = Date.parse(right.reportedAt);
        return (Number.isNaN(rightTime) ? 0 : rightTime) - (Number.isNaN(leftTime) ? 0 : leftTime);
      }),
    [locations],
  );

  useEffect(() => {
    void refreshMonitorState();
  }, []);

  useEffect(() => {
    if (!simulatorRunning && battleSessionSummary.runningCount === 0) {
      return;
    }
    const timer = window.setInterval(() => {
      void loadSimulatorStatus();
    }, 10000);
    return () => window.clearInterval(timer);
  }, [simulatorRunning, battleSessionSummary.runningCount]);

  async function loadShips() {
    setShipsLoading(true);
    setShipsError('');
    try {
      const items = await listAllShips();
      setShips(items);
      setSelectedShipIds((current) => {
        const next = current.filter((id) => items.some((ship) => ship.id === id));
        return next.length ? next : items.slice(0, 2).map((ship) => ship.id);
      });
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载模拟船舶失败';
      setShipsError(text);
      message.error(text);
    } finally {
      setShipsLoading(false);
    }
  }

  async function loadSimulatorStatus() {
    setStatusLoading(true);
    setStatusError('');
    try {
      const data = await getSimulatorStatus();
      setSimulatorRunning(readRunningFlag(data));
      setDeliveryMetrics(readDeliveryMetrics(data));
      setBattleSessionSummary(readBattleSessionSummary(data));
      setStatusUpdatedAt(new Date().toISOString());
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载模拟器状态失败';
      setStatusError(text);
      message.error(text);
    } finally {
      setStatusLoading(false);
    }
  }

  async function refreshMonitorState() {
    await Promise.all([loadShips(), loadSimulatorStatus()]);
  }

  async function start() {
    if (selectedShipIds.length === 0) {
      const text = '请至少选择一艘用于模拟的船舶';
      setStartError(text);
      message.warning(text);
      return;
    }
    setStarting(true);
    setStartError('');
    setStatusError('');
    try {
      const data = await startSimulator(selectedShipIds);
      const serverMessage = readResponseMessage(data);
      const requestId = readRequestId(data);
      setSimulatorRunning(readRunningFlag(data));
      setStatusNote(
        requestId
          ? `${serverMessage || '模拟器状态已更新'}（请求 ID：${requestId}）`
          : serverMessage || `本次选择 ${selectedShipIds.length} 艘船舶启动模拟`,
      );
      if (serverMessage) {
        message.info(serverMessage);
      } else {
        message.success('模拟器已启动');
      }
      await loadSimulatorStatus();
    } catch (err) {
      const text = err instanceof Error ? err.message : '启动模拟器失败';
      setStartError(text);
      message.error(text);
    } finally {
      setStarting(false);
    }
  }

  async function stop() {
    setStopping(true);
    setStartError('');
    setStatusError('');
    try {
      const data = await stopSimulator();
      const requestId = readRequestId(data);
      setSimulatorRunning(readRunningFlag(data));
      setStatusNote(requestId ? `模拟器已停止（请求 ID：${requestId}）` : '模拟器已停止');
      message.success('模拟器已停止');
      await loadSimulatorStatus();
    } catch (err) {
      const text = err instanceof Error ? err.message : '停止模拟器失败';
      setStatusError(text);
      message.error(text);
    } finally {
      setStopping(false);
    }
  }

  const simulatorStatusLabel = statusLoading ? '检查中' : simulatorRunning ? '运行中' : '未运行';
  const simulatorStatusColor = statusLoading ? 'processing' : simulatorRunning ? 'green' : 'default';
  const wsStatusColor = wsStatus === 'connected' ? 'green' : wsStatus === 'connecting' ? 'processing' : 'red';
  const deliveryHealthColor = deliveryMetrics.failureCount > 0 || deliveryMetrics.droppedCount > 0 ? 'orange' : 'green';

  return (
    <div className="monitor-layout">
      <div className="map-panel">
        <MonitorMap locations={locations} />
      </div>
      <aside className="side-panel">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Card title="实时连接">
            <Space direction="vertical" size="small">
              <Space>
                <Radio size={18} />
                <Typography.Text>WebSocket</Typography.Text>
                <Tag color={wsStatusColor}>{wsStatus}</Tag>
              </Space>
              <Space>
                <Typography.Text>模拟器</Typography.Text>
                <Tag color={simulatorStatusColor}>{simulatorStatusLabel}</Tag>
              </Space>
              <Space>
                <Typography.Text>对战任务</Typography.Text>
                <Tag color={battleSessionSummary.runningCount > 0 ? 'gold' : 'default'}>
                  {battleSessionSummary.runningCount > 0 ? `${battleSessionSummary.runningCount} 个运行中` : '当前无运行中任务'}
                </Tag>
              </Space>
            </Space>
          </Card>

          <Card title="模拟数据">
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              {shipsError && <Alert type="error" showIcon message="模拟船舶加载失败" description={shipsError} action={<Button onClick={loadShips}>重试</Button>} />}
              {statusError && <Alert type="error" showIcon message="模拟器状态同步失败" description={statusError} action={<Button onClick={refreshMonitorState}>刷新</Button>} />}
              {startError && <Alert type="error" showIcon message="模拟器启动失败" description={startError} action={<Button onClick={start}>重试</Button>} />}

              <Select
                mode="multiple"
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder="选择要注入模拟轨迹的船舶"
                value={selectedShipIds}
                loading={shipsLoading}
                disabled={shipsLoading || ships.length === 0}
                options={ships.map((ship) => ({ label: `${ship.name} / ${ship.mmsi}`, value: ship.id }))}
                onChange={(value) => setSelectedShipIds(value as number[])}
              />

              <Typography.Text type="secondary">
                {selectedShipNames.length > 0 ? `已选 ${selectedShipIds.length} 艘：${selectedShipNames.join('、')}` : '当前未选择模拟船舶'}
              </Typography.Text>

              <Space wrap>
                <Button
                  type="primary"
                  icon={<Play size={16} />}
                  loading={starting}
                  disabled={selectedShipIds.length === 0 || simulatorRunning || stopping}
                  onClick={start}
                >
                  启动模拟器
                </Button>
                <Button danger icon={<Square size={14} />} loading={stopping} disabled={!simulatorRunning || starting} onClick={stop}>
                  停止模拟器
                </Button>
                <Button icon={<RefreshCw size={16} />} loading={shipsLoading || statusLoading} disabled={starting || stopping} onClick={refreshMonitorState}>
                  刷新状态
                </Button>
              </Space>

              <Typography.Text type="secondary">当前可选船舶 {ships.length} 艘</Typography.Text>
              {statusNote && <Typography.Text type="secondary">{statusNote}</Typography.Text>}
            </Space>
          </Card>

          <Card title="投递状态" extra={<Tag color={deliveryHealthColor}>{deliveryMetrics.failureCount > 0 || deliveryMetrics.droppedCount > 0 ? '需关注' : '正常'}</Tag>}>
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
                  gap: 12,
                }}
              >
                <Statistic title="成功投递" value={deliveryMetrics.successCount} />
                <Statistic title="失败次数" value={deliveryMetrics.failureCount} />
                <Statistic title="重试次数" value={deliveryMetrics.retryCount} />
                <Statistic title="丢弃次数" value={deliveryMetrics.droppedCount} />
              </div>
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <Typography.Text type="secondary">最近成功：{formatDateTime(deliveryMetrics.lastSuccessAt)}</Typography.Text>
                <Typography.Text type="secondary">最近失败：{formatDateTime(deliveryMetrics.lastFailureAt)}</Typography.Text>
                <Typography.Text type="secondary">状态刷新：{formatDateTime(statusUpdatedAt)}</Typography.Text>
                <Typography.Text type="secondary">已登记对战会话：{battleSessionSummary.totalCount}</Typography.Text>
                <Typography.Text type="secondary">最近错误：{deliveryMetrics.lastError || '无'}</Typography.Text>
              </Space>
            </Space>
          </Card>

          <Card title="最新位置">
            <List
              size="small"
              dataSource={recentLocations}
              locale={{ emptyText: '暂无实时点位' }}
              renderItem={(item) => (
                <List.Item>
                  <Space direction="vertical" size={0}>
                    <Typography.Text>
                      {shipNameById.get(item.shipId) ?? `#${item.shipId}`} {item.longitude.toFixed(4)}, {item.latitude.toFixed(4)} / {item.speedKnots.toFixed(1)} 节
                    </Typography.Text>
                    <Typography.Text type="secondary">{formatDateTime(item.reportedAt)}</Typography.Text>
                  </Space>
                </List.Item>
              )}
            />
          </Card>
        </Space>
      </aside>
    </div>
  );
}
