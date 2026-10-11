'use client';

import { App, Button, Card, Divider, Form, Input, Space, Tag, Typography } from 'antd';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { CodeOrnamentBackdrop } from '@/components/brand/CodeOrnament';
import FspLogo from '@/components/brand/FspLogo';
import { errMsg } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { theme } from '@/lib/theme';

const { Title, Paragraph } = Typography;
const c = theme.colors;

const DEMO = [
  { label: 'Кандидат · test@candidate.com', email: 'test@candidate.com' },
  { label: 'Работодатель · test@employer.com', email: 'test@employer.com' },
];

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();

  const onFinish = async (values: { email: string; password: string }) => {
    setLoading(true);
    try {
      const user = await login(values.email, values.password);
      message.success(`Добро пожаловать, ${user.email}!`);
      router.replace(user.role === 'EMPLOYER' ? '/employer' : '/candidate');
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        position: 'relative',
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
        background: c.bg,
        overflow: 'hidden',
      }}
    >
      <CodeOrnamentBackdrop opacity={0.08} />
      <Card
        style={{
          position: 'relative',
          width: '100%',
          maxWidth: 440,
          background: c.surface,
          borderColor: c.border,
        }}
      >
        <Space direction="vertical" size="small" style={{ width: '100%', marginBottom: 16 }}>
          <FspLogo height={32} />
          <Title level={3} className="fsp-display" style={{ margin: 0, fontWeight: 400 }}>
            Вход в платформу
          </Title>
          <Paragraph style={{ margin: 0, color: c.textMuted }}>
            Тестирование, matching и приглашения — в одном аккаунте.
          </Paragraph>
        </Space>

        <Divider plain style={{ borderColor: c.border }}>
          <span className="fsp-label">Демо-аккаунты</span>
        </Divider>
        <Space direction="vertical" style={{ width: '100%' }}>
          {DEMO.map((d) => (
            <Button
              key={d.email}
              block
              onClick={() => form.setFieldsValue({ email: d.email, password: 'Test123!' })}
            >
              {d.label} · пароль Test123!
            </Button>
          ))}
        </Space>
        <Divider plain style={{ borderColor: c.border }} />

        <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ email: '', password: '' }}>
          <Form.Item label="Email" name="email" rules={[{ required: true, type: 'email', message: 'Укажите корректный email' }]}>
            <Input placeholder="you@example.com" autoComplete="username" />
          </Form.Item>
          <Form.Item label="Пароль" name="password" rules={[{ required: true, message: 'Введите пароль' }]}>
            <Input.Password placeholder="••••••••" autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={loading}>
            Войти
          </Button>
        </Form>

        <Paragraph style={{ marginTop: 16, marginBottom: 0, textAlign: 'center' }}>
          Нет аккаунта? <Link href="/register">Зарегистрироваться</Link>
        </Paragraph>
        <Paragraph style={{ textAlign: 'center', marginBottom: 0 }}>
          <Link href="/">На главную</Link> · <Tag style={{ marginLeft: 4, background: c.bg, color: c.textMuted }}>API /api/docs</Tag>
        </Paragraph>
      </Card>
    </div>
  );
}
