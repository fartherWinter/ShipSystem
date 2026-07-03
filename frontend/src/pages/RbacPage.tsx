import { Alert, Button, Card, Col, Input, Row, Select, Space, Statistic, Table, Tag, Typography, message } from 'antd';
import { RefreshCw } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type { Menu, Role, User } from '../types';

function userStatusColor(status: string) {
  return status === 'enabled' || status === 'active' ? 'green' : 'default';
}

export default function RbacPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [menus, setMenus] = useState<Menu[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [userKeyword, setUserKeyword] = useState('');
  const [roleFilter, setRoleFilter] = useState('');
  const [menuKeyword, setMenuKeyword] = useState('');

  const roleUserCount = useMemo(() => {
    const counts = new Map<number, number>();
    users.forEach((user) => {
      counts.set(user.roleId, (counts.get(user.roleId) ?? 0) + 1);
    });
    return counts;
  }, [users]);

  const filteredUsers = useMemo(() => {
    const keyword = userKeyword.trim().toLowerCase();
    return users.filter((user) => {
      if (roleFilter && user.role?.code !== roleFilter) {
        return false;
      }
      if (!keyword) {
        return true;
      }
      const haystack = [user.username, user.displayName, user.role?.name, user.role?.code, user.status]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(keyword);
    });
  }, [roleFilter, userKeyword, users]);

  const filteredMenus = useMemo(() => {
    const keyword = menuKeyword.trim().toLowerCase();
    return [...menus]
      .sort((left, right) => left.sort - right.sort || left.id - right.id)
      .filter((menu) => {
        if (!keyword) {
          return true;
        }
        const haystack = [menu.name, menu.path, menu.icon, String(menu.parentId ?? '')].join(' ').toLowerCase();
        return haystack.includes(keyword);
      });
  }, [menuKeyword, menus]);

  const enabledUserCount = useMemo(() => users.filter((user) => user.status === 'enabled' || user.status === 'active').length, [users]);

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
    void load();
  }, []);

  return (
    <div className="page-stack">
      <div className="page-toolbar">
        <Typography.Title level={3}>权限管理</Typography.Title>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={load} />
      </div>
      {error && <Alert type="error" showIcon message="权限数据加载失败" description={error} action={<Button onClick={load}>重试</Button>} />}

      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="用户总数" value={users.length} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="启用用户" value={enabledUserCount} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card loading={loading}>
            <Statistic title="角色 / 菜单" value={`${roles.length} / ${menus.length}`} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card
            title="用户"
            extra={<Typography.Text type="secondary">显示 {filteredUsers.length} / {users.length}</Typography.Text>}
          >
            <Space wrap style={{ marginBottom: 16 }}>
              <Input.Search
                allowClear
                value={userKeyword}
                placeholder="搜索用户名、姓名、角色"
                style={{ width: 240 }}
                onChange={(event) => setUserKeyword(event.target.value)}
                onSearch={(value) => setUserKeyword(value)}
              />
              <Select
                allowClear
                value={roleFilter || undefined}
                placeholder="按角色筛选"
                style={{ width: 180 }}
                options={roles.map((role) => ({ label: role.name, value: role.code }))}
                onChange={(value) => setRoleFilter(value ?? '')}
              />
            </Space>
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              pagination={{ showSizeChanger: true, showTotal: (value) => `共 ${value} 人` }}
              dataSource={filteredUsers}
              locale={{
                emptyText: error ? '数据加载失败，请重试' : '暂无用户数据',
              }}
              columns={[
                { title: '用户名', dataIndex: 'username' },
                { title: '姓名', dataIndex: 'displayName' },
                {
                  title: '角色',
                  render: (_, item) => <Tag color="blue">{item.role?.name ?? '-'}</Tag>,
                },
                {
                  title: '状态',
                  render: (_, item) => <Tag color={userStatusColor(item.status)}>{item.status}</Tag>,
                },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card
            title="角色"
            extra={<Typography.Text type="secondary">共 {roles.length} 个角色</Typography.Text>}
          >
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
                { title: '说明', dataIndex: 'description', ellipsis: true },
                { title: '用户数', render: (_, item) => roleUserCount.get(item.id) ?? 0 },
              ]}
            />
          </Card>
        </Col>
        <Col span={24}>
          <Card
            title="菜单"
            extra={<Typography.Text type="secondary">显示 {filteredMenus.length} / {menus.length}</Typography.Text>}
          >
            <Space wrap style={{ marginBottom: 16 }}>
              <Input.Search
                allowClear
                value={menuKeyword}
                placeholder="搜索菜单名称、路径或图标"
                style={{ width: 280 }}
                onChange={(event) => setMenuKeyword(event.target.value)}
                onSearch={(value) => setMenuKeyword(value)}
              />
            </Space>
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              pagination={{ showSizeChanger: true, showTotal: (value) => `共 ${value} 项` }}
              dataSource={filteredMenus}
              locale={{
                emptyText: error ? '数据加载失败，请重试' : '暂无菜单数据',
              }}
              columns={[
                { title: '名称', dataIndex: 'name' },
                { title: '路径', dataIndex: 'path' },
                { title: '图标', dataIndex: 'icon' },
                { title: '父级 ID', render: (_, item) => item.parentId ?? '-' },
                { title: '排序', dataIndex: 'sort' },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
