import { Alert, Button, Card, Col, Row, Statistic, Table, Tag, Typography, message } from 'antd';
import { RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { Alarm, DispatchEvent, Ship } from '../types';

export default function DashboardPage() {
  const [ships, setShips] = useState<Ship[]>([]);
  const [alarms, setAlarms] = useState<Alarm[]>([]);
  const [events, setEvents] = useState<DispatchEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [shipRes, alarmRes, eventRes] = await Promise.all([api.ships(), api.alarms('OPEN'), api.dispatchEvents()]);
      setShips(shipRes.items);
      setAlarms(alarmRes.items);
      setEvents(eventRes.items);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载首页态势失败';
      setError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <Typography.Title level={3}>首页态势</Typography.Title>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={load} />
      </div>
      {error && <Alert type="error" showIcon message="首页态势加载失败" description={error} action={<Button onClick={load}>重试</Button>} />}
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="船舶数量" value={ships.length} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="未确认告警" value={alarms.length} valueStyle={{ color: alarms.length ? '#b91c1c' : '#0f766e' }} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="调度事件" value={events.length} />
          </Card>
        </Col>
      </Row>
      <Card title="最新告警">
        <Table
          rowKey="id"
          size="small"
          loading={loading}
          pagination={false}
          dataSource={alarms.slice(0, 5)}
          locale={{
            emptyText: error ? '数据加载失败，请重试' : '暂无未确认告警',
          }}
          columns={[
            { title: '船舶', render: (_, item) => item.ship?.name ?? item.shipId },
            { title: '类型', dataIndex: 'type' },
            { title: '级别', render: (_, item) => <Tag color="red">{item.level}</Tag> },
            { title: '内容', dataIndex: 'message' },
          ]}
        />
      </Card>
    </div>
  );
}
