'use client';

import { CheckOutlined, CloseOutlined, MailOutlined, EyeOutlined } from '@ant-design/icons';
import {
  App,
  Button,
  Card,
  Descriptions,
  Empty,
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
import { dateRu, salaryRange } from '@/lib/meta';
import { theme } from '@/lib/theme';

const { Paragraph, Text } = Typography;

const STATUS_CONFIG: Record<string, { color: string; label: string; icon: string }> = {
  SENT: { color: 'processing', label: 'Новое', icon: '📩' },
  VIEWED: { color: 'warning', label: 'Просмотрено', icon: '👀' },
  ACCEPTED: { color: 'success', label: 'Принято', icon: '✅' },
  REJECTED: { color: 'error', label: 'Отклонено', icon: '❌' },
  WITHDRAWN: { color: 'default', label: 'Отозвано', icon: '🚫' },
};

export default function InvitationsPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<string | null>(null);
  const [detail, setDetail] = useState<any>(null);

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
      message.success(status === 'ACCEPTED' ? 'Приглашение принято!' : 'Приглашение отклонено');
      if (status === 'ACCEPTED') {
        const { data } = await api.get(`/invitations/${id}`);
        setDetail(data);
      }
      await load();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setActing(null);
    }
  };

  const newCount = items.filter((i) => i.status === 'SENT').length;

  return (
    <AppShell requireRole="CANDIDATE">
      <div style={{ maxWidth: 900, margin: '0 auto' }}>
        {/* Header */}
        <div style={{ marginBottom: 24 }}>
          <h1 style={{ fontSize: 28, fontWeight: 700, color: theme.colors.text, margin: 0 }}>
            Приглашения
          </h1>
          <p style={{ color: theme.colors.textSecondary, marginTop: 8 }}>
            {newCount > 0
              ? `У вас ${newCount} новых приглашений`
              : 'Приглашения от работодателей появятся здесь'}
          </p>
        </div>

        {/* Invitations list */}
        {loading ? (
          <div style={{ textAlign: 'center', padding: 60 }}>
            <Spin size="large" />
          </div>
        ) : items.length === 0 ? (
          <Card style={{ borderRadius: theme.radius.lg }}>
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={
                <div>
                  <div style={{ fontSize: 48, marginBottom: 16 }}>📬</div>
                  <div style={{ fontSize: 16, color: theme.colors.textSecondary }}>
                    Приглашений пока нет
                  </div>
                  <div style={{ fontSize: 13, color: theme.colors.textMuted, marginTop: 8 }}>
                    Заполните профиль и пройдите тестирование, чтобы работодатели нашли вас
                  </div>
                </div>
              }
            />
          </Card>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {items.map((item) => {
              const statusConfig = STATUS_CONFIG[item.status] || STATUS_CONFIG.SENT;
              const isOpen = ['SENT', 'VIEWED'].includes(item.status);

              return (
                <Card
                  key={item.id}
                  style={{
                    borderRadius: theme.radius.lg,
                    border: `1px solid ${
                      item.status === 'SENT' ? theme.colors.primary : theme.colors.border
                    }`,
                    opacity: item.status === 'REJECTED' ? 0.7 : 1,
                  }}
                  bodyStyle={{ padding: 20 }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 16 }}>
                    {/* Left: Company info */}
                    <div style={{ flex: 1, minWidth: 200 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
                        <div
                          style={{
                            width: 48,
                            height: 48,
                            borderRadius: theme.radius.md,
                            background: `linear-gradient(135deg, ${theme.colors.primary}, ${theme.colors.primaryDark})`,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            color: 'white',
                            fontSize: 20,
                            fontWeight: 700,
                          }}
                        >
                          {(item.employer?.companyName || '?').charAt(0)}
                        </div>
                        <div>
                          <div style={{ fontWeight: 700, fontSize: 16, color: theme.colors.text }}>
                            {item.employer?.companyName || 'Компания'}
                          </div>
                          {item.employer?.industry && (
                            <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
                              {item.employer.industry}
                            </div>
                          )}
                        </div>
                      </div>

                      {/* Salary */}
                      <div
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: 8,
                          padding: '8px 16px',
                          background: theme.colors.successLight,
                          borderRadius: theme.radius.md,
                          marginBottom: 12,
                        }}
                      >
                        <span style={{ fontSize: 16 }}>💰</span>
                        <span style={{ fontWeight: 700, fontSize: 18, color: theme.colors.success }}>
                          {salaryRange(item.salaryFrom, item.salaryTo)}
                        </span>
                      </div>

                      {/* Message */}
                      {item.message && (
                        <Paragraph
                          style={{
                            marginBottom: 8,
                            color: theme.colors.textSecondary,
                            fontSize: 14,
                          }}
                        >
                          {item.message}
                        </Paragraph>
                      )}

                      {/* Date */}
                      <div style={{ fontSize: 12, color: theme.colors.textMuted }}>
                        {dateRu(item.createdAt)}
                      </div>
                    </div>

                    {/* Right: Status + Actions */}
                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 12 }}>
                      <Tag
                        color={statusConfig.color}
                        style={{ fontSize: 13, padding: '4px 12px' }}
                      >
                        {statusConfig.icon} {statusConfig.label}
                      </Tag>

                      <div style={{ display: 'flex', gap: 8 }}>
                        <Button
                          size="small"
                          icon={<EyeOutlined />}
                          onClick={() => setDetail(item)}
                        >
                          Подробнее
                        </Button>
                        {isOpen && (
                          <>
                            <Popconfirm
                              title="Принять приглашение?"
                              description="Контакты будут открыты работодателю"
                              onConfirm={() => respond(item.id, 'ACCEPTED')}
                            >
                              <Button
                                size="small"
                                type="primary"
                                icon={<CheckOutlined />}
                                loading={acting === item.id}
                              >
                                Принять
                              </Button>
                            </Popconfirm>
                            <Popconfirm
                              title="Отклонить?"
                              onConfirm={() => respond(item.id, 'REJECTED')}
                            >
                              <Button
                                size="small"
                                danger
                                icon={<CloseOutlined />}
                                loading={acting === item.id}
                              >
                                Отклонить
                              </Button>
                            </Popconfirm>
                          </>
                        )}
                      </div>
                    </div>
                  </div>
                </Card>
              );
            })}
          </div>
        )}
      </div>

      {/* Detail Modal */}
      <Modal
        title={
          <Space>
            <span>📬</span>
            <span>Приглашение</span>
          </Space>
        }
        open={!!detail}
        onCancel={() => setDetail(null)}
        footer={[
          <Button key="close" onClick={() => setDetail(null)}>
            Закрыть
          </Button>,
        ]}
        width={500}
      >
        {detail && (
          <div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 16,
                marginBottom: 20,
                padding: 16,
                background: theme.colors.bg,
                borderRadius: theme.radius.md,
              }}
            >
              <div
                style={{
                  width: 56,
                  height: 56,
                  borderRadius: theme.radius.md,
                  background: `linear-gradient(135deg, ${theme.colors.primary}, ${theme.colors.primaryDark})`,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'white',
                  fontSize: 24,
                  fontWeight: 700,
                }}
              >
                {(detail.employer?.companyName || '?').charAt(0)}
              </div>
              <div>
                <div style={{ fontWeight: 700, fontSize: 18, color: theme.colors.text }}>
                  {detail.employer?.companyName || 'Компания'}
                </div>
                {detail.employer?.website && (
                  <a
                    href={detail.employer.website}
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{ fontSize: 13, color: theme.colors.primary }}
                  >
                    {detail.employer.website}
                  </a>
                )}
              </div>
            </div>

            <Descriptions column={1} size="small" style={{ marginBottom: 16 }}>
              <Descriptions.Item label="💰 Вилка">
                <span style={{ fontWeight: 600, fontSize: 16, color: theme.colors.success }}>
                  {salaryRange(detail.salaryFrom, detail.salaryTo)}
                </span>
              </Descriptions.Item>
              <Descriptions.Item label="📅 Дата">
                {dateRu(detail.createdAt)}
              </Descriptions.Item>
              <Descriptions.Item label="Статус">
                <Tag color={STATUS_CONFIG[detail.status]?.color}>
                  {STATUS_CONFIG[detail.status]?.icon}{' '}
                  {STATUS_CONFIG[detail.status]?.label || detail.status}
                </Tag>
              </Descriptions.Item>
            </Descriptions>

            {detail.message && (
              <div style={{ marginBottom: 16 }}>
                <div style={{ fontWeight: 600, marginBottom: 8 }}>Сообщение:</div>
                <div
                  style={{
                    padding: 16,
                    background: theme.colors.bg,
                    borderRadius: theme.radius.md,
                    color: theme.colors.textSecondary,
                    lineHeight: 1.6,
                  }}
                >
                  {detail.message}
                </div>
              </div>
            )}

            {/* Contact info */}
            <div
              style={{
                padding: 16,
                background: detail.status === 'ACCEPTED' ? theme.colors.successLight : theme.colors.warningLight,
                borderRadius: theme.radius.md,
                border: `1px solid ${
                  detail.status === 'ACCEPTED' ? theme.colors.success : theme.colors.warning
                }30`,
              }}
            >
              <div style={{ fontWeight: 600, marginBottom: 8 }}>
                {detail.status === 'ACCEPTED' ? '✅ Контакты открыты' : '🔒 Контакты скрыты'}
              </div>
              {detail.status === 'ACCEPTED' ? (
                <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
                  <p style={{ margin: '4px 0' }}>
                    📧 Email: {detail.employer?.contactEmail || 'hr@company.com'}
                  </p>
                  <p style={{ margin: '4px 0' }}>
                    📞 Телефон: {detail.employer?.contactPhone || '—'}
                  </p>
                  <p style={{ margin: '4px 0' }}>
                    👤 Контактное лицо: {detail.employer?.contactPerson || '—'}
                  </p>
                </div>
              ) : (
                <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
                  Контакты работодателя станут доступны после принятия приглашения
                </div>
              )}
            </div>
          </div>
        )}
      </Modal>
    </AppShell>
  );
}