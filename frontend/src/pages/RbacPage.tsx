import { Alert, Button, Card, Col, Row, Table, Tag, Typography, message } from 'antd';
import { RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { Menu, Role, User } from '../types';

export default function RbacPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [menus, setMenus] = useState<Menu[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [u, r, m] = await Promise.all([api.users(), api.roles(), api.menus()]);
      setUsers(u.items);
      setRoles(r.items);
      setMenus(m.items);
    } catch (err) {
      const text = err instanceof Error ? err.message : '加载权限数据失败';
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
        <Typography.Title level={3}>权限管理</Typography.Title>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={load} />
      </div>
      {error && <Alert type="error" showIcon message="权限数据加载失败" description={error} action={<Button onClick={load}>重试</Button>} />}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card title="用户">
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              pagination={false}
              dataSource={users}
              locale={{
                emptyText: error ? '数据加载失败，请重试' : '暂无用户数据',
              }}
              columns={[
                { title: '用户名', dataIndex: 'username' },
                { title: '姓名', dataIndex: 'displayName' },
                { title: '角色', render: (_, item) => <Tag>{item.role?.name}</Tag> },
                { title: '状态', dataIndex: 'status' },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="角色">
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              pagination={false}
              dataSource={roles}
              locale={{
                emptyText: error ? '数据加载失败，请重试' : '暂无角色数据',
              }}
              columns={[
                { title: '角色名', dataIndex: 'name' },
                { title: '编码', dataIndex: 'code' },
                { title: '说明', dataIndex: 'description' },
              ]}
            />
          </Card>
        </Col>
        <Col span={24}>
          <Card title="菜单">
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              pagination={false}
              dataSource={menus}
              locale={{
                emptyText: error ? '数据加载失败，请重试' : '暂无菜单数据',
              }}
              columns={[
                { title: '名称', dataIndex: 'name' },
                { title: '路径', dataIndex: 'path' },
                { title: '图标', dataIndex: 'icon' },
                { title: '排序', dataIndex: 'sort' },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
