'use client';

import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { GRADES, SPECIALIZATIONS } from '@/lib/meta';
import CandidateCard from '@/components/ui/CandidateCard';
import MatchExplanation from '@/components/ui/MatchExplanation';
import ScoreRing from '@/components/ui/ScoreRing';
import { theme } from '@/lib/theme';

const { Paragraph, Text } = Typography;

interface CandidateRow {
  id: string;
  displayName?: string;
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
  achievements?: Array<{ name: string; points: number }>;
  matchScore?: number;
  matchScoreV2?: number;
  matchBreakdown?: Record<string, { score: number; weight: number; contribution: number }>;
  matchExplanation?: any;
  semanticSimilarity?: number;
  rerankerScore?: number | null;
  retrievalRank?: number;
  finalRank?: number;
}

interface SearchResponse {
  total: number;
  candidates: CandidateRow[];
  queryUnderstanding?: {
    originalQuery: string;
    parsedSpecialization: string | null;
    parsedSeniority: string | null;
    parsedSkills: string[];
    embeddingProvider: string;
  };
  timing?: {
    retrieval: number;
    scoring: number;
    reranking: number;
    total: number;
  };
}

export default function EmployerSearchPageV2() {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<'v2' | 'v1' | 'semantic'>('v2');
  const [result, setResult] = useState<SearchResponse | null>(null);
  const [rows, setRows] = useState<CandidateRow[]>([]);
  const [total, setTotal] = useState(0);
  const [explanation, setExplanation] = useState<CandidateRow | null>(null);
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
    try {
      if (mode === 'semantic') {
        const { data } = await api.post('/matching/v2/semantic', {
          query: v.description || '',
          limit: v.limit || 10,
        });
        setRows(data.candidates || []);
        setTotal((data.candidates || []).length);
        setResult({ total: data.candidates?.length || 0, candidates: data.candidates || [] });
      } else if (mode === 'v2') {
        const { data } = await api.post('/matching/v2/search', {
          description: v.description,
          specialization: v.specialization,
          grade: v.grade,
          stack: v.stack,
          hasAchievements: v.hasAchievements,
          limit: v.limit || 10,
        });
        setRows(data.candidates || []);
        setTotal(data.total || 0);
        setResult(data);
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
        setResult({ total: data.total, candidates: data.candidates });
      }
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
      message.success('Приглашение отправлено');
      setInviteOpen(null);
      inviteForm.resetFields();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setInviting(false);
    }
  };

  return (
    <AppShell requireRole="EMPLOYER">
      <div style={{ maxWidth: 1200, margin: '0 auto' }}>
        {/* Header */}
        <div style={{ marginBottom: 24 }}>
          <h1 style={{ fontSize: 28, fontWeight: 700, color: theme.colors.text, margin: 0 }}>
            Поиск кандидатов
          </h1>
          <p style={{ color: theme.colors.textSecondary, marginTop: 8 }}>
            ML-powered подбор с семантическим анализом и персонализированным ranking
          </p>
        </div>

        {/* Search form */}
        <Card
          style={{
            marginBottom: 24,
            borderRadius: theme.radius.lg,
            border: `1px solid ${theme.colors.border}`,
          }}
        >
          <Form form={form} layout="vertical" onFinish={onSearch}>
            <Row gutter={[16, 12]}>
              <Col xs={24} md={12}>
                <Form.Item
                  label="Описание потребности"
                  name="description"
                  tooltip="ML проанализирует текст и извлечёт специализацию, грейд и навыки"
                >
                  <Input.TextArea
                    rows={3}
                    placeholder="Ищем senior backend разработчика на Go с опытом в микросервисах и очередях, обязательно знание Kubernetes"
                    style={{ fontSize: 14 }}
                  />
                </Form.Item>
              </Col>
              <Col xs={12} md={4}>
                <Form.Item label="Специализация" name="specialization">
                  <Select allowClear options={SPECIALIZATIONS} placeholder="авто" />
                </Form.Item>
              </Col>
              <Col xs={12} md={3}>
                <Form.Item label="Грейд" name="grade">
                  <Select allowClear options={GRADES} placeholder="авто" />
                </Form.Item>
              </Col>
              <Col xs={24} md={5}>
                <Form.Item label="Стек" name="stack">
                  <Select mode="tags" placeholder="Node.js, Redis" />
                </Form.Item>
              </Col>
            </Row>

            <Space wrap>
              <Button
                type="primary"
                htmlType="submit"
                loading={loading}
                onClick={() => setMode('v2')}
                size="large"
              >
                🔍 Подобрать (V2)
              </Button>
              <Button
                loading={loading}
                onClick={() => {
                  setMode('v1');
                  onSearch();
                }}
              >
                V1 (legacy)
              </Button>
              <Button
                loading={loading}
                onClick={() => {
                  setMode('semantic');
                  onSearch();
                }}
              >
                Семантика
              </Button>
              <Form.Item
                name="hasAchievements"
                valuePropName="checked"
                style={{ marginBottom: 0, marginLeft: 8 }}
              >
                <Switch checkedChildren="с ФСП" unCheckedChildren="все" />
              </Form.Item>
            </Space>
          </Form>

          {/* Search results stats */}
          {searched && result && (
            <div
              style={{
                marginTop: 16,
                padding: 16,
                background: theme.colors.bg,
                borderRadius: theme.radius.md,
              }}
            >
              <Space size="large" wrap>
                <Statistic title="Найдено" value={total} />
                {result.timing && (
                  <Statistic
                    title="Время"
                    value={result.timing.total}
                    suffix="мс"
                    valueStyle={{
                      color:
                        result.timing.total < 500
                          ? theme.colors.success
                          : theme.colors.error,
                    }}
                  />
                )}
                {result.queryUnderstanding && (
                  <div>
                    <div style={{ fontSize: 12, color: theme.colors.textMuted }}>
                      ML понял запрос:
                    </div>
                    <Space size={4} wrap>
                      {result.queryUnderstanding.parsedSpecialization && (
                        <Tag color="blue">
                          {result.queryUnderstanding.parsedSpecialization}
                        </Tag>
                      )}
                      {result.queryUnderstanding.parsedSeniority && (
                        <Tag color="green">
                          {result.queryUnderstanding.parsedSeniority}
                        </Tag>
                      )}
                      {result.queryUnderstanding.parsedSkills.slice(0, 5).map((skill) => (
                        <Tag key={skill}>{skill}</Tag>
                      ))}
                    </Space>
                  </div>
                )}
              </Space>
            </div>
          )}
        </Card>

        {/* Candidate cards */}
        {loading ? (
          <div style={{ textAlign: 'center', padding: 48 }}>
            <Spin size="large" />
            <div style={{ marginTop: 16, color: theme.colors.textMuted }}>
              Анализируем запрос и подбираем кандидатов...
            </div>
          </div>
        ) : rows.length === 0 ? (
          <Empty
            description={
              searched
                ? 'Ничего не найдено — попробуйте изменить параметры'
                : 'Задайте параметры подбора'
            }
            style={{ padding: 48 }}
          />
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {rows.map((candidate) => (
              <CandidateCard
                key={candidate.id}
                candidate={candidate}
                onInvite={(id) => {
                  const row = rows.find((r) => r.id === id);
                  if (row) {
                    setInviteOpen(row);
                    inviteForm.setFieldsValue({
                      message: `Здравствуйте! Приглашаем вас обсудить вакансию.`,
                    });
                  }
                }}
                onDetails={(id) => {
                  const row = rows.find((r) => r.id === id);
                  if (row) setExplanation(row);
                }}
              />
            ))}
          </div>
        )}
      </div>

      {/* Explanation Drawer */}
      <Drawer
        title={`Объяснение совпадения`}
        width={600}
        open={!!explanation}
        onClose={() => setExplanation(null)}
      >
        {explanation && (
          <MatchExplanation
            score={explanation.matchScore || 0}
            overall={explanation.matchExplanation?.overall}
            details={explanation.matchExplanation?.details || []}
            breakdown={explanation.matchBreakdown}
            semanticSimilarity={explanation.semanticSimilarity}
            rerankerScore={explanation.rerankerScore}
          />
        )}
      </Drawer>

      {/* Invite Modal */}
      <Modal
        title={`Приглашение · ${inviteOpen?.displayName || ''}`}
        open={!!inviteOpen}
        onCancel={() => setInviteOpen(null)}
        footer={null}
        destroyOnClose
      >
        <Form form={inviteForm} layout="vertical" onFinish={sendInvite}>
          <Form.Item
            label="Сообщение"
            name="message"
            rules={[{ required: true, min: 10, message: 'Минимум 10 символов' }]}
          >
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