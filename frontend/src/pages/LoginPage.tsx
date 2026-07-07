import { Alert, Button, Form, Input, Typography, message } from 'antd';
import { Anchor, Lock, User } from 'lucide-react';
import { useState } from 'react';
import { api } from '../api/client';
import type { LoginResponse } from '../types';

type Props = {
  onLogin: (data: LoginResponse) => void;
};

export default function LoginPage({ onLogin }: Props) {
  const [form] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  async function submit(values: { username: string; password: string }) {
    setSubmitting(true);
    setError('');
    try {
      const data = await api.login(values.username, values.password);
      onLogin(data);
    } catch (err) {
      const text = err instanceof Error ? err.message : '登录失败';
      setError(text);
      message.error(text);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-panel">
        <div className="login-title">
          <Anchor size={30} />
          <div>
            <Typography.Title level={2}>船舶管理与监控调度系统</Typography.Title>
            <Typography.Text type="secondary">默认账号 `admin` / `Admin123!`</Typography.Text>
          </div>
        </div>
        {error && <Alert type="error" showIcon message="登录失败" description={error} className="login-alert" />}
        <Form layout="vertical" form={form} initialValues={{ username: 'admin', password: 'Admin123!' }} onFinish={submit}>
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}>
            <Input prefix={<User size={16} />} />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }]}>
            <Input.Password prefix={<Lock size={16} />} />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={submitting}>
            登录
          </Button>
        </Form>
      </section>
    </main>
  );
}
