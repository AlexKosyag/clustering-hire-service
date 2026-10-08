'use client';

import { App, Card, Empty, List, Space, Spin, Tag, Typography } from 'antd';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { dateRu, salaryRange } from '@/lib/meta';

const { Paragraph, Text } = Typography;

const STATUS_COLOR: Record<string, string> = {
  PENDING: 'gold',
  VIEWED: 'blue',
  SHORTLISTED: 'green',
  REJECTED: 'red',
  WITHDRAWN: 'default',
};

const STATUS_LABEL: Record<string, string> = {
  PENDING: 'Отправлен',
  VIEWED: 'Просмотрен работодателем',
  SHORTLISTED: 'В коротком списке',
  REJECTED: 'Отклонён',
  WITHDRAWN: 'Отозван',
};

export default function ApplicationsPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .get('/applications/mine')
      .then(({ data }) => setItems(data.applications || []))
      .catch((error) => message.error(errMsg(error)))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <AppShell requireRole="CANDIDATE">
      <h1 className="page-title">Мои отклики</h1>
      <p className="page-sub">Отклики на вакансии и их статусы у работодателей.</p>

      <Card>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : items.length === 0 ? (
          <Empty
            description="Вы ещё ни на что не откликнулись"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          >
            <Link href="/vacancies">
              <Tag color="blue" style={{ cursor: 'pointer' }}>
                Перейти в каталог вакансий
              </Tag>
            </Link>
          </Empty>
        ) : (
          <List
            dataSource={items}
            renderItem={(item) => (
              <List.Item>
                <List.Item.Meta
                  title={
                    <Space wrap>
                      <b>{item.vacancy?.title || 'Вакансия'}</b>
                      <Tag color={STATUS_COLOR[item.status] || 'default'}>
                        {STATUS_LABEL[item.status] || item.status}
                      </Tag>
                      <Text type="secondary">{item.vacancy?.employer?.companyName}</Text>
                    </Space>
                  }
                  description={
                    <>
                      <Paragraph style={{ marginBottom: 4 }}>
                        {item.coverLetter || 'Без сопроводительного письма'}
                      </Paragraph>
                      <Text type="secondary">
                        {salaryRange(item.vacancy?.salaryFrom, item.vacancy?.salaryTo)} · отклик{' '}
                        {dateRu(item.createdAt)}
                      </Text>
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
