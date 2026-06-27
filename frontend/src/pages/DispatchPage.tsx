import { Alert, Button, Form, Input, Modal, Select, Space, Table, Tag, Typography, message } from 'antd';
import { Plus, RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { DispatchEvent } from '../types';

const statuses = ['NEW', 'DISPATCHED', 'PROCESSING', 'COMPLETED', 'CANCELLED'];

type Props = {
  items: DispatchEvent[];
  onItemsChange: (items: DispatchEvent[]) => void;
};

export default function DispatchPage({ items, onItemsChange }: Props) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [updatingId, setUpdatingId] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();

  async function load() {
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

  useEffect(() => {
    load();
  }, []);

  async function create() {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await api.createDispatchEvent(values);
      message.success('调度事件已创建');
      setOpen(false);
      form.resetFields();
      await load();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '创建调度事件失败');
    } finally {
      setSaving(false);
    }
  }

  async function updateStatus(id: number, status: string) {
    setUpdatingId(id);
    try {
      await api.updateDispatchStatus(id, { status, remark: '前端调度流转' });
      message.success('状态已更新');
      await load();
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
        <Space>
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={load} />
          <Button type="primary" icon={<Plus size={16} />} onClick={() => setOpen(true)}>
            新建事件
          </Button>
        </Space>
      </div>
      {error && <Alert type="error" showIcon message="调度事件加载失败" description={error} action={<Button onClick={load}>重试</Button>} />}
      <Table
        rowKey="id"
        loading={loading}
        dataSource={items}
        locale={{
          emptyText: error ? '列表加载失败，请重试' : '暂无调度事件',
        }}
        columns={[
          { title: '标题', dataIndex: 'title' },
          { title: '优先级', render: (_, item) => <Tag color={item.priority === 'high' ? 'red' : 'blue'}>{item.priority}</Tag> },
          { title: '状态', render: (_, item) => <Tag>{item.status}</Tag> },
          { title: '说明', dataIndex: 'description' },
          {
            title: '流转',
            width: 190,
            render: (_, item) => (
              <Select
                value={item.status}
                style={{ width: 170 }}
                loading={updatingId === item.id}
                disabled={updatingId === item.id}
                options={statuses.map((status) => ({ label: status, value: status }))}
                onChange={(status) => updateStatus(item.id, status)}
              />
            ),
          },
        ]}
      />
      <Modal title="新建调度事件" open={open} confirmLoading={saving} onOk={create} onCancel={() => setOpen(false)} destroyOnHidden>
        <Form form={form} layout="vertical" initialValues={{ priority: 'normal' }}>
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="priority" label="优先级">
            <Select
              options={[
                { label: '普通', value: 'normal' },
                { label: '高', value: 'high' },
              ]}
            />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={4} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
