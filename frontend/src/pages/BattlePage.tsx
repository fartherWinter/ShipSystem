import { Alert, Button, Card, Empty, List, Progress, Select, Slider, Space, Statistic, Tabs, Tag, Typography, message } from 'antd';
import { ChevronLeft, ChevronRight, Pause, Play, Radar, RefreshCw, Shield, Target } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { api, listAllBattleSessions, startBattleSimulator, stopBattleSimulator } from '../api/client';
import MonitorMap from '../components/MonitorMap';
import type { BattleReport, BattleScenario, BattleSession, BattleSnapshot, BattleState, BattleTimelineItem, BattleUnit } from '../types';
import { battleReportDownloadFilename, buildBattleReportExport } from '../utils/battleReportExport';
import { downloadTextFile } from '../utils/download';

type Props = {
  state: BattleState | null;
  onStateChange: (state: BattleState | null) => void;
};

export default function BattlePage({ state, onStateChange }: Props) {
  const [scenarios, setScenarios] = useState<BattleScenario[]>([]);
  const [scenarioCode, setScenarioCode] = useState('open-water-duel');
  const [loading, setLoading] = useState(false);
  const [scenarioLoading, setScenarioLoading] = useState(false);
  const [scenarioError, setScenarioError] = useState('');
  const [stateError, setStateError] = useState('');

  const scenario = useMemo(
    () => scenarios.find((item) => item.code === scenarioCode) ?? scenarios[0],
    [scenarioCode, scenarios],
  );

  async function loadScenarios() {
    setScenarioLoading(true);
    setScenarioError('');
    try {
      const res = await api.battleScenarios();
      setScenarios(res.items);
      if (res.items[0]) {
        setScenarioCode(res.items[0].code);
      }
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载对战场景失败';
      setScenarioError(text);
      message.error(text);
    } finally {
      setScenarioLoading(false);
    }
  }

  useEffect(() => {
    void loadScenarios();
  }, []);

  useEffect(() => {
    if (state?.sessionId) {
      return;
    }
    void restoreRunningSession();
  }, [state?.sessionId]);

  async function restoreRunningSession() {
    setLoading(true);
    setStateError('');
    try {
      const sessions = await listAllBattleSessions();
      const runningSession = sessions.find((item) => item.status === 'running');
      if (!runningSession) {
        return;
      }
      setScenarioCode(runningSession.scenarioCode);
      onStateChange(await api.battleState(runningSession.sessionId));
    } catch (err) {
      const text = err instanceof Error ? err.message : '恢复运行中对战失败';
      setStateError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  async function start() {
    if (!scenario) {
      message.warning('暂无可用对战场景');
      return;
    }
    setLoading(true);
    setStateError('');
    try {
      const created = await api.createBattleSession({ scenarioCode: scenario.code });
      onStateChange(created.state);
      await startBattleSimulator({
        sessionId: created.session.sessionId,
        scenarioCode: scenario.code,
        originLongitude: scenario.originLongitude,
        originLatitude: scenario.originLatitude,
      });
      message.success('雷达对战模拟已启动');
    } catch (err) {
      message.error(err instanceof Error ? err.message : '启动雷达对战失败');
    } finally {
      setLoading(false);
    }
  }

  async function stop() {
    if (!state?.sessionId) return;
    setLoading(true);
    setStateError('');
    try {
      await stopBattleSimulator(state.sessionId);
      const stopped = await api.stopBattleSession(state.sessionId);
      onStateChange(stopped);
      message.success('雷达对战模拟已停止');
    } catch (err) {
      message.error(err instanceof Error ? err.message : '停止雷达对战失败');
    } finally {
      setLoading(false);
    }
  }

  async function refresh() {
    if (!state?.sessionId) return;
    setLoading(true);
    setStateError('');
    try {
      onStateChange(await api.battleState(state.sessionId));
    } catch (err) {
      const text = err instanceof Error ? err.message : '刷新战斗状态失败';
      setStateError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Tabs
      className="battle-tabs"
      items={[
        {
          key: 'live',
          label: '实时对战',
          children: (
            <LiveBattlePanel
              state={state}
              scenario={scenario}
              scenarios={scenarios}
              scenarioCode={scenarioCode}
              loading={loading}
              scenarioLoading={scenarioLoading}
              scenarioError={scenarioError}
              stateError={stateError}
              onScenarioChange={setScenarioCode}
              onReloadScenarios={loadScenarios}
              onStart={start}
              onStop={stop}
              onRefresh={refresh}
            />
          ),
        },
        {
          key: 'replay',
          label: '历史回放',
          children: <BattleReplayPanel />,
        },
      ]}
    />
  );
}

function LiveBattlePanel({
  state,
  scenario,
  scenarios,
  scenarioCode,
  loading,
  scenarioLoading,
  scenarioError,
  stateError,
  onScenarioChange,
  onReloadScenarios,
  onStart,
  onStop,
  onRefresh,
}: {
  state: BattleState | null;
  scenario?: BattleScenario;
  scenarios: BattleScenario[];
  scenarioCode: string;
  loading: boolean;
  scenarioLoading: boolean;
  scenarioError: string;
  stateError: string;
  onScenarioChange: (value: string) => void;
  onReloadScenarios: () => void;
  onStart: () => void;
  onStop: () => void;
  onRefresh: () => void;
}) {
  const units = state?.units ?? [];
  const projectiles = state?.projectiles ?? [];
  const radarTargets = state?.radarTargets ?? [];
  const activeProjectiles = projectiles.filter((item) => item.status === 'flying');
  const blueUnits = units.filter((item) => item.side === 'blue');
  const redUnits = units.filter((item) => item.side === 'red');
  const isRunning = state?.status === 'running';

  return (
    <div className="battle-layout">
      <section className="battle-map-panel">
        <MonitorMap battleUnits={units} radarTargets={radarTargets} projectiles={projectiles} fitMode="fit-data" />
      </section>
      <aside className="battle-side-panel">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Card title="雷达对战控制">
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              {scenarioError && (
                <Alert type="error" showIcon message="对战场景加载失败" description={scenarioError} action={<Button onClick={onReloadScenarios}>重试</Button>} />
              )}
              {stateError && <Alert type="error" showIcon message="战斗状态刷新失败" description={stateError} />}
              <Select
                value={scenarioCode}
                onChange={onScenarioChange}
                loading={scenarioLoading}
                disabled={isRunning || scenarioLoading || scenarios.length === 0}
                options={scenarios.map((item) => ({ value: item.code, label: item.name }))}
                style={{ width: '100%' }}
              />
              {scenario && (
                <Typography.Text type="secondary">
                  {scenario.description} / 雷达 {scenario.radarRangeKm}km / 武器 {scenario.weaponRangeKm}km
                </Typography.Text>
              )}
              <Space wrap>
                <Button type="primary" icon={<Play size={16} />} loading={loading} disabled={isRunning} onClick={onStart}>
                  启动对战
                </Button>
                <Button danger icon={<Pause size={16} />} loading={loading} disabled={!state?.sessionId || !isRunning} onClick={onStop}>
                  停止
                </Button>
                <Button icon={<RefreshCw size={16} />} loading={loading} disabled={!state?.sessionId} onClick={onRefresh} />
              </Space>
              <Space>
                <Tag color={statusColor(state?.status)}>{state?.status ?? '未启动'}</Tag>
                {state?.sessionId && <Typography.Text copyable>{state.sessionId}</Typography.Text>}
              </Space>
            </Space>
          </Card>

          <div className="battle-stat-grid">
            <Card>
              <Statistic title="雷达目标" value={radarTargets.filter((item) => item.detected).length} prefix={<Radar size={16} />} />
            </Card>
            <Card>
              <Statistic title="飞行弹幕" value={activeProjectiles.length} prefix={<Target size={16} />} />
            </Card>
          </div>

          <ForceCard title="蓝方舰队" color="blue" units={blueUnits} />
          <ForceCard title="红方舰队" color="red" units={redUnits} />
          <EventCard title="战斗事件流" events={(state?.events ?? []).slice(0, 12)} />
        </Space>
      </aside>
    </div>
  );
}

function BattleReplayPanel() {
  const [sessions, setSessions] = useState<BattleSession[]>([]);
  const [selectedSessionId, setSelectedSessionId] = useState('');
  const [timeline, setTimeline] = useState<BattleTimelineItem[]>([]);
  const [snapshots, setSnapshots] = useState<BattleSnapshot[]>([]);
  const [report, setReport] = useState<BattleReport | null>(null);
  const [index, setIndex] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [playing, setPlaying] = useState(false);
  const [loading, setLoading] = useState(false);
  const [sessionsError, setSessionsError] = useState('');
  const [detailError, setDetailError] = useState('');

  const current = snapshots[index];
  const tickIndexMap = useMemo(() => new Map(snapshots.map((item, itemIndex) => [item.tick, itemIndex])), [snapshots]);

  useEffect(() => {
    refreshSessions();
  }, []);

  useEffect(() => {
    if (!playing || snapshots.length <= 1) return;
    const interval = window.setInterval(() => {
      setIndex((prev) => {
        if (prev >= snapshots.length - 1) {
          setPlaying(false);
          return prev;
        }
        return prev + 1;
      });
    }, Math.max(120, 1000 / speed));
    return () => window.clearInterval(interval);
  }, [playing, snapshots.length, speed]);

  async function refreshSessions() {
    setLoading(true);
    setSessionsError('');
    try {
      const items = await listAllBattleSessions();
      setSessions(items);
      if (!selectedSessionId && items[0]) {
        await loadSession(items[0].sessionId);
      }
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载历史对战失败';
      setSessionsError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  async function loadSession(sessionId: string) {
    setSelectedSessionId(sessionId);
    setPlaying(false);
    setIndex(0);
    setLoading(true);
    setDetailError('');
    try {
      const [timelineRes, snapshotsRes, reportRes] = await Promise.all([
        api.battleTimeline(sessionId),
        api.battleSnapshots(sessionId),
        api.battleReport(sessionId),
      ]);
      setTimeline(timelineRes.items);
      setSnapshots(snapshotsRes.items);
      setReport(reportRes);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载回放数据失败';
      setDetailError(text);
      setTimeline([]);
      setSnapshots([]);
      setReport(null);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  function togglePlay() {
    if (!snapshots.length) return;
    if (playing) {
      setPlaying(false);
      return;
    }
    if (index >= snapshots.length - 1) {
      setIndex(0);
    }
    setPlaying(true);
  }

  function move(delta: number) {
    setPlaying(false);
    setIndex((prev) => Math.min(Math.max(prev + delta, 0), Math.max(snapshots.length - 1, 0)));
  }

  function jumpToTick(tick: number) {
    setPlaying(false);
    const exactIndex = tickIndexMap.get(tick);
    if (exactIndex !== undefined) {
      setIndex(exactIndex);
      return;
    }
    const closestIndex = snapshots.reduce(
      (bestIndex, snapshot, snapshotIndex) =>
        Math.abs(snapshot.tick - tick) < Math.abs(snapshots[bestIndex].tick - tick) ? snapshotIndex : bestIndex,
      0,
    );
    setIndex(closestIndex);
  }

  return (
    <div className="battle-replay-layout">
      <section className="battle-replay-map-panel">
        {current ? (
          <MonitorMap battleUnits={current.units} radarTargets={current.radarTargets} projectiles={current.projectiles} fitMode="fit-data" />
        ) : (
          <div className="battle-empty-panel">
            <Empty description="暂无回放快照" />
          </div>
        )}
      </section>
      <aside className="battle-side-panel">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Card
            title="历史对战"
            extra={<Button icon={<RefreshCw size={16} />} loading={loading} onClick={refreshSessions} />}
          >
            {sessionsError && (
              <Alert
                type="error"
                showIcon
                message="历史对战加载失败"
                description={sessionsError}
                action={<Button onClick={refreshSessions}>重试</Button>}
                style={{ marginBottom: 12 }}
              />
            )}
            <List
              size="small"
              dataSource={sessions}
              locale={{ emptyText: sessionsError ? '历史对战加载失败，请重试' : '暂无历史对战' }}
              renderItem={(item) => (
                <List.Item>
                  <Button
                    block
                    type={item.sessionId === selectedSessionId ? 'primary' : 'default'}
                    onClick={() => loadSession(item.sessionId)}
                  >
                    <span className="battle-session-button">
                      <span>{item.name}</span>
                      <Tag color={statusColor(item.status)}>{item.status}</Tag>
                    </span>
                  </Button>
                </List.Item>
              )}
            />
          </Card>

          {detailError && (
            <Alert
              type="error"
              showIcon
              message="回放数据加载失败"
              description={detailError}
              action={selectedSessionId ? <Button onClick={() => loadSession(selectedSessionId)}>重试</Button> : undefined}
            />
          )}

          <Card title="回放控制">
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Space wrap>
                <Button icon={<ChevronLeft size={16} />} disabled={!snapshots.length || index <= 0} onClick={() => move(-1)} />
                <Button type="primary" icon={playing ? <Pause size={16} /> : <Play size={16} />} disabled={!snapshots.length} onClick={togglePlay}>
                  {playing ? '暂停' : '播放'}
                </Button>
                <Button
                  icon={<ChevronRight size={16} />}
                  disabled={!snapshots.length || index >= snapshots.length - 1}
                  onClick={() => move(1)}
                />
                <Select
                  value={speed}
                  onChange={setSpeed}
                  options={[0.5, 1, 2, 4].map((value) => ({ value, label: `${value}x` }))}
                  style={{ width: 92 }}
                />
              </Space>
              <Slider
                min={0}
                max={Math.max(snapshots.length - 1, 0)}
                value={index}
                disabled={!snapshots.length}
                onChange={(value) => {
                  setPlaying(false);
                  setIndex(Array.isArray(value) ? value[0] : value);
                }}
              />
              <Space>
                <Tag>Tick {current?.tick ?? '-'}</Tag>
                <Typography.Text type="secondary">{current ? formatTime(current.snapshotTime) : '暂无时间'}</Typography.Text>
              </Space>
            </Space>
          </Card>

          <BattleReportCard report={report} timeline={timeline} />
          <TimelineCard timeline={timeline} currentTick={current?.tick} onSelectTick={jumpToTick} />
          <EventCard title="当前帧事件" events={current?.events ?? []} />
        </Space>
      </aside>
    </div>
  );
}

function BattleReportCard({ report, timeline }: { report: BattleReport | null; timeline: BattleTimelineItem[] }) {
  return (
    <Card title="战报统计">
      {!report ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无战报" />
      ) : (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div className="battle-stat-grid">
            <Statistic title="胜负" value={winnerLabel(report.winner)} />
            <Statistic title="发射数" value={report.firedCount} />
            <Statistic title="命中数" value={report.hitCount} />
            <Statistic title="摧毁数" value={report.destroyedCount} />
          </div>
          <List
            size="small"
            header="损伤排行"
            dataSource={report.damageRanking.slice(0, 6)}
            locale={{ emptyText: '暂无损伤数据' }}
            renderItem={(item) => {
              const percent = item.maxHp > 0 ? Math.round((item.damageTaken / item.maxHp) * 100) : 0;
              return (
                <List.Item>
                  <div className="battle-damage-row">
                    <Space>
                      <Tag color={item.side === 'blue' ? 'blue' : 'red'}>{item.side}</Tag>
                      <Typography.Text>{item.name}</Typography.Text>
                    </Space>
                    <Progress percent={percent} size="small" status={item.status === 'destroyed' ? 'exception' : 'active'} />
                  </div>
                </List.Item>
              );
            }}
          />
          <Space wrap>
            <Button onClick={() => exportBattleReport(report, timeline, "json")}>导出 JSON</Button>
            <Button onClick={() => exportBattleReport(report, timeline, "csv")}>导出 CSV</Button>
            <Button onClick={() => exportBattleReport(report, timeline, "html")}>导出 HTML</Button>
          </Space>
          <EventCard title="关键事件" events={report.keyEvents.slice(0, 8)} />
        </Space>
      )}
    </Card>
  );
}

function TimelineCard({
  timeline,
  currentTick,
  onSelectTick,
}: {
  timeline: BattleTimelineItem[];
  currentTick?: number;
  onSelectTick: (tick: number) => void;
}) {
  return (
    <Card title="时间轴摘要">
      <List
        size="small"
        dataSource={timeline.filter((item) => item.eventCount > 0).slice(-12).reverse()}
        locale={{ emptyText: '暂无事件摘要' }}
        renderItem={(item) => (
          <List.Item className={item.tick === currentTick ? 'battle-current-tick' : undefined}>
            <button
              type="button"
              className="battle-timeline-entry"
              onClick={() => onSelectTick(item.tick)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelectTick(item.tick);
                }
              }}
            >
              <Space direction="vertical" size={2}>
                <Space>
                  <Tag>Tick {item.tick}</Tag>
                  <Typography.Text type="secondary">{formatTime(item.snapshotTime)}</Typography.Text>
                </Space>
                {item.events.slice(0, 2).map((event, index) => (
                  <Typography.Text key={`${item.tick}-${event.type}-${index}`}>{event.message}</Typography.Text>
                ))}
              </Space>
            </button>
          </List.Item>
        )}
      />
    </Card>
  );
}

function ForceCard({ title, color, units }: { title: string; color: 'blue' | 'red'; units: BattleUnit[] }) {
  return (
    <Card title={<Space><Shield size={16} />{title}</Space>}>
      <Space direction="vertical" size="small" style={{ width: '100%' }}>
        {units.length === 0 && <Typography.Text type="secondary">暂无单位</Typography.Text>}
        {units.map((unit) => {
          const percent = unit.maxHp > 0 ? Math.max(0, Math.round((unit.hp / unit.maxHp) * 100)) : 0;
          return (
            <div key={unit.unitId} className="battle-unit-row">
              <div>
                <Typography.Text strong>{unit.name}</Typography.Text>
                <Typography.Text type="secondary">
                  {unit.status} / {unit.speedKnots.toFixed(1)} kt / {unit.course.toFixed(0)} deg
                </Typography.Text>
              </div>
              <Progress percent={percent} size="small" status={unit.status === 'destroyed' ? 'exception' : 'active'} strokeColor={color === 'blue' ? '#2563eb' : '#dc2626'} />
            </div>
          );
        })}
      </Space>
    </Card>
  );
}

function EventCard({ title, events }: { title: string; events: BattleState['events'] }) {
  return (
    <Card title={title}>
      <List
        size="small"
        dataSource={events}
        locale={{ emptyText: '暂无战斗事件' }}
        renderItem={(item) => (
          <List.Item>
            <Space direction="vertical" size={2}>
              <Space>
                <Tag color={item.severity === 'CRITICAL' ? 'red' : item.severity === 'WARN' ? 'orange' : 'blue'}>
                  {item.type}
                </Tag>
                <Typography.Text type="secondary">{formatTime(item.occurredAt)}</Typography.Text>
              </Space>
              <Typography.Text>{item.message}</Typography.Text>
            </Space>
          </List.Item>
        )}
      />
    </Card>
  );
}

function statusColor(status?: string) {
  if (status === 'running') return 'green';
  if (status?.includes('victory')) return 'gold';
  if (status === 'stopped') return 'default';
  return 'blue';
}

function winnerLabel(value: string) {
  if (value === 'blue') return '蓝方胜利';
  if (value === 'red') return '红方胜利';
  if (value === 'pending') return '进行中';
  return '未决';
}

function exportBattleReport(report: BattleReport, timeline: BattleTimelineItem[], format: "json" | "csv" | "html") {
  const exported = buildBattleReportExport(report, timeline, format);
  downloadTextFile(battleReportDownloadFilename(report.session.sessionId, format), exported.text, exported.mimeType);
  message.success(`已导出战报 ${format.toUpperCase()}`);
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}
