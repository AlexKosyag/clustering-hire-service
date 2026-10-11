'use client';

import { BellOutlined, CheckCircleOutlined, MailOutlined, TrophyOutlined } from '@ant-design/icons';
import { App, Card, Empty, List, Space, Spin, Tag, Typography } from 'antd';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { theme } from '@/lib/theme';

const { Paragraph, Text } = Typography;
const c = theme.colors;

const TYPE_ICON: Record<string, React.ReactNode> = {
  INVITATION_RECEIVED: <MailOutlined style={{ color: c.primary }} />,
  INVITATION_STATUS: <CheckCircleOutlined style={{ color: c.success }} />,
  TEST_RESULT: <TrophyOutlined style={{ color: c.warning }} />,
};

export default function NotificationsPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .get('/notifications')
      .then(({ data }) => {
        setItems(data.items || []);
        setTotal(data.total || 0);
      })
      .catch((error) => message.error(errMsg(error)))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <AppShell>
      <h1 className="page-title">
        Уведомления <Tag color="blue">{total}</Tag>
      </h1>
      <p className="page-sub">
        События приглашений и результатов тестирования — источник истины, а не отдельная
        таблица.
      </p>

      <Card>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : items.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space direction="vertical">
                <span>Новых событий нет</span>
                <Text type="secondary">
                  <BellOutlined /> Здесь появятся приглашения и результаты ваших тестов
                </Text>
              </Space>
            }
          />
        ) : (
          <List
            dataSource={items}
            renderItem={(item) => (
              <List.Item style={{ opacity: item.read ? 0.75 : 1 }}>
                <List.Item.Meta
                  avatar={TYPE_ICON[item.type] || <BellOutlined />}
                  title={
                    <Space wrap>
                      {!item.read && <Tag color="blue">новое</Tag>}
                      <b>{item.title}</b>
                      <Tag>{item.type}</Tag>
                    </Space>
                  }
                  description={
                    <>
                      <Paragraph style={{ marginBottom: 2 }}>{item.body}</Paragraph>
                      <Text type="secondary">{new Date(item.createdAt).toLocaleString('ru-RU')}</Text>
                    </>
                  }
                />
              </List.Item>
            )}
          />
        )}
      </Card>
    </AppShell>
  );
}
