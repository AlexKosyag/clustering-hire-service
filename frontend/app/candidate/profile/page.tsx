'use client';

import {
  Alert,
  App,
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
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { EditOutlined, SyncOutlined, EyeOutlined } from '@ant-design/icons';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { errMsg, api } from '@/lib/api';
import { GRADES, SPECIALIZATIONS, dateRu, gradeColor, gradeLabel, specLabel } from '@/lib/meta';
import { theme } from '@/lib/theme';
import GradeBadge from '@/components/ui/GradeBadge';
import TechTag from '@/components/ui/TechTag';

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
  const [previewMode, setPreviewMode] = useState(false);
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
      showFullName: profile.showFullName ?? true,
      showEmail: profile.showEmail ?? false,
      showPhone: profile.showPhone ?? false,
    });
    setEditing(true);
  };

  const onFinish = async (values: any) => {
    setSaving(true);
    try {
      const { data } = await api.put('/candidates/profile', values);
      setProfile(data);
      setEditing(false);
      message.success('Профиль сохранён');
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSaving(false);
    }
  };

  const syncFsp = async () => {
    setSyncing(true);
    try {
      const { data } = await api.post('/fsp/connect', {
        fspId: profile.fspId || profile.email,
      });
      message.success(`ФСП синхронизирован: ${data.achievementsFound ?? 0} достижений`);
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
      <div style={{ maxWidth: 1000, margin: '0 auto' }}>
        {/* Header */}
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: 24,
          }}
        >
          <div>
            <h1 style={{ fontSize: 28, fontWeight: 700, color: theme.colors.text, margin: 0 }}>
              Профиль
            </h1>
            <p style={{ color: theme.colors.textSecondary, marginTop: 8 }}>
              Данные профиля участвуют в matching-поиске
            </p>
          </div>
          <Space>
            <Button
              icon={<EyeOutlined />}
              onClick={() => setPreviewMode(true)}
            >
              Превью
            </Button>
            <Button
              type="primary"
              icon={<EditOutlined />}
              onClick={openEdit}
            >
              Редактировать
            </Button>
          </Space>
        </div>

        <Row gutter={[16, 16]}>
          {/* Main Profile */}
          <Col xs={24} lg={16}>
            <Card
              style={{ borderRadius: theme.radius.lg, marginBottom: 16 }}
              title={
                <Space>
                  <span>👤</span>
                  <span>Основная информация</span>
                </Space>
              }
            >
              <Descriptions column={{ xs: 1, sm: 2 }} size="small">
                <Descriptions.Item label="Имя">
                  {profile.firstName && profile.lastName
                    ? `${profile.firstName} ${profile.lastName}`
                    : 'Не заполнено'}
                </Descriptions.Item>
                <Descriptions.Item label="Email">{profile.email}</Descriptions.Item>
                <Descriptions.Item label="Город">{profile.city || '—'}</Descriptions.Item>
                <Descriptions.Item label="Телефон">{profile.phone || '—'}</Descriptions.Item>
                <Descriptions.Item label="Специализация">
                  {profile.specialization ? (
                    <Tag color="blue">{specLabel(profile.specialization)}</Tag>
                  ) : (
                    '—'
                  )}
                </Descriptions.Item>
                <Descriptions.Item label="Опыт">
                  {profile.experienceYears != null
                    ? `${profile.experienceYears} лет`
                    : '—'}
                </Descriptions.Item>
              </Descriptions>

              <Divider orientation="left" plain>
                Грейд
              </Divider>
              <Space size="middle">
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    Заявленный:
                  </Text>{' '}
                  <GradeBadge grade={profile.declaredGrade} />
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    Подтверждённый:
                  </Text>{' '}
                  <GradeBadge grade={profile.verifiedGrade} verified />
                </div>
              </Space>

              <Divider orientation="left" plain>
                Стек технологий
              </Divider>
              <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                {profile.stack?.length > 0 ? (
                  profile.stack.map((tech: string) => (
                    <TechTag key={tech} tech={tech} />
                  ))
                ) : (
                  <Text type="secondary">Стек не заполнен</Text>
                )}
              </div>

              {profile.roles?.length > 0 && (
                <>
                  <Divider orientation="left" plain>
                    Роли
                  </Divider>
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                    {profile.roles.map((role: string) => (
                      <Tag key={role}>{role}</Tag>
                    ))}
                  </div>
                </>
              )}

              {profile.softSkills?.length > 0 && (
                <>
                  <Divider orientation="left" plain>
                    Soft skills
                  </Divider>
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                    {profile.softSkills.map((skill: string) => (
                      <Tag color="cyan" key={skill}>
                        {skill}
                      </Tag>
                    ))}
                  </div>
                </>
              )}

              <Divider orientation="left" plain>
                О себе
              </Divider>
              <Paragraph>
                {profile.bio || 'Заполните описание — оно используется для семантического matching'}
              </Paragraph>
            </Card>

            {/* Achievements */}
            <Card
              style={{ borderRadius: theme.radius.lg }}
              title={
                <Space>
                  <span>🏆</span>
                  <span>Достижения ФСП</span>
                </Space>
              }
              extra={
                <Statistic
                  title="Очков"
                  value={profile.achievementPoints ?? 0}
                  valueStyle={{ fontSize: 16 }}
                />
              }
            >
              <List
                size="small"
                dataSource={profile.achievements || []}
                locale={{
                  emptyText: (
                    <div style={{ textAlign: 'center', padding: 24 }}>
                      <div style={{ fontSize: 32, marginBottom: 8 }}>🎯</div>
                      <Text type="secondary">
                        Достижения не подтверждены. Подключите ФСП ID для синхронизации.
                      </Text>
                    </div>
                  ),
                }}
                renderItem={(item: any) => (
                  <List.Item>
                    <List.Item.Meta
                      title={
                        <Space>
                          <span>{item.name}</span>
                          <Tag color={CATEGORY_COLORS[item.category] || 'default'}>
                            {item.category}
                          </Tag>
                        </Space>
                      }
                      description={
                        <Space size="middle">
                          {item.year && <span>Год: {item.year}</span>}
                          {item.place && <span>Место: {item.place}</span>}
                          <b>+{item.points} очк.</b>
                        </Space>
                      }
                    />
                  </List.Item>
                )}
              />
            </Card>
          </Col>

          {/* Sidebar */}
          <Col xs={24} lg={8}>
            {/* Test Status */}
            <Card
              style={{ borderRadius: theme.radius.lg, marginBottom: 16 }}
              title={
                <Space>
                  <span>📝</span>
                  <span>Тестирование</span>
                </Space>
              }
            >
              <div style={{ textAlign: 'center', marginBottom: 16 }}>
                <div
                  style={{
                    fontSize: 48,
                    marginBottom: 8,
                  }}
                >
                  {profile.testStatus === 'PASSED' || profile.testStatus === 'UPGRADED'
                    ? '✅'
                    : profile.testStatus === 'FAILED'
                    ? '❌'
                    : '📝'}
                </div>
                <Tag
                  color={
                    profile.testStatus === 'PASSED' || profile.testStatus === 'UPGRADED'
                      ? 'green'
                      : profile.testStatus === 'FAILED'
                      ? 'red'
                      : 'default'
                  }
                >
                  {profile.testStatus || 'NOT_STARTED'}
                </Tag>
              </div>

              {profile.testScore != null && (
                <div style={{ textAlign: 'center', marginBottom: 16 }}>
                  <Statistic title="Балл" value={profile.testScore} suffix="/ 100" />
                </div>
              )}

              {profile.lastTestDate && (
                <div style={{ textAlign: 'center', marginBottom: 16 }}>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    Последний тест: {dateRu(profile.lastTestDate)}
                  </Text>
                </div>
              )}

              {cooldownActive && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message={`Пересдача ${cooldown!.toLocaleDateString('ru-RU')}`}
                />
              )}
            </Card>

            {/* FSP Integration */}
            <Card
              style={{ borderRadius: theme.radius.lg, marginBottom: 16 }}
              title={
                <Space>
                  <span>🔗</span>
                  <span>ФСП интеграция</span>
                </Space>
              }
            >
              {profile.fspId ? (
                <div>
                  <Tag color="green" style={{ marginBottom: 12 }}>
                    ✓ Подключено
                  </Tag>
                  <div style={{ fontSize: 13, color: theme.colors.textSecondary, marginBottom: 12 }}>
                    ID: {profile.fspId}
                  </div>
                </div>
              ) : (
                <Paragraph type="secondary" style={{ fontSize: 13 }}>
                  Подключите ФСП ID для автоматической синхронизации достижений
                </Paragraph>
              )}
              <Button
                block
                icon={<SyncOutlined spin={syncing} />}
                loading={syncing}
                onClick={syncFsp}
              >
                Синхронизировать с ФСП
              </Button>
            </Card>

            {/* Privacy */}
            <Card
              style={{
                borderRadius: theme.radius.lg,
                background: theme.colors.infoLight,
                border: `1px solid ${theme.colors.info}30`,
              }}
              title={
                <Space>
                  <span>🔒</span>
                  <span>Приватность</span>
                </Space>
              }
            >
              <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
                <p style={{ marginTop: 0 }}>
                  Контактные данные видны работодателю только после того, как вы примете приглашение.
                </p>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span>Показывать ФИО:</span>
                    <Tag color={profile.showFullName ? 'green' : 'default'}>
                      {profile.showFullName ? 'Да' : 'Нет'}
                    </Tag>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span>Показывать email:</span>
                    <Tag color={profile.showEmail ? 'green' : 'default'}>
                      {profile.showEmail ? 'Да' : 'Нет'}
                    </Tag>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span>Показывать телефон:</span>
                    <Tag color={profile.showPhone ? 'green' : 'default'}>
                      {profile.showPhone ? 'Да' : 'Нет'}
                    </Tag>
                  </div>
                </div>
              </div>
            </Card>
          </Col>
        </Row>
      </div>

      {/* Edit Modal */}
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
            extra="Enter — добавить тег; регистр не важен"
          >
            <Select
              mode="tags"
              options={KNOWN_TECH.map((t) => ({ value: t }))}
              placeholder="Node.js, PostgreSQL…"
            />
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

          <Divider orientation="left" plain>
            Настройки приватности
          </Divider>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item
                label="Показывать ФИО"
                name="showFullName"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                label="Показывать email"
                name="showEmail"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                label="Показывать телефон"
                name="showPhone"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={() => setEditing(false)}>Отмена</Button>
          </Space>
        </Form>
      </Modal>

      {/* Preview Modal */}
      <Modal
        title="Как видит работодатель"
        open={previewMode}
        onCancel={() => setPreviewMode(false)}
        footer={null}
        width={600}
      >
        <div
          style={{
            padding: 24,
            background: theme.colors.bg,
            borderRadius: theme.radius.md,
          }}
        >
          <div style={{ display: 'flex', gap: 16, marginBottom: 16 }}>
            <div
              style={{
                width: 64,
                height: 64,
                borderRadius: theme.radius.full,
                background: `linear-gradient(135deg, ${theme.colors.primary}, ${theme.colors.primaryDark})`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'white',
                fontSize: 24,
                fontWeight: 700,
              }}
            >
              {(profile.firstName || '?').charAt(0)}
            </div>
            <div>
              <div style={{ fontSize: 18, fontWeight: 600, color: theme.colors.text }}>
                {profile.showFullName && profile.firstName
                  ? `${profile.firstName} ${profile.lastName || ''}`
                  : `Кандидат #${profile.id.slice(0, 4)}`}
              </div>
              <div style={{ color: theme.colors.textSecondary }}>
                {profile.specialization ? specLabel(profile.specialization) : '—'}
              </div>
              <div style={{ marginTop: 8 }}>
                <GradeBadge grade={profile.verifiedGrade || profile.declaredGrade} verified={!!profile.verifiedGrade} />
              </div>
            </div>
          </div>

          {profile.stack?.length > 0 && (
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: theme.colors.textMuted, marginBottom: 8 }}>
                Стек:
              </div>
              <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                {profile.stack.slice(0, 8).map((tech: string) => (
                  <TechTag key={tech} tech={tech} size="sm" />
                ))}
              </div>
            </div>
          )}

          {profile.bio && (
            <div>
              <div style={{ fontSize: 12, color: theme.colors.textMuted, marginBottom: 4 }}>
                О себе:
              </div>
              <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
                {profile.bio.slice(0, 200)}
                {profile.bio.length > 200 && '…'}
              </div>
            </div>
          )}

          <div
            style={{
              marginTop: 16,
              padding: 12,
              background: theme.colors.infoLight,
              borderRadius: theme.radius.sm,
              fontSize: 12,
              color: theme.colors.textSecondary,
            }}
          >
            🔒 Контакты скрыты до принятия приглашения
          </div>
        </div>
      </Modal>
    </AppShell>
  );
}