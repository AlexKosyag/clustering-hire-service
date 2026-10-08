'use client';

import { SearchOutlined, SendOutlined } from '@ant-design/icons';
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
  List,
  Modal,
  Row,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { GRADES, SPECIALIZATIONS, dateRu, gradeColor, gradeLabel, salaryRange, specLabel } from '@/lib/meta';

const { Paragraph, Text } = Typography;

export default function VacanciesPage() {
  const { user } = useAuth();
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [items, setItems] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<any>(null);
  const [applyFor, setApplyFor] = useState<any>(null);
  const [applying, setApplying] = useState(false);
  const [applyForm] = Form.useForm();

  const load = async (values?: any) => {
    setLoading(true);
    const v = values || {};
    try {
      const { data } = await api.get('/vacancies', {
        params: {
          limit: 50,
          search: v.search || undefined,
          specialization: v.specialization || undefined,
          grade: v.grade || undefined,
        },
      });
      setItems(data.vacancies || []);
      setTotal(data.total || 0);
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

  const apply = async (values: any) => {
    setApplying(true);
    try {
      await api.post(`/vacancies/${applyFor.id}/apply`, { coverLetter: values.coverLetter });
      message.success('Отклик отправлен работодателю');
      setApplyFor(null);
      applyForm.resetFields();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setApplying(false);
    }
  };

  const startApply = (vacancy: any) => {
    if (!user) {
      message.info('Войдите как кандидат, чтобы откликнуться');
      return;
    }
    if (user.role !== 'CANDIDATE') {
      message.warning('Откликнуться может только кандидат');
      return;
    }
    applyForm.setFieldsValue({ coverLetter: '' });
    setApplyFor(vacancy);
  };

  return (
    <AppShell>
      <h1 className="page-title">Каталог вакансий</h1>
      <p className="page-sub">Найдено вакансий: {total}</p>

      <Card style={{ marginBottom: 16 }}>
        <Form form={form} layout="inline" onFinish={load}>
          <Form.Item name="search">
            <Input placeholder="Поиск по названию и описанию" prefix={<SearchOutlined />} allowClear />
          </Form.Item>
          <Form.Item name="specialization">
            <Select allowClear placeholder="Специализация" options={SPECIALIZATIONS} style={{ minWidth: 170 }} />
          </Form.Item>
          <Form.Item name="grade">
            <Select allowClear placeholder="Грейд" options={GRADES} style={{ minWidth: 130 }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              Найти
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
        </div>
      ) : items.length === 0 ? (
        <Empty description="Вакансии не найдены" />
      ) : (
        <List
          dataSource={items}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
          renderItem={(vacancy) => (
            <Card size="small" style={{ marginBottom: 12 }}>
              <List.Item
                actions={[
                  <Button key="d" size="small" onClick={() => setSelected(vacancy)}>
                    Подробнее
                  </Button>,
                  <Button
                    key="a"
                    size="small"
                    type="primary"
                    icon={<SendOutlined />}
                    onClick={() => startApply(vacancy)}
                  >
                    Откликнуться
                  </Button>,
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space wrap>
                      <b>{vacancy.title}</b>
                      <Tag color="blue">{specLabel(vacancy.specialization)}</Tag>
                      <Tag color={gradeColor(vacancy.grade)}>{gradeLabel(vacancy.grade)}</Tag>
                      {vacancy.isActive ? <Tag color="green">активна</Tag> : <Tag>скрыта</Tag>}
                    </Space>
                  }
                  description={
                    <Space direction="vertical" size={0}>
                      <Text type="secondary">{vacancy.employer?.companyName}</Text>
                      <Space wrap>
                        <Text strong>{salaryRange(vacancy.salaryFrom, vacancy.salaryTo)}</Text>
                        <Text type="secondary">· опубликовано {dateRu(vacancy.createdAt)}</Text>
                      </Space>
                      <Space wrap style={{ marginTop: 4 }}>
                        {(vacancy.stack || []).slice(0, 8).map((s: string) => (
                          <Tag key={s}>{s}</Tag>
                        ))}
                      </Space>
                    </Space>
                  }
                />
              </List.Item>
            </Card>
          )}
        />
      )}

      <Drawer
        title={selected?.title}
        width={640}
        open={!!selected}
        onClose={() => setSelected(null)}
        extra={
          <Button type="primary" icon={<SendOutlined />} onClick={() => startApply(selected)}>
            Откликнуться
          </Button>
        }
      >
        {selected && (
          <>
            <Descriptions column={2} size="small">
              <Descriptions.Item label="Компания">{selected.employer?.companyName}</Descriptions.Item>
              <Descriptions.Item label="Отрасль">{selected.employer?.industry || '—'}</Descriptions.Item>
              <Descriptions.Item label="Специализация">
                <Tag color="blue">{specLabel(selected.specialization)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Грейд">
                <Tag color={gradeColor(selected.grade)}>{gradeLabel(selected.grade)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Вилка">{salaryRange(selected.salaryFrom, selected.salaryTo)}</Descriptions.Item>
              <Descriptions.Item label="Сайт">{selected.employer?.website || '—'}</Descriptions.Item>
            </Descriptions>

            <Paragraph style={{ marginTop: 16, whiteSpace: 'pre-wrap' }}>{selected.description}</Paragraph>

            <Space wrap>
              {(selected.stack || []).map((s: string) => (
                <Tag color="geekblue" key={s}>
                  {s}
                </Tag>
              ))}
            </Space>

            {!user && (
              <Alert
                style={{ marginTop: 16 }}
                type="info"
                showIcon
                message={
                  <span>
                    Чтобы откликнуться, <Link href="/login">войдите</Link> как кандидат.
                  </span>
                }
              />
            )}
          </>
        )}
      </Drawer>

      <Modal
        title={`Отклик · ${applyFor?.title || ''}`}
        open={!!applyFor}
        onCancel={() => setApplyFor(null)}
        footer={null}
        destroyOnClose
      >
        <Form form={applyForm} layout="vertical" onFinish={apply}>
          <Form.Item label="Сопроводительное письмо" name="coverLetter">
            <Input.TextArea
              rows={5}
              maxLength={4000}
              showCount
              placeholder="Почему вам интересна эта вакансия, ключевые технологии и достижения…"
            />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={applying} icon={<SendOutlined />}>
              Отправить отклик
            </Button>
            <Button onClick={() => setApplyFor(null)}>Отмена</Button>
          </Space>
        </Form>
      </Modal>
    </AppShell>
  );
}
