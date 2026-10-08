'use client';

import { ArrowRightOutlined, RocketOutlined, SafetyCertificateOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { Button, Card, Col, Row, Space, Tag, Typography } from 'antd';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';
import { useAuth } from '@/lib/auth';

const { Title, Paragraph } = Typography;

export default function HomePage() {
  const { user, ready } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (ready && user) router.replace(user.role === 'EMPLOYER' ? '/employer' : '/candidate');
  }, [ready, user, router]);

  return (
    <div style={{ minHeight: '100vh', background: 'linear-gradient(160deg,#001529 0%,#003a70 60%,#1677ff 100%)', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
      <Card style={{ maxWidth: 960, width: '100%' }} styles={{ body: { padding: 32 } }}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Tag color="blue" style={{ margin: 0 }}>
            Federation of Trade Unions · Digital Platform
          </Tag>
          <Title level={2} style={{ margin: 0 }}>
            Поиск работы и сотрудников без «воды»
          </Title>
          <Paragraph style={{ fontSize: 16, color: '#595959' }}>
            Платформа-агрегатор ИТ-вакансий: адаптивное компьютерное тестирование (CAT + 3PL IRT),
            semantic-matching кандидатов и вакансий, карьерный трек, достижения и приглашения от работодателей.
          </Paragraph>

          <Row gutter={[16, 16]}>
            <Col xs={24} md={8}>
              <Card size="small" title={<Space><ThunderboltOutlined /> Matching</Space>}>
                Каскадная семантика (384-dim) + категория, стек, грейд и достижения — прозрачное объяснение каждого совпадения.
              </Card>
            </Col>
            <Col xs={24} md={8}>
              <Card size="small" title={<Space><SafetyCertificateOutlined /> Тестирование</Space>}>
                20 вопросов, выбираемых по Fisher Information, оценка theta, пороги грейдов, анти-чит и cooldown 90 дней.
              </Card>
            </Col>
            <Col xs={24} md={8}>
              <Card size="small" title={<Space><RocketOutlined /> ФСП-интеграция</Space>}>
                Достижения и активность подтверждаются данными Федерации профсоюзов (mock-интеграция в MVP).
              </Card>
            </Col>
          </Row>

          <Space size="middle" wrap>
            <Link href="/login">
              <Button type="primary" size="large" icon={<ArrowRightOutlined />}>
                Войти в платформу
              </Button>
            </Link>
            <Link href="/register">
              <Button size="large">Создать аккаунт</Button>
            </Link>
          </Space>

          <Paragraph style={{ color: '#8c8c8c', marginBottom: 0 }}>
            Демо-доступы: <Tag>test@candidate.com</Tag> <Tag>test@employer.com</Tag> пароль <Tag>Test123!</Tag>
          </Paragraph>
        </Space>
      </Card>
    </div>
  );
}
