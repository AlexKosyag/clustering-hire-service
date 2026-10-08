'use client';

import { DeleteOutlined, EditOutlined, PlusOutlined, UnorderedListOutlined } from '@ant-design/icons';
import {
  App,
  Alert,
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
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
} from 'antd';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { GRADES, SPECIALIZATIONS, gradeColor, gradeLabel, salaryRange, specLabel } from '@/lib/meta';

const APP_STATUS = [
  { value: 'PENDING', label: 'PENDING — отправлен' },
  { value: 'VIEWED', label: 'VIEWED — просмотрен' },
  { value: 'SHORTLISTED', label: 'SHORTLISTED — короткий список' },
  { value: 'REJECTED', label: 'REJECTED — отклонён' },
  { value: 'WITHDRAWN', label: 'WITHDRAWN — отозван' },
];

const APP_COLOR: Record<string, string> = {
  PENDING: 'gold',
  VIEWED: 'blue',
  SHORTLISTED: 'green',
  REJECTED: 'red',
  WITHDRAWN: 'default',
};

export default function EmployerVacanciesPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();

  const [drawerVacancy, setDrawerVacancy] = useState<any>(null);
  const [apps, setApps] = useState<any[]>([]);
  const [appsLoading, setAppsLoading] = useState(false);
  const [acting, setActing] = useState<string | null>(null);

  const load = async () => {
    try {
      const { data: profile } = await api.get('/employers/profile');
      const { data } = await api.get('/vacancies', {
        params: { employerId: profile.id, onlyActive: 'false', limit: 100 },
      });
      setItems(data.vacancies || []);
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

  // Открытие карточки откликов по ссылке с дашборда (?open=<vacancyId>)
  useEffect(() => {
    const id = new URLSearchParams(window.location.search).get('open');
    if (id) {
      const vacancy = items.find((v) => v.id === id);
      if (vacancy) openApplications(vacancy);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({
      specialization: 'BACKEND',
      grade: 'MIDDLE',
      stack: ['Node.js', 'PostgreSQL'],
      salaryFrom: 200000,
      salaryTo: 300000,
    });
    setModalOpen(true);
  };

  const openEdit = (vacancy: any) => {
    setEditing(vacancy);
    form.setFieldsValue({
      title: vacancy.title,
      description: vacancy.description,
      specialization: vacancy.specialization,
      grade: vacancy.grade,
      stack: vacancy.stack,
      salaryFrom: vacancy.salaryFrom,
      salaryTo: vacancy.salaryTo,
      isActive: vacancy.isActive,
    });
    setModalOpen(true);
  };

  const submit = async (values: any) => {
    setSaving(true);
    try {
      if (editing) {
        await api.put(`/vacancies/${editing.id}`, values);
        message.success('Вакансия обновлена');
      } else {
        await api.post('/vacancies', values);
        message.success('Вакансия создана');
      }
      setModalOpen(false);
      await load();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await api.delete(`/vacancies/${id}`);
      message.success('Вакансия удалена');
      await load();
    } catch (error) {
      message.error(errMsg(error));
    }
  };

  const toggle = async (vacancy: any, isActive: boolean) => {
    try {
      await api.put(`/vacancies/${vacancy.id}`, { isActive });
      await load();
    } catch (error) {
      message.error(errMsg(error));
    }
  };

  const openApplications = async (vacancy: any) => {
    setDrawerVacancy(vacancy);
    setAppsLoading(true);
    setApps([]);
    try {
      const { data } = await api.get(`/vacancies/${vacancy.id}/applications`);
      setApps(data.applications || []);
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setAppsLoading(false);
    }
  };

  const changeStatus = async (applicationId: string, status: string) => {
    setActing(applicationId);
    try {
      await api.post(`/applications/${applicationId}/status`, {}, { params: { status } });
      setApps((prev) => prev.map((a) => (a.id === applicationId ? { ...a, status } : a)));
      message.success('Статус отклика обновлён');
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setActing(null);
    }
  };

  const columns = [
    {
      title: 'Вакансия',
      key: 'title',
      render: (_: any, row: any) => (
        <Space direction="vertical" size={0}>
          <b>{row.title}</b>
          <span className="muted">{salaryRange(row.salaryFrom, row.salaryTo)}</span>
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
      dataIndex: 'grade',
      render: (v: string) => <Tag color={gradeColor(v)}>{gradeLabel(v)}</Tag>,
    },
    {
      title: 'Стек',
      dataIndex: 'stack',
      render: (stack: string[]) => (stack || []).slice(0, 5).map((s) => <Tag key={s}>{s}</Tag>),
    },
    {
      title: 'Активна',
      dataIndex: 'isActive',
      width: 130,
      render: (v: boolean, row: any) => (
        <Switch
          size="small"
          checked={v}
          checkedChildren="да"
          unCheckedChildren="нет"
          onChange={(checked) => toggle(row, checked)}
        />
      ),
    },
    {
      title: 'Действия',
      key: 'actions',
      width: 230,
      render: (_: any, row: any) => (
        <Space>
          <Button size="small" icon={<UnorderedListOutlined />} onClick={() => openApplications(row)}>
            Отклики
          </Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)} />
          <Popconfirm title="Удалить вакансию?" onConfirm={() => remove(row.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <AppShell requireRole="EMPLOYER">
      <h1 className="page-title">Мои вакансии</h1>
      <p className="page-sub">Создание, редактирование и отклики кандидатов на ваши вакансии.</p>

      <Card
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            Создать вакансию
          </Button>
        }
      >
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : items.length === 0 ? (
          <Empty description="Вакансий пока нет" />
        ) : (
          <Table rowKey="id" dataSource={items} columns={columns as any} pagination={false} />
        )}
      </Card>

      <Modal
        title={editing ? 'Редактирование вакансии' : 'Новая вакансия'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={null}
        width={760}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={submit}>
          <Form.Item
            label="Название"
            name="title"
            rules={[{ required: true, min: 3, max: 160, message: 'От 3 до 160 символов' }]}
          >
            <Input placeholder="Backend-разработчик (Node.js)" />
          </Form.Item>
          <Form.Item
            label="Описание"
            name="description"
            rules={[{ required: true, min: 20, max: 10000, message: 'От 20 до 10000 символов' }]}
          >
            <Input.TextArea rows={5} placeholder="Задачи, требования, условия…" />
          </Form.Item>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="Специализация" name="specialization" rules={[{ required: true }]}>
                <Select options={SPECIALIZATIONS} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Грейд" name="grade" rules={[{ required: true }]}>
                <Select options={GRADES} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Стек" name="stack" rules={[{ required: true }]}>
                <Select mode="tags" placeholder="Node.js, PostgreSQL" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="Вилка от, ₽" name="salaryFrom" rules={[{ required: true }]}>
                <InputNumber min={0} step={10000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Вилка до, ₽" name="salaryTo" rules={[{ required: true }]}>
                <InputNumber min={0} step={10000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            {editing && (
              <Col span={8}>
                <Form.Item label="Опубликована" name="isActive" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
            )}
          </Row>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              {editing ? 'Сохранить' : 'Создать'}
            </Button>
            <Button onClick={() => setModalOpen(false)}>Отмена</Button>
          </Space>
        </Form>
      </Modal>

      <Drawer
        title={`Отклики · ${drawerVacancy?.title || ''}`}
        width={640}
        open={!!drawerVacancy}
        onClose={() => setDrawerVacancy(null)}
      >
        {appsLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : apps.length === 0 ? (
          <Empty description="Откликов пока нет" />
        ) : (
          apps.map((app) => (
            <Card key={app.id} size="small" style={{ marginBottom: 12 }}>
              <Descriptions column={1} size="small">
                <Descriptions.Item label="Кандидат">
                  <b>{app.candidate?.displayName || 'Кандидат'}</b>
                </Descriptions.Item>
                <Descriptions.Item label="Специализация / грейд">
                  <Tag color="blue">{specLabel(app.candidate?.specialization)}</Tag>
                  <Tag color={gradeColor(app.candidate?.verifiedGrade)}>
                    {gradeLabel(app.candidate?.verifiedGrade)}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="Стек">
                  {(app.candidate?.stack || []).map((s: string) => (
                    <Tag key={s}>{s}</Tag>
                  ))}
                </Descriptions.Item>
                <Descriptions.Item label="Тест">
                  <Tag color={app.candidate?.testStatus === 'PASSED' ? 'green' : 'default'}>
                    {app.candidate?.testStatus || 'NOT_TAKEN'}{' '}
                    {app.candidate?.testScore != null ? `· ${app.candidate.testScore}` : ''}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="Письмо">{app.coverLetter || '—'}</Descriptions.Item>
                <Descriptions.Item label="Статус">
                  <Select
                    size="small"
                    style={{ width: '100%' }}
                    value={app.status}
                    loading={acting === app.id}
                    options={APP_STATUS}
                    onChange={(value) => changeStatus(app.id, value)}
                  />
                </Descriptions.Item>
              </Descriptions>
              {app.candidate?.contacts && (
                <Alert
                  style={{ marginTop: 8 }}
                  type="success"
                  showIcon
                  message={`Контакты открыты: ${app.candidate.contacts.email}`}
                />
              )}
            </Card>
          ))
        )}
      </Drawer>
    </AppShell>
  );
}
