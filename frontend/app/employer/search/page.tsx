'use client';

import { ClockCircleOutlined, SearchOutlined, UserAddOutlined } from '@ant-design/icons';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Progress,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { GRADES, SPECIALIZATIONS, gradeColor, gradeLabel, specLabel } from '@/lib/meta';

const { Paragraph, Text } = Typography;

interface CandidateRow {
  id: string;
  displayName?: string;
  firstName?: string;
  lastName?: string;
  specialization?: string;
  declaredGrade?: string;
  verifiedGrade?: string;
  stack?: string[];
  city?: string;
  experienceYears?: number | null;
  bio?: string;
  testStatus?: string;
  testScore?: number | null;
  achievementPoints?: number;
  matchScore?: number;
  matchExplanation?: { overall?: number; details?: any[] };
  similarity?: number;
  reason?: string;
}

export default function EmployerSearchPage() {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<'match' | 'semantic'>('match');
  const [rows, setRows] = useState<CandidateRow[]>([]);
  const [total, setTotal] = useState(0);
  const [elapsed, setElapsed] = useState<number | null>(null);
  const [explanation, setExplanation] = useState<any>(null);
  const [inviteOpen, setInviteOpen] = useState<CandidateRow | null>(null);
  const [inviting, setInviting] = useState(false);
  const [inviteForm] = Form.useForm();
  const [searched, setSearched] = useState(false);

  useEffect(() => {
    form.setFieldsValue({
      specialization: 'BACKEND',
      grade: 'MIDDLE',
      stack: ['Node.js', 'PostgreSQL', 'Redis', 'Docker'],
      limit: 10,
    });
    onSearch();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const onSearch = async (values?: any) => {
    const v = values || form.getFieldsValue();
    setLoading(true);
    setSearched(true);
    const started = Date.now();
    try {
      if (mode === 'semantic') {
        const { data } = await api.post('/matching/semantic', {
          query: v.description || v.query || '',
          limit: v.limit || 10,
        });
        setRows(data.candidates || []);
        setTotal((data.candidates || []).length);
      } else {
        const { data } = await api.post('/matching/search', {
          description: v.description,
          specialization: v.specialization,
          grade: v.grade,
          stack: v.stack,
          hasAchievements: v.hasAchievements,
          limit: v.limit || 10,
        });
        setRows(data.candidates || []);
        setTotal(data.total || 0);
      }
      setElapsed(Date.now() - started);
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setLoading(false);
    }
  };

  const sendInvite = async (values: any) => {
    if (!inviteOpen) return;
    setInviting(true);
    try {
      await api.post('/invitations', {
        candidateId: inviteOpen.id,
        message: values.message,
        salaryFrom: values.salaryFrom,
        salaryTo: values.salaryTo,
      });
      message.success('Приглашение отправлено — контакты откроются после ACCEPT');
      setInviteOpen(null);
      inviteForm.resetFields();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setInviting(false);
    }
  };

  const nameOf = (row: CandidateRow) =>
    row.displayName || [row.lastName, row.firstName].filter(Boolean).join(' ') || 'Кандидат';

  const columns = [
    {
      title: 'Кандидат',
      key: 'name',
      render: (_: unknown, row: CandidateRow) => (
        <Space direction="vertical" size={0}>
          <b>{nameOf(row)}</b>
          <Text type="secondary">{row.city || '—'}{row.experienceYears != null ? ` · ${row.experienceYears} лет опыта` : ''}</Text>
        </Space>
      ),
    },
    {
      title: 'Специализация',
      dataIndex: 'specialization',
      render: (v: string) => <Tag color="blue">{specLabel(v)}</Tag>,
    },
    {
      title: 'Грейд',
      key: 'grade',
      render: (_: unknown, row: CandidateRow) => (
        <Space size={4}>
          <Tag color={gradeColor(row.verifiedGrade)}>подтв.: {gradeLabel(row.verifiedGrade)}</Tag>
          <Tag color={gradeColor(row.declaredGrade)}>заявл.: {gradeLabel(row.declaredGrade)}</Tag>
        </Space>
      ),
    },
    {
      title: 'Стек',
      key: 'stack',
      render: (_: unknown, row: CandidateRow) =>
        (row.stack || []).slice(0, 6).map((s) => <Tag key={s}>{s}</Tag>),
    },
    {
      title: 'Тест',
      key: 'test',
      width: 140,
      render: (_: unknown, row: CandidateRow) => (
        <Space direction="vertical" size={0}>
          <Tag color={row.testStatus === 'PASSED' ? 'green' : row.testStatus === 'FAILED' ? 'red' : 'default'}>
            {row.testStatus || 'NOT_TAKEN'}
          </Tag>
          {row.testScore != null && <Text type="secondary">{row.testScore} баллов</Text>}
        </Space>
      ),
    },
    {
      title: 'Совпадение',
      key: 'score',
      width: 210,
      render: (_: unknown, row: CandidateRow) => (
        <Space direction="vertical" size={2} style={{ width: '100%' }}>
          {row.matchScore != null ? (
            <Progress
              percent={row.matchScore}
              size="small"
              status={row.matchScore >= 70 ? 'success' : row.matchScore >= 45 ? 'normal' : 'exception'}
            />
          ) : row.similarity != null ? (
            <Progress percent={Math.round(row.similarity * 100)} size="small" format={(p) => `cos ${p}%`} />
          ) : (
            <Text type="secondary">—</Text>
          )}
          <Space size={4}>
            {row.matchExplanation && (
              <Button size="small" type="link" style={{ padding: 0 }} onClick={() => setExplanation({ row })}>
                Объяснение
              </Button>
            )}
            <Button
              size="small"
              type="primary"
              icon={<UserAddOutlined />}
              onClick={() => {
                setInviteOpen(row);
                inviteForm.setFieldsValue({ message: `Здравствуйте! Приглашаем вас обсудить вакансию.` });
              }}
            >
              Пригласить
            </Button>
          </Space>
        </Space>
      ),
    },
  ];

  const details = explanation?.row?.matchExplanation?.details || [];

  return (
    <AppShell requireRole="EMPLOYER">
      <h1 className="page-title">Поиск кандидатов</h1>
      <p className="page-sub">
        Каскадный scoring: категория 30% · стек 25% · семантика 20% · грейд 15% · достижения 10%.
        Hard-фильтры — специализация и грейд ±1.
      </p>

      <Card style={{ marginBottom: 16 }}>
        <Form form={form} layout="vertical" onFinish={onSearch}>
          <Row gutter={[12, 8]}>
            <Col xs={24} md={9}>
              <Form.Item label="Описание потребности" name="description">
                <Input.TextArea
                  rows={2}
                  placeholder="Ищем backend-разработчика на Node.js: REST API, PostgreSQL, Redis, очереди, высокая нагрузка"
                />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item label="Специализация" name="specialization">
                <Select allowClear options={SPECIALIZATIONS} placeholder="любая" />
              </Form.Item>
            </Col>
            <Col xs={12} md={3}>
              <Form.Item label="Грейд" name="grade">
                <Select allowClear options={GRADES} placeholder="любой" />
              </Form.Item>
            </Col>
            <Col xs={24} md={5}>
              <Form.Item label="Стек" name="stack">
                <Select mode="tags" placeholder="Node.js, Redis" />
              </Form.Item>
            </Col>
            <Col xs={12} md={3}>
              <Form.Item label="Лимит" name="limit">
                <InputNumber min={1} max={50} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Space wrap>
            <Button
              type="primary"
              htmlType="submit"
              loading={loading}
              icon={<SearchOutlined />}
              onClick={() => setMode('match')}
            >
              Подобрать (scoring)
            </Button>
            <Button
              loading={loading}
              onClick={() => {
                setMode('semantic');
                onSearch({ ...form.getFieldsValue(), query: form.getFieldValue('description') });
              }}
            >
              Только семантика (cosine)
            </Button>
            <Form.Item name="hasAchievements" valuePropName="checked" style={{ marginBottom: 0 }}>
              <Switch checkedChildren="с достиж." unCheckedChildren="все" defaultChecked={false} />
            </Form.Item>
          </Space>
        </Form>

        {searched && (
          <div style={{ marginTop: 12 }}>
            <Space size="large" wrap>
              <Statistic title="Найдено" value={total} />
              <Statistic
                title="Время поиска"
                value={elapsed ?? '—'}
                suffix="мс"
                valueStyle={{ color: (elapsed ?? 0) < 500 ? '#3f8600' : '#cf1322' }}
              />
              <Statistic title="Режим" value={mode === 'semantic' ? 'semantic' : 'scoring'} />
            </Space>
            {(elapsed ?? 0) >= 500 && (
              <Alert style={{ marginTop: 8 }} type="warning" showIcon message="Поиск занял более 500 мс (цель: < 500 мс)" />
            )}
          </div>
        )}
      </Card>

      <Card>
        {rows.length === 0 ? (
          <Empty description={searched ? 'Ничего не найдено — ослабьте фильтры' : 'Задайте параметры подбора'} />
        ) : (
          <Table
            rowKey="id"
            size="middle"
            pagination={{ pageSize: 10, showSizeChanger: false }}
            dataSource={rows}
            columns={columns as any}
            expandable={{
              expandedRowRender: (row: CandidateRow) => (
                <Paragraph style={{ margin: 0 }}>{row.bio || row.reason || 'Описание отсутствует'}</Paragraph>
              ),
            }}
          />
        )}
      </Card>

      <Drawer
        title={`Объяснение совпадения · ${explanation?.row ? nameOf(explanation.row) : ''}`}
        width={560}
        open={!!explanation}
        onClose={() => setExplanation(null)}
      >
        {explanation?.row && (
          <>
            <Descriptions column={2} size="small" style={{ marginBottom: 12 }}>
              <Descriptions.Item label="Итоговый score">
                <b>{explanation.row.matchScore ?? '—'}</b>
              </Descriptions.Item>
              <Descriptions.Item label="Семантика (cosine)">
                {explanation.row.matchExplanation?.semantic ?? (explanation.row.similarity ?? '—')}
              </Descriptions.Item>
            </Descriptions>
            <Table
              size="small"
              rowKey="factor"
              pagination={false}
              dataSource={details}
              columns={[
                { title: 'Фактор', dataIndex: 'factor' },
                {
                  title: 'Совпадение',
                  dataIndex: 'match',
                  width: 90,
                  render: (v: any) => (typeof v === 'number' ? `${Math.round(v * 100)}%` : String(v)),
                },
                {
                  title: 'Вес',
                  dataIndex: 'weight',
                  width: 70,
                  render: (v: any) => (typeof v === 'number' ? `${Math.round(v * 100)}%` : String(v)),
                },
                { title: 'Пояснение', dataIndex: 'explanation' },
              ]}
            />
            <Paragraph type="secondary" style={{ marginTop: 12 }}>
              Score = Σ(вес × совпадение). Порог отображения в выдаче — 40 баллов.
            </Paragraph>
          </>
        )}
      </Drawer>

      <Modal
        title={`Приглашение · ${inviteOpen ? nameOf(inviteOpen) : ''}`}
        open={!!inviteOpen}
        onCancel={() => setInviteOpen(null)}
        footer={null}
        destroyOnClose
      >
        <Form form={inviteForm} layout="vertical" onFinish={sendInvite}>
          <Form.Item label="Сообщение" name="message" rules={[{ required: true, min: 10, message: 'Минимум 10 символов' }]}>
            <Input.TextArea rows={4} />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label="Вилка от, ₽" name="salaryFrom" rules={[{ required: true }]}>
                <InputNumber min={0} step={10000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label="Вилка до, ₽" name="salaryTo" rules={[{ required: true }]}>
                <InputNumber min={0} step={10000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Space>
            <Button type="primary" htmlType="submit" loading={inviting}>
              Отправить приглашение
            </Button>
            <Button onClick={() => setInviteOpen(null)}>Отмена</Button>
          </Space>
        </Form>
      </Modal>
    </AppShell>
  );
}
