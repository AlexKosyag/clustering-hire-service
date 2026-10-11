'use client';

import { App, Button, Card, Form, Input, Radio, Typography } from 'antd';
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

export default function RegisterPage() {
  const { register } = useAuth();
  const router = useRouter();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);

  const onFinish = async (values: { email: string; password: string; role: 'CANDIDATE' | 'EMPLOYER' }) => {
    setLoading(true);
    try {
      const user = await register(values.email, values.password, values.role);
      message.success('Аккаунт создан');
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
      <Card style={{ position: 'relative', width: '100%', maxWidth: 480, background: c.surface, borderColor: c.border }}>
        <FspLogo height={32} style={{ marginBottom: 12 }} />
        <Title level={3} className="fsp-display" style={{ marginTop: 0, fontWeight: 400 }}>
          Регистрация
        </Title>
        <Paragraph style={{ color: c.textMuted }}>Пароль: минимум 8 символов, буквы в разных регистрах и цифра.</Paragraph>

        <Form layout="vertical" onFinish={onFinish} initialValues={{ role: 'CANDIDATE' }}>
          <Form.Item label="Я регистрируюсь как" name="role">
            <Radio.Group
              optionType="button"
              buttonStyle="solid"
              options={[
                { label: 'Кандидат', value: 'CANDIDATE' },
                { label: 'Работодатель', value: 'EMPLOYER' },
              ]}
            />
          </Form.Item>
          <Form.Item label="Email" name="email" rules={[{ required: true, type: 'email', message: 'Укажите корректный email' }]}>
            <Input placeholder="you@example.com" />
          </Form.Item>
          <Form.Item
            label="Пароль"
            name="password"
            rules={[
              { required: true, message: 'Введите пароль' },
              { min: 8, message: 'Минимум 8 символов' },
              {
                pattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/,
                message: 'Нужны буквы в разных регистрах и цифра',
              },
            ]}
          >
            <Input.Password placeholder="••••••••" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={loading}>
            Создать аккаунт
          </Button>
        </Form>

        <Paragraph style={{ marginTop: 16, marginBottom: 0, textAlign: 'center' }}>
          Уже есть аккаунт? <Link href="/login">Войти</Link>
        </Paragraph>
      </Card>
    </div>
  );
}
