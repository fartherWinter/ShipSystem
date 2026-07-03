import { Alert, Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Table, Tag, Typography, message } from 'antd';
import { Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type { Ship } from '../types';

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
    second: '2-digit',
  });
}

function compareShips(left: Ship, right: Ship) {
  const leftTime = Date.parse(left.updatedAt || left.createdAt);
  const rightTime = Date.parse(right.updatedAt || right.createdAt);
  if (Number.isNaN(leftTime) && Number.isNaN(rightTime)) {
    return right.id - left.id;
  }
  if (Number.isNaN(leftTime)) {
    return 1;
  }
  if (Number.isNaN(rightTime)) {
    return -1;
  }
  if (rightTime !== leftTime) {
    return rightTime - leftTime;
  }
  return right.id - left.id;
}

export default function ShipsPage() {
  const [items, setItems] = useState<Ship[]>([]);
  const [keyword, setKeyword] = useState('');
  const [queryKeyword, setQueryKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [editing, setEditing] = useState<Ship | null>(null);
  const [open, setOpen] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [lastUpdatedAt, setLastUpdatedAt] = useState('');
  const [form] = Form.useForm();

  const sortedItems = useMemo(() => [...items].sort(compareShips), [items]);

  async function load(
    nextKeyword = queryKeyword,
    nextPage = page,
    nextPageSize = pageSize,
    options?: { silent?: boolean },
  ) {
    const normalizedKeyword = nextKeyword.trim();
    const silent = options?.silent ?? false;
    setLoading(true);
    setError('');
    try {
      const data = await api.ships(normalizedKeyword, { page: nextPage, size: nextPageSize });
      setItems(data.items);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
      setQueryKeyword(normalizedKeyword);
      setLastUpdatedAt(new Date().toISOString());
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载船舶列表失败';
      setError(text);
      if (!silent) {
        message.error(text);
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load('', 1, pageSize);
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void load(queryKeyword, page, pageSize, { silent: true });
    }, 15000);
    return () => window.clearInterval(timer);
  }, [page, pageSize, queryKeyword]);

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
      await load(queryKeyword, page, pageSize);
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
      const nextPage = page > 1 && items.length === 1 ? page - 1 : page;
      await load(queryKeyword, nextPage, pageSize);
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
              void load(nextKeyword, 1, pageSize);
            }}
          />
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load(queryKeyword, page, pageSize)} />
          <Button type="primary" icon={<Plus size={16} />} onClick={() => showModal()}>
            新增船舶
          </Button>
        </Space>
      </div>
      {error && (
        <Alert
          type="error"
          showIcon
          message="船舶列表加载失败"
          description={error}
          action={<Button onClick={() => void load(queryKeyword, page, pageSize)}>重试</Button>}
        />
      )}
      <Space direction="vertical" size={4}>
        <Typography.Text type="secondary">当前显示 {sortedItems.length} / {total} 艘船舶</Typography.Text>
        <Typography.Text type="secondary">最近刷新：{formatDateTime(lastUpdatedAt)}</Typography.Text>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={sortedItems}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (value) => `共 ${value} 艘`,
          onChange: (nextPage, nextPageSize) => {
            void load(queryKeyword, nextPage, nextPageSize);
          },
        }}
        locale={{
          emptyText: error ? '列表加载失败，请重试' : '暂无船舶数据',
        }}
        columns={[
          { title: '船名', dataIndex: 'name' },
          { title: 'MMSI', dataIndex: 'mmsi' },
          { title: '类型', dataIndex: 'shipType', render: (value) => value || '-' },
          { title: '船旗', dataIndex: 'flag', render: (value) => value || '-' },
          { title: '尺度', render: (_, item) => `${item.lengthM || 0}m x ${item.widthM || 0}m` },
          {
            title: '状态',
            render: (_, item) => <Tag color={item.status === 'active' ? 'green' : item.status === 'maintenance' ? 'orange' : 'default'}>{item.status}</Tag>,
          },
          { title: '更新时间', render: (_, item) => formatDateTime(item.updatedAt) },
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
      <Modal title={editing ? '编辑船舶' : '新增船舶'} open={open} confirmLoading={saving} onOk={save} onCancel={closeModal} destroyOnHidden>
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
