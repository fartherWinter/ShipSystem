import { Alert, Button, Input, Select, Space, Table, Typography, message } from 'antd';
import { RefreshCw, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
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

function formatDateTime(value: string) {
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

  async function loadShips() {
    setShipsLoading(true);
    setError('');
    try {
      const res = await api.ships();
      setShips(res.items);
      setShipId((current) => (current && res.items.some((ship) => ship.id === current) ? current : res.items[0]?.id));
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载船舶列表失败';
      setError(text);
      message.error(text);
    } finally {
      setShipsLoading(false);
    }
  }

  useEffect(() => {
    loadShips();
  }, []);

  async function loadTrack() {
    if (!shipId) return;
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

  function handleShipChange(value: number) {
    setShipId(value);
    setTrack([]);
    setTrackError('');
  }

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <Typography.Title level={3}>轨迹回放</Typography.Title>
        <Space wrap>
          <Select
            value={shipId}
            style={{ width: 220 }}
            showSearch
            optionFilterProp="label"
            loading={shipsLoading}
            disabled={shipsLoading || ships.length === 0}
            options={ships.map((ship) => ({ label: ship.name, value: ship.id }))}
            onChange={handleShipChange}
          />
          <Input
            type="datetime-local"
            value={startAt}
            style={{ width: 220 }}
            onChange={(event) => setStartAt(event.target.value)}
          />
          <Input
            type="datetime-local"
            value={endAt}
            style={{ width: 220 }}
            onChange={(event) => setEndAt(event.target.value)}
          />
          <Button type="primary" icon={<Search size={16} />} loading={trackLoading} disabled={!shipId} onClick={loadTrack}>
            查询
          </Button>
          <Button onClick={clearFilters}>清空时间</Button>
          <Button icon={<RefreshCw size={16} />} loading={shipsLoading} onClick={loadShips} />
        </Space>
      </div>
      {error && <Alert type="error" showIcon message="船舶列表加载失败" description={error} action={<Button onClick={loadShips}>重试</Button>} />}
      {trackError && <Alert type="error" showIcon message="轨迹查询失败" description={trackError} action={<Button onClick={loadTrack}>重试</Button>} />}
      {!!track.length && (
        <Typography.Text type="secondary">
          当前共加载 {track.length} 个轨迹点
          {startAt || endAt ? '，已按时间范围过滤' : ''}
        </Typography.Text>
      )}
      <div className="track-map">
        <MonitorMap locations={track.slice(-1)} track={track} />
      </div>
      <Table
        rowKey="id"
        size="small"
        loading={trackLoading}
        dataSource={track}
        locale={{
          emptyText: trackError ? '轨迹查询失败，请重试' : '暂无轨迹数据',
        }}
        columns={[
          { title: '时间', render: (_, item) => formatDateTime(item.reportedAt) },
          { title: '经度', render: (_, item) => item.longitude.toFixed(4) },
          { title: '纬度', render: (_, item) => item.latitude.toFixed(4) },
          { title: '航速', render: (_, item) => `${item.speedKnots.toFixed(1)} 节` },
          { title: '航向', render: (_, item) => `${item.course.toFixed(1)}°` },
        ]}
      />
    </div>
  );
}
