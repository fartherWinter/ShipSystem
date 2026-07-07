import { Button, Layout, Menu, Space, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';
import { Gauge, LogOut, RadioTower } from 'lucide-react';
import type { User } from '../types';

const { Header, Content, Sider } = Layout;

export type NavItem = {
  key: string;
  label: string;
  icon: ReactNode;
};

type Props = {
  currentPath: string;
  onNavigate: (path: string) => void;
  onLogout: () => void;
  user: User;
  wsStatus: string;
  items: NavItem[];
  children: ReactNode;
};

export default function AppShell({ currentPath, onNavigate, onLogout, user, wsStatus, items, children }: Props) {
  return (
    <Layout className="app-shell">
      <Sider width={236} theme="light" className="app-sider">
        <div className="brand">
          <RadioTower size={24} />
          <div>
            <Typography.Text strong>船舶监控调度</Typography.Text>
            <Typography.Text type="secondary">ShipSystem</Typography.Text>
          </div>
        </div>
        <Menu mode="inline" selectedKeys={[currentPath]} items={items} onClick={(event) => onNavigate(event.key)} />
      </Sider>
      <Layout>
        <Header className="app-header">
          <Space>
            <Gauge size={18} />
            <Tag color={wsStatus === 'connected' ? 'green' : wsStatus === 'connecting' ? 'blue' : 'red'}>
              {wsStatus === 'connected' ? '实时已连接' : wsStatus === 'connecting' ? '正在连接' : '实时未连接'}
            </Tag>
          </Space>
          <Space>
            <Typography.Text>{user.displayName}</Typography.Text>
            <Tag>{user.role?.name ?? '未知角色'}</Tag>
            <Button icon={<LogOut size={16} />} onClick={onLogout}>
              退出
            </Button>
          </Space>
        </Header>
        <Content className="app-content">{children}</Content>
      </Layout>
    </Layout>
  );
}
