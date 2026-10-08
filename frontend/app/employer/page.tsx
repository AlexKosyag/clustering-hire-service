'use client';

import {
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Form,
  Input,
  List,
  Modal,
  Progress,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Tag,
  Typography,
} from 'antd';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { GRADES, SPECIALIZATIONS, dateRu, gradeColor, gradeLabel, salaryRange, specLabel } from '@/lib/meta';

const { Paragraph, Text } = Typography;

const INVITE_COLOR: Record<string, string> = {
  SENT: 'blue',
  VIEWED: 'gold',
  ACCEPTED: 'green',
  REJECTED: 'red',
  WITHDRAWN: 'default',
};

export default function EmployerDashboardPage() {
  const { message } = App.useApp();
  const [dashboard, setDashboard] = useState<any>(null);
  const [profile, setProfile] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [editOpen, setEditOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [needLoading, setNeedLoading] = useState(false);
  const [form] = Form.useForm();
  const [needForm] = Form.useForm();

  const load = async () => {
    try {
      const [d, p] = await Promise.all([
        api.get('/employers/dashboard'),
        api.get('/employers/profile'),
      ]);
      setDashboard(d.data);
      setProfile(p.data);
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const openEdit = () => {
    form.setFieldsValue({
      companyName: profile?.companyName ?? '',
      companyDescription: profile?.companyDescription ?? '',
      industry: profile?.industry ?? '',
      website: profile?.website ?? '',
      contactPerson: profile?.contactPerson ?? '',
      contactEmail: profile?.contactEmail ?? '',
      contactPhone: profile?.contactPhone ?? '',
    });
    setEditOpen(true);
  };

  const saveProfile = async (values: any) => {
    setSaving(true);
    try {
      const { data } = await api.put('/employers/profile', values);
      setProfile(data);
      setEditOpen(false);
      message.success('Профиль компании обновлён');
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSaving(false);
    }
  };

  const createNeed = async (values: any) => {
    setNeedLoading(true);
    try {
      await api.post('/employers/need', values);
      message.success('Потребность сохранена — используется в подборе и рекомендациях');
      needForm.resetFields();
      await load();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setNeedLoading(false);
    }
  };

  const toggleVacancy = async (id: string, isActive: boolean) => {
    try {
      await api.put(`/vacancies/${id}`, { isActive });
      await load();
      message.success(isActive ? 'Вакансия опубликована' : 'Вакансия скрыта');
    } catch (error) {
      message.error(errMsg(error));
    }
  };

  if (loading) {
    return (
      <AppShell requireRole="EMPLOYER">
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
        </div>
      </AppShell>
    );
  }

  const stats = dashboard?.stats || {};
  const byStatus = stats.invitationsByStatus || {};

  return (
    <AppShell requireRole="EMPLOYER">
      <h1 className="page-title">Рабочий стол работодателя</h1>
      <p className="page-sub">{profile?.companyName || 'Компания'} · вакансии, отклики и подбор кандидатов.</p>

      <div className="stat-grid" style={{ marginBottom: 16 }}>
        <Card size="small">
          <Statistic title="Приглашения" value={stats.invitations ?? 0} />
          <div style={{ marginTop: 8 }}>
            {Object.entries(byStatus).map(([key, value]) => (
              <Tag key={key} color={INVITE_COLOR[key]}>
                {key}: {String(value)}
              </Tag>
            ))}
          </div>
        </Card>
        <Card size="small">
          <Statistic title="Активные вакансии" value={stats.vacancies ?? 0} />
        </Card>
        <Card size="small">
          <Statistic title="Отклики" value={stats.applications ?? 0} />
        </Card>
        <Card size="small">
          <Statistic title="Кандидатов в подборе" value={stats.candidatesMatched ?? 0} />
          <Link href="/employer/search">
            <Button size="small" type="link" style={{ padding: 0 }}>
              Открыть поиск →
            </Button>
          </Link>
        </Card>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card
            title="Последние приглашения"
            extra={
              <Link href="/employer/search">
                <Button size="small">Пригласить кандидата</Button>
              </Link>
            }
          >
            <List
              size="small"
              dataSource={dashboard?.recentInvitations || []}
              locale={{ emptyText: 'Приглашений пока нет' }}
              renderItem={(item: any) => (
                <List.Item>
                  <List.Item.Meta
                    title={
                      <Space wrap>
                        <b>{item.candidate?.displayName || 'Кандидат'}</b>
                        <Tag color="blue">{specLabel(item.candidate?.specialization)}</Tag>
                        <Tag color={gradeColor(item.candidate?.verifiedGrade)}>
                          {gradeLabel(item.candidate?.verifiedGrade)}
                        </Tag>
                        <Tag color={INVITE_COLOR[item.status]}>{item.status}</Tag>
                      </Space>
                    }
                    description={`${dateRu(item.createdAt)} · стек: ${(item.candidate?.stack || []).join(', ') || '—'}`}
                  />
                </List.Item>
              )}
            />
          </Card>

          <Card title="Мои вакансии" style={{ marginTop: 16 }}>
            <List
              size="small"
              dataSource={dashboard?.vacancies || []}
              locale={{ emptyText: 'Вакансий нет' }}
              renderItem={(vacancy: any) => (
                <List.Item
                  actions={[
                    <Switch
                      key="s"
                      size="small"
                      checked={vacancy.isActive}
                      checkedChildren="активна"
                      unCheckedChildren="скрыта"
                      onChange={(checked) => toggleVacancy(vacancy.id, checked)}
                    />,
                    <Link key="a" href={`/employer/vacancies?open=${vacancy.id}`}>
                      <Button size="small">Отклики</Button>
                    </Link>,
                  ]}
                >
                  <List.Item.Meta
                    title={
                      <Space wrap>
                        <b>{vacancy.title}</b>
                        <Tag>{specLabel(vacancy.specialization)}</Tag>
                        <Tag color={gradeColor(vacancy.grade)}>{gradeLabel(vacancy.grade)}</Tag>
                      </Space>
                    }
                    description={salaryRange(vacancy.salaryFrom, vacancy.salaryTo)}
                  />
                </List.Item>
              )}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title="Потребность для подбора">
            <Paragraph type="secondary">
              Опишите, кого ищете — текст участвует в семантическом matching и рекомендациях.
            </Paragraph>
            <Form form={needForm} layout="vertical" onFinish={createNeed}>
              <Form.Item
                label="Описание потребности"
                name="description"
                rules={[
                  { required: true, min: 10, message: 'Минимум 10 символов' },
                  { max: 4000 },
                ]}
              >
                <Input.TextArea rows={3} placeholder="Ищем backend-разработчика на Node.js с опытом от 3 лет…" />
              </Form.Item>
              <Row gutter={12}>
                <Col span={8}>
                  <Form.Item label="Специализация" name="specialization">
                    <Select allowClear options={SPECIALIZATIONS} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label="Грейд" name="grade">
                    <Select allowClear options={GRADES} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label="Стек" name="stack">
                    <Select mode="tags" placeholder="Node.js, Redis" />
                  </Form.Item>
                </Col>
              </Row>
              <Button type="primary" htmlType="submit" loading={needLoading}>
                Сохранить потребность
              </Button>
            </Form>
          </Card>

          <Card
            title="Профиль компании"
            style={{ marginTop: 16 }}
            extra={
              <Button size="small" onClick={openEdit}>
                Редактировать
              </Button>
            }
          >
            <Descriptions column={1} size="small">
              <Descriptions.Item label="Компания">{profile?.companyName || '—'}</Descriptions.Item>
              <Descriptions.Item label="Отрасль">{profile?.industry || '—'}</Descriptions.Item>
              <Descriptions.Item label="Сайт">{profile?.website || '—'}</Descriptions.Item>
              <Descriptions.Item label="Контактное лицо">{profile?.contactPerson || '—'}</Descriptions.Item>
              <Descriptions.Item label="Email">{profile?.contactEmail || profile?.email || '—'}</Descriptions.Item>
              <Descriptions.Item label="Телефон">{profile?.contactPhone || '—'}</Descriptions.Item>
            </Descriptions>
            <Divider style={{ margin: '12px 0' }} />
            <Paragraph style={{ marginBottom: 0 }}>{profile?.companyDescription || 'Описание не заполнено.'}</Paragraph>
          </Card>

          {(dashboard?.topCandidates || []).length > 0 && (
            <Card title="Лучшие кандидаты под вашу потребность" style={{ marginTop: 16 }}>
              <List
                size="small"
                dataSource={dashboard.topCandidates.slice(0, 5)}
                renderItem={(candidate: any) => (
                  <List.Item>
                    <div style={{ width: '100%' }}>
                      <Space wrap>
                        <b>{candidate.displayName}</b>
                        <Tag color="blue">{specLabel(candidate.specialization)}</Tag>
                        <Tag color={gradeColor(candidate.verifiedGrade)}>{gradeLabel(candidate.verifiedGrade)}</Tag>
                      </Space>
                      {candidate.matchScore != null && (
                        <Progress percent={candidate.matchScore} size="small" status="active" />
                      )}
                      <Text type="secondary">{(candidate.stack || []).join(', ')}</Text>
                    </div>
                  </List.Item>
                )}
              />
            </Card>
          )}
        </Col>
      </Row>

      <Modal
        title="Профиль компании"
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        footer={null}
        width={680}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={saveProfile}>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label="Название компании" name="companyName" rules={[{ max: 120 }]}>
                <Input />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label="Отрасль" name="industry" rules={[{ max: 80 }]}>
                <Input />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="Сайт" name="website" rules={[{ max: 200 }]}>
            <Input placeholder="https://company.ru" />
          </Form.Item>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="Контактное лицо" name="contactPerson">
                <Input />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Контактный email" name="contactEmail">
                <Input />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Телефон" name="contactPhone">
                <Input />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="О компании" name="companyDescription" rules={[{ max: 4000 }]}>
            <Input.TextArea rows={4} />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={saving}>
            Сохранить
          </Button>{' '}
          <Button onClick={() => setEditOpen(false)}>Отмена</Button>
        </Form>
      </Modal>
    </AppShell>
  );
}
