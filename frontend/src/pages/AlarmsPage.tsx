import { Alert, Button, Select, Space, Table, Tag, Typography, message } from 'antd';
import { Check, RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { Alarm } from '../types';

export default function AlarmsPage() {
  const [status, setStatus] = useState('');
  const [items, setItems] = useState<Alarm[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [ackingId, setAckingId] = useState<number | null>(null);

  async function load(nextStatus = status) {
    setLoading(true);
    setError('');
    try {
      const data = await api.alarms(nextStatus);
      setItems(data.items);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载告警列表失败';
      setError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load('');
  }, []);

  async function ack(id: number) {
    setAckingId(id);
    try {
      await api.ackAlarm(id);
      message.success('告警已确认');
      await load();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '确认告警失败');
    } finally {
      setAckingId(null);
    }
  }

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <Typography.Title level={3}>告警中心</Typography.Title>
        <Space>
          <Select
            value={status}
            style={{ width: 150 }}
            options={[
              { label: '全部', value: '' },
              { label: '未确认', value: 'OPEN' },
              { label: '已确认', value: 'ACKED' },
            ]}
            onChange={(value) => {
              setStatus(value);
              load(value);
            }}
          />
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => load()} />
        </Space>
      </div>
      {error && <Alert type="error" showIcon message="告警列表加载失败" description={error} action={<Button onClick={() => load()}>重试</Button>} />}
      <Table
        rowKey="id"
        loading={loading}
        dataSource={items}
        locale={{
          emptyText: error ? '列表加载失败，请重试' : '暂无告警数据',
        }}
        columns={[
          { title: '船舶', render: (_, item) => item.ship?.name ?? item.shipId },
          { title: '标题', dataIndex: 'title' },
          { title: '类型', dataIndex: 'type' },
          { title: '级别', render: (_, item) => <Tag color={item.level === 'WARN' ? 'orange' : 'red'}>{item.level}</Tag> },
          { title: '状态', render: (_, item) => <Tag color={item.status === 'OPEN' ? 'red' : 'green'}>{item.status}</Tag> },
          { title: '内容', dataIndex: 'message' },
          {
            title: '操作',
            width: 100,
            render: (_, item) =>
              item.status === 'OPEN' ? (
                <Button icon={<Check size={16} />} loading={ackingId === item.id} onClick={() => ack(item.id)}>
                  确认
                </Button>
              ) : null,
          },
        ]}
      />
    </div>
  );
}
