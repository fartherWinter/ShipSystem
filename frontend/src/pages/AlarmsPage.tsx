import { Alert, Button, Select, Space, Table, Tag, Typography, message } from 'antd';
import { Check, RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { Alarm } from '../types';

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
  });
}

export default function AlarmsPage() {
  const [status, setStatus] = useState('');
  const [items, setItems] = useState<Alarm[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [ackingId, setAckingId] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);

  async function load(nextStatus = status, nextPage = page, nextPageSize = pageSize) {
    setLoading(true);
    setError('');
    try {
      const data = await api.alarms(nextStatus, { page: nextPage, size: nextPageSize });
      setItems(data.items);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载告警列表失败';
      setError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load('', 1, pageSize);
  }, []);

  async function ack(id: number) {
    setAckingId(id);
    try {
      await api.ackAlarm(id);
      message.success('告警已确认');
      const nextPage = page > 1 && items.length === 1 ? page - 1 : page;
      await load(status, nextPage, pageSize);
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
              void load(value, 1, pageSize);
            }}
          />
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load(status, page, pageSize)} />
        </Space>
      </div>
      {error && <Alert type="error" showIcon message="告警列表加载失败" description={error} action={<Button onClick={() => void load(status, page, pageSize)}>重试</Button>} />}
      <Typography.Text type="secondary">当前显示 {items.length} / {total} 条告警</Typography.Text>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={items}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (value) => `共 ${value} 条`,
          onChange: (nextPage, nextPageSize) => {
            void load(status, nextPage, nextPageSize);
          },
        }}
        locale={{
          emptyText: error ? '列表加载失败，请重试' : '暂无告警数据',
        }}
        columns={[
          { title: '船舶', render: (_, item) => item.ship?.name ?? item.shipId },
          { title: '标题', dataIndex: 'title' },
          { title: '类型', dataIndex: 'type' },
          { title: '级别', render: (_, item) => <Tag color={item.level === 'WARN' ? 'orange' : 'red'}>{item.level}</Tag> },
          { title: '状态', render: (_, item) => <Tag color={item.status === 'OPEN' ? 'red' : 'green'}>{item.status}</Tag> },
          { title: '告警时间', render: (_, item) => formatDateTime(item.createdAt) },
          { title: '确认时间', render: (_, item) => formatDateTime(item.ackAt) },
          { title: '内容', dataIndex: 'message', ellipsis: true },
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
