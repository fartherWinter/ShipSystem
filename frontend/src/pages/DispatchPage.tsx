import { Alert, Button, Form, Input, Modal, Select, Space, Table, Tag, Typography, message } from 'antd';
import { Plus, RefreshCw } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type { DispatchEvent, Ship } from '../types';

const statuses = ['NEW', 'DISPATCHED', 'PROCESSING', 'COMPLETED', 'CANCELLED'];
const statusOptions = statuses.map((status) => ({ label: status, value: status }));
const priorityOptions = [
  { label: '普通', value: 'normal' },
  { label: '高', value: 'high' },
];

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

function priorityColor(priority: string) {
  return priority === 'high' ? 'red' : 'blue';
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
  });
}

type Props = {
  items: DispatchEvent[];
  onItemsChange: (items: DispatchEvent[]) => void;
};

export default function DispatchPage({ items, onItemsChange }: Props) {
  const [ships, setShips] = useState<Ship[]>([]);
  const [loading, setLoading] = useState(false);
  const [shipsLoading, setShipsLoading] = useState(false);
  const [error, setError] = useState('');
  const [shipsError, setShipsError] = useState('');
  const [saving, setSaving] = useState(false);
  const [updatingId, setUpdatingId] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const [statusOpen, setStatusOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [statusTarget, setStatusTarget] = useState<DispatchEvent | null>(null);
  const [form] = Form.useForm();
  const [statusForm] = Form.useForm();

  const filteredItems = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return items.filter((item) => {
      if (statusFilter && item.status !== statusFilter) {
        return false;
      }
      if (!normalizedKeyword) {
        return true;
      }
      const haystack = [item.title, item.description, item.ship?.name, item.priority, item.status].filter(Boolean).join(' ').toLowerCase();
      return haystack.includes(normalizedKeyword);
    });
  }, [items, keyword, statusFilter]);

  async function loadEvents() {
    setLoading(true);
    setError('');
    try {
      const data = await api.dispatchEvents();
      onItemsChange(data.items);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载调度事件失败';
      setError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  async function loadShips() {
    setShipsLoading(true);
    setShipsError('');
    try {
      const data = await api.ships();
      setShips(data.items);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载船舶列表失败';
      setShipsError(text);
      message.error(text);
    } finally {
      setShipsLoading(false);
    }
  }

  async function refreshAll() {
    await Promise.all([loadEvents(), loadShips()]);
  }

  useEffect(() => {
    refreshAll();
  }, []);

  function openCreateModal() {
    form.resetFields();
    form.setFieldsValue({ priority: 'normal' });
    setOpen(true);
  }

  function closeCreateModal() {
    setOpen(false);
    form.resetFields();
  }

  function openStatusModal(item: DispatchEvent, nextStatus: string) {
    setStatusTarget(item);
    statusForm.resetFields();
    statusForm.setFieldsValue({ status: nextStatus, remark: '' });
    setStatusOpen(true);
  }

  function closeStatusModal() {
    setStatusOpen(false);
    setStatusTarget(null);
    statusForm.resetFields();
  }

  async function create() {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await api.createDispatchEvent(values);
      message.success('调度事件已创建');
      closeCreateModal();
      await loadEvents();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '创建调度事件失败');
    } finally {
      setSaving(false);
    }
  }

  async function submitStatusUpdate() {
    if (!statusTarget) {
      return;
    }
    const values = await statusForm.validateFields();
    setUpdatingId(statusTarget.id);
    try {
      await api.updateDispatchStatus(statusTarget.id, {
        status: values.status,
        remark: values.remark?.trim() || undefined,
      });
      message.success('状态已更新');
      closeStatusModal();
      await loadEvents();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '更新调度状态失败');
    } finally {
      setUpdatingId(null);
    }
  }

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <Typography.Title level={3}>调度事件</Typography.Title>
        <Space wrap>
          <Input.Search
            allowClear
            value={keyword}
            placeholder="按标题、说明或船舶搜索"
            style={{ width: 260 }}
            onChange={(event) => setKeyword(event.target.value)}
            onSearch={(value) => setKeyword(value)}
          />
          <Select
            allowClear
            value={statusFilter || undefined}
            placeholder="筛选状态"
            style={{ width: 170 }}
            options={statusOptions}
            onChange={(value) => setStatusFilter(value ?? '')}
          />
          <Button icon={<RefreshCw size={16} />} loading={loading || shipsLoading} onClick={refreshAll} />
          <Button type="primary" icon={<Plus size={16} />} onClick={openCreateModal}>
            新建事件
          </Button>
        </Space>
      </div>
      {error && <Alert type="error" showIcon message="调度事件加载失败" description={error} action={<Button onClick={loadEvents}>重试</Button>} />}
      <Typography.Text type="secondary">当前显示 {filteredItems.length} / {items.length} 条调度事件</Typography.Text>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={filteredItems}
        locale={{
          emptyText: error ? '列表加载失败，请重试' : '暂无调度事件',
        }}
        columns={[
          { title: '关联船舶', render: (_, item) => item.ship?.name ?? '-' },
          { title: '标题', dataIndex: 'title' },
          { title: '优先级', render: (_, item) => <Tag color={priorityColor(item.priority)}>{item.priority}</Tag> },
          { title: '状态', render: (_, item) => <Tag color={statusColor(item.status)}>{item.status}</Tag> },
          { title: '创建时间', render: (_, item) => formatDateTime(item.createdAt) },
          { title: '说明', dataIndex: 'description', ellipsis: true },
          {
            title: '流转',
            width: 220,
            render: (_, item) => (
              <Select
                value={item.status}
                style={{ width: 190 }}
                loading={updatingId === item.id}
                disabled={updatingId === item.id}
                options={statusOptions}
                onChange={(status) => openStatusModal(item, status)}
              />
            ),
          },
        ]}
      />
      <Modal title="新建调度事件" open={open} confirmLoading={saving} onOk={create} onCancel={closeCreateModal} destroyOnHidden>
        <Form form={form} layout="vertical" initialValues={{ priority: 'normal' }}>
          {shipsError && <Alert type="error" showIcon message="关联船舶加载失败" description={shipsError} action={<Button onClick={loadShips}>重试</Button>} style={{ marginBottom: 16 }} />}
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="shipId" label="关联船舶">
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              loading={shipsLoading}
              placeholder="可选：绑定到具体船舶"
              options={ships.map((ship) => ({ label: `${ship.name} / ${ship.mmsi}`, value: ship.id }))}
            />
          </Form.Item>
          <Form.Item name="priority" label="优先级">
            <Select options={priorityOptions} />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={4} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title={statusTarget ? `流转事件：${statusTarget.title}` : '流转调度事件'}
        open={statusOpen}
        confirmLoading={statusTarget ? updatingId === statusTarget.id : false}
        onOk={submitStatusUpdate}
        onCancel={closeStatusModal}
        destroyOnHidden
      >
        <Form form={statusForm} layout="vertical">
          <Form.Item name="status" label="目标状态" rules={[{ required: true }]}>
            <Select options={statusOptions} />
          </Form.Item>
          <Form.Item name="remark" label="流转备注">
            <Input.TextArea rows={4} placeholder="可选：记录此次状态变更原因或交接信息" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
