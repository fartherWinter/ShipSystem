import { Alert, Button, Card, List, Space, Tag, Typography, message } from 'antd';
import { Play, Radio } from 'lucide-react';
import { useState } from 'react';
import MonitorMap from '../components/MonitorMap';
import { startSimulator } from '../api/client';
import type { ShipLocation } from '../types';

type Props = {
  locations: ShipLocation[];
  wsStatus: string;
};

export default function MonitorPage({ locations, wsStatus }: Props) {
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState('');

  async function start() {
    setStarting(true);
    setError('');
    try {
      await startSimulator();
      message.success('模拟器已启动');
    } catch (err) {
      const text = err instanceof Error ? err.message : '启动模拟器失败';
      setError(text);
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
              {error && <Alert type="error" showIcon message="模拟器启动失败" description={error} action={<Button onClick={start}>重试</Button>} />}
              <Button type="primary" icon={<Play size={16} />} loading={starting} onClick={start}>
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
                    #{item.shipId} {item.longitude.toFixed(4)}, {item.latitude.toFixed(4)} / {item.speedKnots.toFixed(1)} 节
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
