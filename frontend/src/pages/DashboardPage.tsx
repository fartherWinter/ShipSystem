import { Alert, Button, Card, Col, Row, Statistic, Table, Tag, Typography, message } from 'antd';
import { RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { Alarm, DispatchEvent } from '../types';

function priorityColor(priority: string) {
  return priority === 'high' ? 'red' : 'blue';
}

function statusColor(status: string) {
  switch (status) {
    case 'NEW':
      return 'blue';
    case 'DISPATCHED':
      return 'cyan';
    case 'PROCESSING':
      return 'orange';
    case 'COMPLETED':
      return 'green';
    case 'CANCELLED':
      return 'default';
    default:
      return 'default';
  }
}

function formatDateTime(value: string) {
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
  });
}

export default function DashboardPage() {
  const [shipTotal, setShipTotal] = useState(0);
  const [alarmTotal, setAlarmTotal] = useState(0);
  const [eventTotal, setEventTotal] = useState(0);
  const [alarms, setAlarms] = useState<Alarm[]>([]);
  const [events, setEvents] = useState<DispatchEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [shipRes, alarmRes, eventRes] = await Promise.all([
        api.ships('', { page: 1, size: 1 }),
        api.alarms('OPEN', { page: 1, size: 5 }),
        api.dispatchEvents('', { page: 1, size: 5 }),
      ]);
      setShipTotal(shipRes.total);
      setAlarmTotal(alarmRes.total);
      setEventTotal(eventRes.total);
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
    void load();
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
            <Statistic title="船舶总量" value={shipTotal} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="未确认告警" value={alarmTotal} valueStyle={{ color: alarmTotal ? '#b91c1c' : '#0f766e' }} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="调度事件总数" value={eventTotal} />
          </Card>
        </Col>
      </Row>
      <Row gutter={[16, 16]}>
        <Col xs={24} xl={12}>
          <Card title="最新告警">
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              pagination={false}
              dataSource={alarms}
              locale={{
                emptyText: error ? '数据加载失败，请重试' : '暂无未确认告警',
              }}
              columns={[
                { title: '船舶', render: (_, item) => item.ship?.name ?? item.shipId },
                { title: '类型', dataIndex: 'type' },
                { title: '级别', render: (_, item) => <Tag color={item.level === 'WARN' ? 'orange' : 'red'}>{item.level}</Tag> },
                { title: '内容', dataIndex: 'message', ellipsis: true },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title="最近调度事件">
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              pagination={false}
              dataSource={events}
              locale={{
                emptyText: error ? '数据加载失败，请重试' : '暂无调度事件',
              }}
              columns={[
                { title: '船舶', render: (_, item) => item.ship?.name ?? '-' },
                { title: '标题', dataIndex: 'title', ellipsis: true },
                { title: '优先级', render: (_, item) => <Tag color={priorityColor(item.priority)}>{item.priority}</Tag> },
                { title: '状态', render: (_, item) => <Tag color={statusColor(item.status)}>{item.status}</Tag> },
                { title: '创建时间', render: (_, item) => formatDateTime(item.createdAt) },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
