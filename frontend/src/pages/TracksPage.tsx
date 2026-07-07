import { Alert, Button, Card, Col, Input, Row, Select, Space, Statistic, Table, Typography, message } from 'antd';
import { RefreshCw, Search } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { api, listAllShips } from '../api/client';
import MonitorMap from '../components/MonitorMap';
import type { Ship, ShipLocation } from '../types';

function toApiTimestamp(value: string): string | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    throw new Error('请选择有效的时间范围');
  }
  return parsed.toISOString();
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
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function formatDateTimeLocalInput(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function compareTrackAsc(left: ShipLocation, right: ShipLocation) {
  const leftTime = Date.parse(left.reportedAt);
  const rightTime = Date.parse(right.reportedAt);
  if (Number.isNaN(leftTime) && Number.isNaN(rightTime)) {
    return left.id - right.id;
  }
  if (Number.isNaN(leftTime)) {
    return 1;
  }
  if (Number.isNaN(rightTime)) {
    return -1;
  }
  if (leftTime !== rightTime) {
    return leftTime - rightTime;
  }
  return left.id - right.id;
}

function compareTrackDesc(left: ShipLocation, right: ShipLocation) {
  return compareTrackAsc(right, left);
}

function haversineNm(left: ShipLocation, right: ShipLocation) {
  const toRadians = (degrees: number) => (degrees * Math.PI) / 180;
  const earthRadiusKm = 6371;
  const deltaLat = toRadians(right.latitude - left.latitude);
  const deltaLon = toRadians(right.longitude - left.longitude);
  const lat1 = toRadians(left.latitude);
  const lat2 = toRadians(right.latitude);

  const a =
    Math.sin(deltaLat / 2) * Math.sin(deltaLat / 2) +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLon / 2) * Math.sin(deltaLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  const distanceKm = earthRadiusKm * c;
  return distanceKm / 1.852;
}

function formatDuration(start?: string, end?: string) {
  if (!start || !end) {
    return '-';
  }
  const startTime = Date.parse(start);
  const endTime = Date.parse(end);
  if (Number.isNaN(startTime) || Number.isNaN(endTime) || endTime < startTime) {
    return '-';
  }

  let remainingSeconds = Math.floor((endTime - startTime) / 1000);
  const hours = Math.floor(remainingSeconds / 3600);
  remainingSeconds -= hours * 3600;
  const minutes = Math.floor(remainingSeconds / 60);
  const seconds = remainingSeconds - minutes * 60;

  if (hours > 0) {
    return `${hours}小时 ${minutes}分钟`;
  }
  if (minutes > 0) {
    return `${minutes}分钟 ${seconds}秒`;
  }
  return `${seconds}秒`;
}

export default function TracksPage() {
  const [ships, setShips] = useState<Ship[]>([]);
  const [shipId, setShipId] = useState<number>();
  const [track, setTrack] = useState<ShipLocation[]>([]);
  const [shipsLoading, setShipsLoading] = useState(false);
  const [trackLoading, setTrackLoading] = useState(false);
  const [error, setError] = useState('');
  const [trackError, setTrackError] = useState('');
  const [startAt, setStartAt] = useState('');
  const [endAt, setEndAt] = useState('');
  const [lastQueriedAt, setLastQueriedAt] = useState('');

  const shipOptions = useMemo(
    () =>
      [...ships]
        .sort((left, right) => left.name.localeCompare(right.name, 'zh-CN') || left.id - right.id)
        .map((ship) => ({ label: `${ship.name} / ${ship.mmsi}`, value: ship.id })),
    [ships],
  );

  const selectedShip = ships.find((ship) => ship.id === shipId) ?? null;
  const orderedTrack = useMemo(() => [...track].sort(compareTrackAsc), [track]);
  const tableTrack = useMemo(() => [...track].sort(compareTrackDesc), [track]);

  const trackStats = useMemo(() => {
    if (orderedTrack.length === 0) {
      return {
        pointCount: 0,
        startedAt: '',
        endedAt: '',
        duration: '-',
        totalDistanceNm: 0,
        latestSpeedKnots: 0,
      };
    }

    let totalDistanceNm = 0;
    for (let index = 1; index < orderedTrack.length; index += 1) {
      totalDistanceNm += haversineNm(orderedTrack[index - 1], orderedTrack[index]);
    }

    const startedAt = orderedTrack[0].reportedAt;
    const endedAt = orderedTrack[orderedTrack.length - 1].reportedAt;
    const latestSpeedKnots = orderedTrack[orderedTrack.length - 1].speedKnots;

    return {
      pointCount: orderedTrack.length,
      startedAt,
      endedAt,
      duration: formatDuration(startedAt, endedAt),
      totalDistanceNm,
      latestSpeedKnots,
    };
  }, [orderedTrack]);

  async function loadShips() {
    setShipsLoading(true);
    setError('');
    try {
      const items = await listAllShips();
      setShips(items);
      setShipId((current) => (current && items.some((ship) => ship.id === current) ? current : items[0]?.id));
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载船舶列表失败';
      setError(text);
      message.error(text);
    } finally {
      setShipsLoading(false);
    }
  }

  useEffect(() => {
    void loadShips();
  }, []);

  async function loadTrack() {
    if (!shipId) {
      return;
    }
    try {
      const start = toApiTimestamp(startAt);
      const end = toApiTimestamp(endAt);
      if (start && end && start > end) {
        const text = '开始时间不能晚于结束时间';
        setTrackError(text);
        message.warning(text);
        return;
      }
      setTrackLoading(true);
      setTrackError('');
      try {
        const data = await api.tracks(shipId, { start, end });
        setTrack(data.items);
        setLastQueriedAt(new Date().toISOString());
      } catch (err) {
        const text = err instanceof Error ? err.message : '查询轨迹失败';
        setTrackError(text);
        message.error(text);
      } finally {
        setTrackLoading(false);
      }
    } catch (err) {
      const text = err instanceof Error ? err.message : '时间范围无效';
      setTrackError(text);
      message.error(text);
    }
  }

  function clearFilters() {
    setStartAt('');
    setEndAt('');
    setTrackError('');
  }

  function applyRangePreset(hours: number) {
    const end = new Date();
    const start = new Date(end.getTime() - hours * 3600_000);
    setStartAt(formatDateTimeLocalInput(start));
    setEndAt(formatDateTimeLocalInput(end));
    setTrackError('');
  }

  function handleShipChange(value: number) {
    setShipId(value);
    setTrack([]);
    setTrackError('');
    setLastQueriedAt('');
  }

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <Typography.Title level={3}>轨迹回放</Typography.Title>
        <Space wrap>
          <Select
            value={shipId}
            style={{ width: 240 }}
            showSearch
            optionFilterProp="label"
            loading={shipsLoading}
            disabled={shipsLoading || ships.length === 0}
            options={shipOptions}
            onChange={handleShipChange}
          />
          <Input type="datetime-local" value={startAt} style={{ width: 220 }} onChange={(event) => setStartAt(event.target.value)} />
          <Input type="datetime-local" value={endAt} style={{ width: 220 }} onChange={(event) => setEndAt(event.target.value)} />
          <Button type="primary" icon={<Search size={16} />} loading={trackLoading} disabled={!shipId} onClick={() => void loadTrack()}>
            查询
          </Button>
          <Button onClick={() => applyRangePreset(1)}>近 1 小时</Button>
          <Button onClick={() => applyRangePreset(6)}>近 6 小时</Button>
          <Button onClick={() => applyRangePreset(24)}>近 24 小时</Button>
          <Button onClick={clearFilters}>清空时间</Button>
          <Button icon={<RefreshCw size={16} />} loading={shipsLoading} onClick={() => void loadShips()} />
        </Space>
      </div>
      {error && (
        <Alert
          type="error"
          showIcon
          message="船舶列表加载失败"
          description={error}
          action={<Button onClick={() => void loadShips()}>重试</Button>}
        />
      )}
      {trackError && (
        <Alert
          type="error"
          showIcon
          message="轨迹查询失败"
          description={trackError}
          action={<Button onClick={() => void loadTrack()}>重试</Button>}
        />
      )}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}>
          <Card loading={trackLoading}>
            <Statistic title="轨迹点数" value={trackStats.pointCount} />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <Card loading={trackLoading}>
            <Statistic title="覆盖时长" value={trackStats.duration} />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <Card loading={trackLoading}>
            <Statistic title="累计航程" value={trackStats.totalDistanceNm.toFixed(1)} suffix="海里" />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <Card loading={trackLoading}>
            <Statistic title="最新航速" value={trackStats.latestSpeedKnots.toFixed(1)} suffix="节" />
          </Card>
        </Col>
      </Row>
      <Space direction="vertical" size={4}>
        <Typography.Text type="secondary">
          当前船舶：{selectedShip ? `${selectedShip.name} / ${selectedShip.mmsi}` : '未选择'}
        </Typography.Text>
        <Typography.Text type="secondary">
          时间范围：{trackStats.pointCount > 0 ? `${formatDateTime(trackStats.startedAt)} 至 ${formatDateTime(trackStats.endedAt)}` : '尚未查询到轨迹数据'}
        </Typography.Text>
        <Typography.Text type="secondary">最近查询：{formatDateTime(lastQueriedAt)}</Typography.Text>
      </Space>
      <div className="track-map">
        <MonitorMap locations={orderedTrack.length ? [orderedTrack[orderedTrack.length - 1]] : []} track={orderedTrack} fitMode="fit-data" />
      </div>
      <Table
        rowKey="id"
        size="small"
        loading={trackLoading}
        dataSource={tableTrack}
        pagination={{
          showSizeChanger: true,
          showTotal: (value) => `共 ${value} 个轨迹点`,
        }}
        locale={{
          emptyText: trackError ? '轨迹查询失败，请重试' : '暂无轨迹数据',
        }}
        columns={[
          { title: '上报时间', render: (_, item) => formatDateTime(item.reportedAt) },
          { title: '经度', render: (_, item) => item.longitude.toFixed(4) },
          { title: '纬度', render: (_, item) => item.latitude.toFixed(4) },
          { title: '航速', render: (_, item) => `${item.speedKnots.toFixed(1)} 节` },
          { title: '航向', render: (_, item) => `${item.course.toFixed(1)}°` },
        ]}
      />
    </div>
  );
}
