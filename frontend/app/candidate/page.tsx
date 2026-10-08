'use client';

import { EditOutlined, SyncOutlined } from '@ant-design/icons';
import {
  App,
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Progress,
  Row,
  Select,
  Spin,
  Statistic,
  Tag,
  Typography,
} from 'antd';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { errMsg, api } from '@/lib/api';
import { GRADES, SPECIALIZATIONS, dateRu, gradeColor, gradeLabel, specLabel } from '@/lib/meta';

const { Text, Paragraph } = Typography;

const CATEGORY_COLORS: Record<string, string> = {
  competition: 'gold',
  certification: 'blue',
  event: 'purple',
  education: 'green',
  contribution: 'geekblue',
  community: 'magenta',
};

const KNOWN_TECH = [
  'Node.js', 'TypeScript', 'JavaScript', 'React', 'Next.js', 'Vue', 'Angular', 'PostgreSQL', 'MySQL',
  'MongoDB', 'Redis', 'Docker', 'Kubernetes', 'Terraform', 'Go', 'Python', 'Java', 'C#', 'Kotlin',
  'Swift', 'Flutter', 'Android', 'iOS', 'GraphQL', 'REST API', 'gRPC', 'Kafka', 'RabbitMQ',
  'CI/CD', 'Grafana', 'Prometheus', 'Ansible', 'AWS', 'GCP', 'Figma', 'PyTorch', 'TensorFlow',
];

export default function CandidateProfilePage() {
  const { message } = App.useApp();
  const [profile, setProfile] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [form] = Form.useForm();

  const load = async () => {
    try {
      const { data } = await api.get('/candidates/profile');
      setProfile(data);
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
      firstName: profile.firstName ?? '',
      lastName: profile.lastName ?? '',
      middleName: profile.middleName ?? '',
      city: profile.city ?? '',
      phone: profile.phone ?? '',
      specialization: profile.specialization,
      declaredGrade: profile.declaredGrade,
      experienceYears: profile.experienceYears,
      stack: profile.stack ?? [],
      roles: profile.roles ?? [],
      softSkills: profile.softSkills ?? [],
      bio: profile.bio ?? '',
    });
    setEditing(true);
  };

  const onFinish = async (values: any) => {
    setSaving(true);
    try {
      const { data } = await api.put('/candidates/profile', values);
      setProfile(data);
      setEditing(false);
      message.success('Профиль сохранён — вектор обновлён в фоне');
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSaving(false);
    }
  };

  const syncFsp = async () => {
    setSyncing(true);
    try {
      const { data } = await api.post('/fsp/connect', { fspId: profile.fspId || profile.email });
      message.success(`ФСП синхронизирован: подтверждено достижений — ${data.achievementsFound ?? 0}`);
      await load();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSyncing(false);
    }
  };

  if (loading) {
    return (
      <AppShell requireRole="CANDIDATE">
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
        </div>
      </AppShell>
    );
  }

  if (!profile) return null;

  const cooldown = profile.testCooldownUntil ? new Date(profile.testCooldownUntil) : null;
  const cooldownActive = !!cooldown && cooldown.getTime() > Date.now();

  return (
    <AppShell requireRole="CANDIDATE">
      <h1 className="page-title">Профиль кандидата</h1>
      <p className="page-sub">Данные профиля участвуют в matching-поиске и определяют рекомендации вакансий.</p>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card
            title={profile.displayName || 'Мой профиль'}
            extra={
              <Button type="primary" icon={<EditOutlined />} onClick={openEdit}>
                Редактировать
              </Button>
            }
          >
            <Descriptions column={{ xs: 1, sm: 2 }} size="small">
              <Descriptions.Item label="Специализация">
                <Tag color="blue">{specLabel(profile.specialization)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Грейд">
                <Tag color={gradeColor(profile.declaredGrade)}>заявлен: {gradeLabel(profile.declaredGrade)}</Tag>
                <Tag color={gradeColor(profile.verifiedGrade)}>подтверждён: {gradeLabel(profile.verifiedGrade)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Город">{profile.city || '—'}</Descriptions.Item>
              <Descriptions.Item label="Опыт">
                {profile.experienceYears != null ? `${profile.experienceYears} лет` : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Телефон">{profile.phone || '—'}</Descriptions.Item>
              <Descriptions.Item label="Email">{profile.email}</Descriptions.Item>
            </Descriptions>

            <Divider orientation="left" plain>
              Стек
            </Divider>
            <div>
              {(profile.stack || []).map((s: string) => (
                <Tag color="geekblue" key={s}>
                  {s}
                </Tag>
              ))}
            </div>

            {!!profile.roles?.length && (
              <>
                <Divider orientation="left" plain>
                  Роли
                </Divider>
                <div>
                  {profile.roles.map((r: string) => (
                    <Tag key={r}>{r}</Tag>
                  ))}
                </div>
              </>
            )}

            {!!profile.softSkills?.length && (
              <>
                <Divider orientation="left" plain>
                  Soft skills
                </Divider>
                <div>
                  {profile.softSkills.map((r: string) => (
                    <Tag color="cyan" key={r}>
                      {r}
                    </Tag>
                  ))}
                </div>
              </>
            )}

            <Divider orientation="left" plain>
              О себе
            </Divider>
            <Paragraph>{profile.bio || 'Заполните описание — оно используется как текст для семантического matching.'}</Paragraph>
          </Card>

          <Card title="Достижения" style={{ marginTop: 16 }}>
            <div className="stat-grid" style={{ marginBottom: 12 }}>
              <Statistic title="Всего очков" value={profile.achievementPoints ?? 0} suffix="очк." />
              <Statistic title="Достижений" value={(profile.achievements || []).length} />
              <Statistic title="Последняя активность" value={dateRu(profile.lastActive)} />
            </div>
            <List
              size="small"
              dataSource={profile.achievements || []}
              locale={{ emptyText: 'Достижения не подтверждены — синхронизируйтесь с ФСП' }}
              renderItem={(item: any) => (
                <List.Item>
                  <List.Item.Meta
                    title={
                      <span>
                        {item.name} <Tag color={CATEGORY_COLORS[item.category] || 'default'}>{item.category}</Tag>
                      </span>
                    }
                    description={
                      <>
                        {item.year ? `Год: ${item.year}` : ''} {item.place ? `· Место: ${item.place}` : ''}{' '}
                        <b>· +{item.points} очк.</b>
                      </>
                    }
                  />
                </List.Item>
              )}
            />
          </Card>
        </Col>

        <Col xs={24} lg={8}>
          <Card title="Тестирование">
            <div className="stat-grid">
              <Statistic title="Статус" value={profile.testStatus || 'NOT_STARTED'} />
              <Statistic title="Балл" value={profile.testScore ?? '—'} />
            </div>
            <Paragraph type="secondary" style={{ marginTop: 8 }}>
              Последний тест: {dateRu(profile.lastTestDate)}
            </Paragraph>
            {cooldownActive && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message={`Пересдача ${cooldown!.toLocaleDateString('ru-RU')}`}
              />
            )}
            <Link href="/candidate/test">
              <Button type="primary" block disabled={cooldownActive}>
                {cooldownActive ? 'Тест уже пройден' : 'Пройти / продолжить тест'}
              </Button>
            </Link>
          </Card>

          <Card title="Интеграция с ФСП" style={{ marginTop: 16 }}>
            <Paragraph type="secondary">
              Достижения и активность подтверждаются данными Федерации профсоюзов (mock-интеграция в MVP).
            </Paragraph>
            <Statistic title="ФСП ID" value={profile.fspId || 'не подключён'} valueStyle={{ fontSize: 16 }} />
            <Button
              style={{ marginTop: 12 }}
              block
              icon={<SyncOutlined spin={syncing} />}
              loading={syncing}
              onClick={syncFsp}
            >
              Синхронизировать с ФСП
            </Button>
          </Card>

          <Card title="Видимость" style={{ marginTop: 16 }}>
            <Paragraph type="secondary" style={{ marginBottom: 0 }}>
              Контакты скрыты до тех пор, пока вы не примете приглашение работодателя — принцип
              «employer-first».
            </Paragraph>
            <div style={{ marginTop: 8 }}>
              <Tag color="green">Email виден работодателю только после ACCEPT</Tag>
            </div>
          </Card>
        </Col>
      </Row>

      <Modal
        title="Редактирование профиля"
        open={editing}
        onCancel={() => setEditing(false)}
        footer={null}
        width={760}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="Имя" name="firstName">
                <Input />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Фамилия" name="lastName">
                <Input />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Отчество" name="middleName">
                <Input />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="Специализация" name="specialization">
                <Select allowClear options={SPECIALIZATIONS} placeholder="—" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Заявленный грейд" name="declaredGrade">
                <Select allowClear options={GRADES} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Опыт, лет" name="experienceYears">
                <InputNumber min={0} max={50} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="Город" name="city">
                <Input />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Телефон" name="phone">
                <Input />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            label="Стек технологий"
            name="stack"
            extra="Enter — добавить тег; регистр не важен (Node.js = Node JS = nodejs)"
          >
            <Select mode="tags" options={KNOWN_TECH.map((t) => ({ value: t }))} placeholder="Node.js, PostgreSQL…" />
          </Form.Item>
          <Form.Item label="Роли" name="roles">
            <Select mode="tags" placeholder="Backend-разработчик" />
          </Form.Item>
          <Form.Item label="Soft skills" name="softSkills">
            <Select mode="tags" placeholder="коммуникация, наставничество" />
          </Form.Item>
          <Form.Item label="О себе" name="bio">
            <Input.TextArea rows={4} maxLength={1000} showCount />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={saving}>
            Сохранить
          </Button>{' '}
          <Button onClick={() => setEditing(false)}>Отмена</Button>
        </Form>
      </Modal>
    </AppShell>
  );
}
