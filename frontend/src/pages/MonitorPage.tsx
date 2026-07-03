import { Alert, Button, Card, List, Select, Space, Tag, Typography, message } from 'antd';
import { Play, Radio } from 'lucide-react';
import { useEffect, useState } from 'react';
import MonitorMap from '../components/MonitorMap';
import { api, startSimulator } from '../api/client';
import type { Ship, ShipLocation } from '../types';

type Props = {
  locations: ShipLocation[];
  wsStatus: string;
};

export default function MonitorPage({ locations, wsStatus }: Props) {
  const [ships, setShips] = useState<Ship[]>([]);
  const [selectedShipIds, setSelectedShipIds] = useState<number[]>([]);
  const [shipsLoading, setShipsLoading] = useState(false);
  const [shipsError, setShipsError] = useState('');
  const [starting, setStarting] = useState(false);
  const [startError, setStartError] = useState('');

  useEffect(() => {
    loadShips();
  }, []);

  async function loadShips() {
    setShipsLoading(true);
    setShipsError('');
    try {
      const data = await api.ships();
      setShips(data.items);
      setSelectedShipIds((current) => {
        const next = current.filter((id) => data.items.some((ship) => ship.id === id));
        return next.length ? next : data.items.slice(0, 2).map((ship) => ship.id);
      });
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载模拟船舶失败';
      setShipsError(text);
      message.error(text);
    } finally {
      setShipsLoading(false);
    }
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
    try {
      await startSimulator(selectedShipIds);
      message.success('模拟器已启动');
    } catch (err) {
      const text = err instanceof Error ? err.message : '启动模拟器失败';
      setStartError(text);
      message.error(text);
    } finally {
      setStarting(false);
    }
  }

  return (
    <div className="monitor-layout">
      <div className="map-panel">
        <MonitorMap locations={locations} />
      </div>
      <aside className="side-panel">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Card title="实时连接">
            <Space>
              <Radio size={18} />
              <Tag color={wsStatus === 'connected' ? 'green' : 'red'}>{wsStatus}</Tag>
            </Space>
          </Card>
          <Card title="模拟数据">
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              {shipsError && <Alert type="error" showIcon message="模拟船舶加载失败" description={shipsError} action={<Button onClick={loadShips}>重试</Button>} />}
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
                onChange={setSelectedShipIds}
              />
              <Space>
                <Button onClick={loadShips} loading={shipsLoading}>
                  刷新船舶
                </Button>
                <Typography.Text type="secondary">已选 {selectedShipIds.length} 艘</Typography.Text>
              </Space>
              <Button type="primary" icon={<Play size={16} />} loading={starting} disabled={selectedShipIds.length === 0} onClick={start}>
                启动模拟器
              </Button>
            </Space>
          </Card>
          <Card title="最新位置">
            <List
              size="small"
              dataSource={locations}
              locale={{ emptyText: '暂无实时点位' }}
              renderItem={(item) => (
                <List.Item>
                  <Typography.Text>
                    {ships.find((ship) => ship.id === item.shipId)?.name ?? `#${item.shipId}`} {item.longitude.toFixed(4)}, {item.latitude.toFixed(4)} /{' '}
                    {item.speedKnots.toFixed(1)} 节
                  </Typography.Text>
                </List.Item>
              )}
            />
          </Card>
        </Space>
      </aside>
    </div>
  );
}
