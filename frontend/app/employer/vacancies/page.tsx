'use client';

import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import {
  App,
  Button,
  Card,
  Col,
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

export default function EmployerVacanciesPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();

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
      width: 110,
      render: (_: any, row: any) => (
        <Space>
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
      <p className="page-sub">Создание и редактирование ваших вакансий.</p>

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

    </AppShell>
  );
}
