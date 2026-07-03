import { Alert, Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Table, Tag, message } from 'antd';
import { Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { Ship } from '../types';

export default function ShipsPage() {
  const [items, setItems] = useState<Ship[]>([]);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [editing, setEditing] = useState<Ship | null>(null);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();

  async function load(nextKeyword = keyword) {
    const normalizedKeyword = nextKeyword.trim();
    setLoading(true);
    setError('');
    try {
      const data = await api.ships(normalizedKeyword);
      setItems(data.items);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载船舶列表失败';
      setError(text);
      message.error(text);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  function showModal(ship?: Ship) {
    setEditing(ship ?? null);
    form.resetFields();
    form.setFieldsValue(ship ?? { status: 'active', flag: 'CN' });
    setOpen(true);
  }

  function closeModal() {
    setOpen(false);
    setEditing(null);
    form.resetFields();
  }

  async function save() {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await api.updateShip(editing.id, values);
        message.success('船舶已更新');
      } else {
        await api.createShip(values);
        message.success('船舶已创建');
      }
      closeModal();
      await load();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '保存船舶失败');
    } finally {
      setSaving(false);
    }
  }

  async function remove(id: number) {
    setDeletingId(id);
    try {
      await api.deleteShip(id);
      message.success('船舶已删除');
      await load();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '删除船舶失败');
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <h2>船舶管理</h2>
        <Space wrap>
          <Input.Search
            allowClear
            value={keyword}
            placeholder="按船名或 MMSI 搜索"
            style={{ width: 280 }}
            onChange={(event) => setKeyword(event.target.value)}
            onSearch={(value) => {
              const nextKeyword = value.trim();
              setKeyword(nextKeyword);
              load(nextKeyword);
            }}
          />
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => load()} />
          <Button type="primary" icon={<Plus size={16} />} onClick={() => showModal()}>
            新增船舶
          </Button>
        </Space>
      </div>
      {error && <Alert type="error" showIcon message="船舶列表加载失败" description={error} action={<Button onClick={() => load()}>重试</Button>} />}
      <Table
        rowKey="id"
        loading={loading}
        dataSource={items}
        locale={{
          emptyText: error ? '列表加载失败，请重试' : '暂无船舶数据',
        }}
        columns={[
          { title: '船名', dataIndex: 'name' },
          { title: 'MMSI', dataIndex: 'mmsi' },
          { title: '类型', dataIndex: 'shipType' },
          { title: '船旗', dataIndex: 'flag' },
          { title: '尺度', render: (_, item) => `${item.lengthM || 0}m x ${item.widthM || 0}m` },
          { title: '状态', render: (_, item) => <Tag color={item.status === 'active' ? 'green' : 'default'}>{item.status}</Tag> },
          {
            title: '操作',
            width: 150,
            render: (_, item) => (
              <Space>
                <Button icon={<Pencil size={16} />} onClick={() => showModal(item)} />
                <Popconfirm title="确认删除该船舶？" onConfirm={() => remove(item.id)}>
                  <Button danger icon={<Trash2 size={16} />} loading={deletingId === item.id} />
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Modal
        title={editing ? '编辑船舶' : '新增船舶'}
        open={open}
        confirmLoading={saving}
        onOk={save}
        onCancel={closeModal}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="船名" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="mmsi"
            label="MMSI"
            rules={[
              { required: true },
              { pattern: /^\d{9}$/, message: 'MMSI 必须为 9 位数字' },
            ]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="shipType" label="类型">
            <Input />
          </Form.Item>
          <Form.Item name="flag" label="船旗">
            <Input />
          </Form.Item>
          <Space>
            <Form.Item name="lengthM" label="船长">
              <InputNumber min={0} max={500} />
            </Form.Item>
            <Form.Item name="widthM" label="船宽">
              <InputNumber min={0} max={100} />
            </Form.Item>
          </Space>
          <Form.Item name="status" label="状态">
            <Select
              options={[
                { label: '在航', value: 'active' },
                { label: '停用', value: 'inactive' },
                { label: '维护', value: 'maintenance' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
