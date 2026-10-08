'use client';

import { Empty, Select, Space, Spin, Table, Tag, Typography } from 'antd';
import { App } from 'antd';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { dateRu, gradeColor, gradeLabel, specLabel } from '@/lib/meta';

const { Text, Paragraph } = Typography;

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

interface Row extends Record<string, any> {
  id: string;
}

export default function EmployerApplicationsPage() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<Row[]>([]);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<string | null>(null);

  const load = async () => {
    try {
      const { data: profile } = await api.get('/employers/profile');
      const { data: vacancies } = await api.get('/vacancies', {
        params: { employerId: profile.id, onlyActive: 'false', limit: 100 },
      });
      const list: Row[] = [];
      for (const vacancy of vacancies.vacancies || []) {
        const { data } = await api.get(`/vacancies/${vacancy.id}/applications`);
        for (const app of data.applications || []) {
          list.push({ ...app, vacancyTitle: vacancy.title, vacancyGrade: vacancy.grade });
        }
      }
      list.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
      setRows(list);
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

  const changeStatus = async (id: string, status: string) => {
    setActing(id);
    try {
      await api.post(`/applications/${id}/status`, {}, { params: { status } });
      setRows((prev) => prev.map((r) => (r.id === id ? { ...r, status } : r)));
      message.success('Статус обновлён');
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setActing(null);
    }
  };

  const columns = [
    {
      title: 'Кандидат',
      key: 'candidate',
      render: (_: any, row: Row) => (
        <Space direction="vertical" size={0}>
          <b>{row.candidate?.displayName || 'Кандидат'}</b>
          <Text type="secondary">
            {specLabel(row.candidate?.specialization)} · {gradeLabel(row.candidate?.verifiedGrade)} ·{' '}
            {row.candidate?.city || '—'}
          </Text>
        </Space>
      ),
    },
    {
      title: 'Вакансия',
      dataIndex: 'vacancyTitle',
      render: (v: string, row: Row) => (
        <Space direction="vertical" size={0}>
          <span>{v}</span>
          <Tag color={gradeColor(row.vacancyGrade)}>{gradeLabel(row.vacancyGrade)}</Tag>
        </Space>
      ),
    },
    {
      title: 'Тест',
      key: 'test',
      width: 130,
      render: (_: any, row: Row) => (
        <Tag color={row.candidate?.testStatus === 'PASSED' ? 'green' : row.candidate?.testStatus === 'FAILED' ? 'red' : 'default'}>
          {row.candidate?.testStatus || 'NOT_TAKEN'}
          {row.candidate?.testScore != null ? ` · ${row.candidate.testScore}` : ''}
        </Tag>
      ),
    },
    {
      title: 'Отклик',
      dataIndex: 'createdAt',
      width: 110,
      render: (v: string) => dateRu(v),
    },
    {
      title: 'Статус',
      key: 'status',
      width: 250,
      render: (_: any, row: Row) => (
        <Space direction="vertical" style={{ width: '100%' }}>
          <Tag color={APP_COLOR[row.status] || 'default'}>{row.status}</Tag>
          <Select
            size="small"
            style={{ width: '100%' }}
            value={row.status}
            loading={acting === row.id}
            options={APP_STATUS}
            onChange={(value) => changeStatus(row.id, value)}
          />
        </Space>
      ),
    },
  ];

  return (
    <AppShell requireRole="EMPLOYER">
      <h1 className="page-title">Отклики на вакансии</h1>
      <p className="page-sub">
        Все отклики по вашим вакансиям. Статусы: PENDING → VIEWED → SHORTLISTED / REJECTED.
      </p>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
        </div>
      ) : rows.length === 0 ? (
        <Empty description="Откликов пока нет — опубликуйте вакансии и дождитесь кандидатов" />
      ) : (
        <Table rowKey="id" dataSource={rows} columns={columns as any} pagination={{ pageSize: 10 }} />
      )}
    </AppShell>
  );
}
