'use client';

import { CheckOutlined, CloseOutlined, MailOutlined } from '@ant-design/icons';
import {
  App,
  Button,
  Card,
  Descriptions,
  Empty,
  List,
  Modal,
  Popconfirm,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { dateRu, money, salaryRange } from '@/lib/meta';

const { Paragraph, Text } = Typography;

const STATUS_COLOR: Record<string, string> = {
  SENT: 'blue',
  VIEWED: 'gold',
  ACCEPTED: 'green',
  REJECTED: 'red',
  WITHDRAWN: 'default',
};

const STATUS_LABEL: Record<string, string> = {
  SENT: 'Отправлено',
  VIEWED: 'Просмотрено',
  ACCEPTED: 'Принято',
  REJECTED: 'Отклонено',
  WITHDRAWN: 'Отозвано',
};

export default function InvitationsPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<string | null>(null);
  const [contacts, setContacts] = useState<any>(null);

  const load = async () => {
    try {
      const { data } = await api.get('/invitations');
      setItems(data.invitations || []);
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

  const respond = async (id: string, status: 'ACCEPTED' | 'REJECTED') => {
    setActing(id);
    try {
      await api.post(`/invitations/${id}/${status === 'ACCEPTED' ? 'accept' : 'reject'}`, {});
      message.success(status === 'ACCEPTED' ? 'Приглашение принято' : 'Приглашение отклонено');
      if (status === 'ACCEPTED') {
        const { data } = await api.get(`/invitations/${id}`);
        setContacts(data);
      }
      await load();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setActing(null);
    }
  };

  return (
    <AppShell requireRole="CANDIDATE">
      <h1 className="page-title">Приглашения от работодателей</h1>
      <p className="page-sub">
        Контакты работодателя и вашего кандидата открываются только после принятия приглашения
        («employer-first»).
      </p>

      <Card>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : items.length === 0 ? (
          <Empty description="Приглашений пока нет — заполните профиль, чтобы работодатели нашли вас" />
        ) : (
          <List
            dataSource={items}
            renderItem={(item) => {
              const open = ['SENT', 'VIEWED'].includes(item.status);
              return (
                <List.Item
                  actions={[
                    <Button
                      key="c"
                      size="small"
                      icon={<MailOutlined />}
                      onClick={() => setContacts(item)}
                    >
                      Подробнее
                    </Button>,
                    open ? (
                      <Popconfirm
                        key="a"
                        title="Принять приглашение?"
                        description="Контакты будут открыты, работодатель увидит ваш email и телефон."
                        onConfirm={() => respond(item.id, 'ACCEPTED')}
                      >
                        <Button size="small" type="primary" icon={<CheckOutlined />} loading={acting === item.id}>
                          Принять
                        </Button>
                      </Popconfirm>
                    ) : null,
                    open ? (
                      <Popconfirm key="r" title="Отклонить приглашение?" onConfirm={() => respond(item.id, 'REJECTED')}>
                        <Button size="small" danger icon={<CloseOutlined />} loading={acting === item.id}>
                          Отклонить
                        </Button>
                      </Popconfirm>
                    ) : null,
                  ].filter(Boolean)}
                >
                  <List.Item.Meta
                    title={
                      <Space wrap>
                        <b>{item.employer?.companyName || 'Компания'}</b>
                        <Tag color={STATUS_COLOR[item.status]}>{STATUS_LABEL[item.status] || item.status}</Tag>
                        {item.employer?.industry && <Tag>{item.employer.industry}</Tag>}
                      </Space>
                    }
                    description={
                      <>
                        <Paragraph style={{ marginBottom: 4 }}>{item.message}</Paragraph>
                        <Text type="secondary">
                          {salaryRange(item.salaryFrom, item.salaryTo)} · создано {dateRu(item.createdAt)}
                        </Text>
                      </>
                    }
                  />
                </List.Item>
              );
            }}
          />
        )}
      </Card>

      <Modal
        title="Приглашение"
        open={!!contacts}
        onCancel={() => setContacts(null)}
        footer={[
          <Button key="close" onClick={() => setContacts(null)}>
            Закрыть
          </Button>,
        ]}
      >
        {contacts && (
          <>
            <Descriptions column={1} size="small">
              <Descriptions.Item label="Компания">
                {contacts.employer?.companyName || '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Сайт">
                {contacts.employer?.website || '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Вилка">
                {salaryRange(contacts.salaryFrom, contacts.salaryTo)}
              </Descriptions.Item>
              <Descriptions.Item label="Статус">
                <Tag color={STATUS_COLOR[contacts.status]}>{STATUS_LABEL[contacts.status] || contacts.status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Сообщение">{contacts.message || '—'}</Descriptions.Item>
            </Descriptions>

            <Card size="small" style={{ marginTop: 12 }} title="Контактная информация">
              {contacts.candidate?.contacts ? (
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="Email">{contacts.candidate.contacts.email}</Descriptions.Item>
                  <Descriptions.Item label="Телефон">{contacts.candidate.contacts.phone || '—'}</Descriptions.Item>
                </Descriptions>
              ) : (
                <Paragraph type="secondary" style={{ marginBottom: 0 }}>
                  {contacts.status === 'ACCEPTED'
                    ? 'Вы приняли приглашение — ваши контакты открыты работодателю. Контакты компании появятся в переписке.'
                    : 'Контакты скрыты до принятия приглашения.'}
                </Paragraph>
              )}
            </Card>
          </>
        )}
      </Modal>
    </AppShell>
  );
}
