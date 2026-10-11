'use client';

import { ArrowRightOutlined, RocketOutlined, SafetyCertificateOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { Button, Card, Col, Row, Space, Tag, Typography } from 'antd';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';
import CodeOrnament, { CodeOrnamentBackdrop } from '@/components/brand/CodeOrnament';
import FspLogo from '@/components/brand/FspLogo';
import { useAuth } from '@/lib/auth';
import { theme } from '@/lib/theme';

const { Title, Paragraph } = Typography;
const c = theme.colors;

const FEATURES = [
  {
    icon: <ThunderboltOutlined />,
    color: c.primary,
    title: 'Matching',
    text: 'Каскадная семантика (384-dim) + категория, стек, грейд и достижения — прозрачное объяснение каждого совпадения.',
  },
  {
    icon: <SafetyCertificateOutlined />,
    color: c.turquoise,
    title: 'Тестирование',
    text: '20 вопросов, выбираемых по Fisher Information, оценка theta, пороги грейдов, анти-чит и cooldown 90 дней.',
  },
  {
    icon: <RocketOutlined />,
    color: c.violet,
    title: 'ФСП-интеграция',
    text: 'Достижения и активность подтверждаются данными Федерации спортивного программирования (mock-интеграция в MVP).',
  },
];

export default function HomePage() {
  const { user, ready } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (ready && user) router.replace(user.role === 'EMPLOYER' ? '/employer' : '/candidate');
  }, [ready, user, router]);

  return (
    <div style={{ minHeight: '100vh', background: c.bg, display: 'flex', flexDirection: 'column' }}>
      {/* Герой */}
      <div style={{ position: 'relative', overflow: 'hidden', borderBottom: `1px solid ${c.borderLight}` }}>
        <CodeOrnamentBackdrop opacity={0.1} />
        <div
          style={{
            position: 'relative',
            maxWidth: 960,
            margin: '0 auto',
            width: '100%',
            padding: '64px 24px 48px',
          }}
        >
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            <FspLogo height={44} />
            <Tag
              style={{
                margin: 0,
                background: 'transparent',
                borderColor: c.border,
                color: c.textMuted,
                fontFamily: theme.fonts.display,
                letterSpacing: '0.08em',
              }}
            >
              Federation of Sports Programming · Digital Platform
            </Tag>
            <Title
              level={1}
              className="fsp-display"
              style={{ margin: 0, fontSize: 40, fontWeight: 400, color: c.text }}
            >
              Поиск работы и сотрудников без «воды»
            </Title>
            <Paragraph style={{ fontSize: 16, color: c.textSecondary, maxWidth: 760, marginBottom: 0 }}>
              Платформа-агрегатор ИТ-вакансий: адаптивное компьютерное тестирование (CAT + 3PL IRT),
              semantic-matching кандидатов и вакансий, карьерный трек, достижения и приглашения от работодателей.
            </Paragraph>

            <Space size="middle" wrap>
              <Link href="/login">
                <Button
                  type="primary"
                  size="large"
                  icon={<ArrowRightOutlined />}
                  style={{ fontFamily: theme.fonts.mono, fontWeight: 600 }}
                >
                  Войти в платформу
                </Button>
              </Link>
              <Link href="/register">
                <Button size="large" style={{ fontFamily: theme.fonts.mono }}>
                  Создать аккаунт
                </Button>
              </Link>
            </Space>

            <Paragraph style={{ color: c.textMuted, marginBottom: 0 }}>
              Демо-доступы: <Tag style={{ background: c.surface, color: c.textSecondary }}>test@candidate.com</Tag>{' '}
              <Tag style={{ background: c.surface, color: c.textSecondary }}>test@employer.com</Tag> пароль{' '}
              <Tag style={{ background: c.surface, color: c.textSecondary }}>Test123!</Tag>
            </Paragraph>
          </Space>
        </div>
      </div>

      <CodeOrnament height={32} opacity={0.9} />

      {/* Возможности */}
      <div style={{ maxWidth: 960, margin: '0 auto', width: '100%', padding: '48px 24px', flex: 1 }}>
        <Row gutter={[16, 16]}>
          {FEATURES.map((f) => (
            <Col xs={24} md={8} key={f.title}>
              <Card
                size="small"
                style={{ background: c.surface, borderColor: c.border, height: '100%' }}
                title={
                  <Space>
                    <span style={{ color: f.color }}>{f.icon}</span>
                    <span className="fsp-label" style={{ color: c.text }}>
                      {f.title}
                    </span>
                  </Space>
                }
              >
                <span style={{ color: c.textSecondary }}>{f.text}</span>
              </Card>
            </Col>
          ))}
        </Row>
      </div>

      {/* Футер */}
      <div style={{ borderTop: `1px solid ${c.borderLight}`, padding: '16px 24px', textAlign: 'center' }}>
        <span className="muted">
          © Федерация спортивного программирования России · Платформа ИТ-вакансий
        </span>
      </div>
    </div>
  );
}
