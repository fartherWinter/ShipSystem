import { Alert, Button, Select, Space, Table, Typography, message } from 'antd';
import { RefreshCw, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import MonitorMap from '../components/MonitorMap';
import type { Ship, ShipLocation } from '../types';

export default function TracksPage() {
  const [ships, setShips] = useState<Ship[]>([]);
  const [shipId, setShipId] = useState<number>();
  const [track, setTrack] = useState<ShipLocation[]>([]);
  const [shipsLoading, setShipsLoading] = useState(false);
  const [trackLoading, setTrackLoading] = useState(false);
  const [error, setError] = useState('');
  const [trackError, setTrackError] = useState('');

  async function loadShips() {
    setShipsLoading(true);
    setError('');
    try {
      const res = await api.ships();
      setShips(res.items);
      setShipId((current) => current ?? res.items[0]?.id);
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
    setTrackLoading(true);
    setTrackError('');
    try {
      const data = await api.tracks(shipId);
      setTrack(data.items);
    } catch (err) {
      const text = err instanceof Error ? err.message : '查询轨迹失败';
      setTrackError(text);
      message.error(text);
    } finally {
      setTrackLoading(false);
    }
  }

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <Typography.Title level={3}>轨迹回放</Typography.Title>
        <Space>
          <Select
            value={shipId}
            style={{ width: 220 }}
            loading={shipsLoading}
            disabled={shipsLoading || ships.length === 0}
            options={ships.map((ship) => ({ label: ship.name, value: ship.id }))}
            onChange={setShipId}
          />
          <Button type="primary" icon={<Search size={16} />} loading={trackLoading} disabled={!shipId} onClick={loadTrack}>
            查询
          </Button>
          <Button icon={<RefreshCw size={16} />} loading={shipsLoading} onClick={loadShips} />
        </Space>
      </div>
      {error && <Alert type="error" showIcon message="船舶列表加载失败" description={error} action={<Button onClick={loadShips}>重试</Button>} />}
      {trackError && <Alert type="error" showIcon message="轨迹查询失败" description={trackError} action={<Button onClick={loadTrack}>重试</Button>} />}
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
          { title: '时间', dataIndex: 'reportedAt' },
          { title: '经度', dataIndex: 'longitude' },
          { title: '纬度', dataIndex: 'latitude' },
          { title: '航速', dataIndex: 'speedKnots' },
          { title: '航向', dataIndex: 'course' },
        ]}
      />
    </div>
  );
}
