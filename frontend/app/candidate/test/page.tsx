'use client';

import { CheckCircleOutlined, CloseCircleOutlined, ThunderboltOutlined } from '@ant-design/icons';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Progress,
  Radio,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd';
import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { GRADES, SPECIALIZATIONS, dateRu, gradeLabel } from '@/lib/meta';

const { Title, Paragraph, Text } = Typography;

const mmss = (seconds: number) => {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
};

export default function TestPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [bootError, setBootError] = useState<string | null>(null);
  const [session, setSession] = useState<any>(null);
  const [question, setQuestion] = useState<any>(null);
  const [questionNumber, setQuestionNumber] = useState(1);
  const [totalQuestions, setTotalQuestions] = useState(20);
  const [currentScore, setCurrentScore] = useState<number>(0);
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [text, setText] = useState('');
  const [sending, setSending] = useState(false);
  const [feedback, setFeedback] = useState<any>(null);
  const [result, setResult] = useState<any>(null);
  const [canRetakeAt, setCanRetakeAt] = useState<string | null>(null);
  const [spec, setSpec] = useState<string | undefined>();
  const [declared, setDeclared] = useState<string | undefined>();
  const shownAt = useRef(Date.now());

  useEffect(() => {
    (async () => {
      try {
        const { data } = await api.get('/testing/active');
        setCanRetakeAt(data.canRetakeAt ?? null);
        if (data.session) {
          const cur = await api.get(`/testing/session/${data.session.id}/current`);
          if (cur.data.question) {
            setSession({ sessionId: data.session.id, timeLimitMinutes: 60 });
            setQuestion(cur.data.question);
            setQuestionNumber(cur.data.questionNumber || 1);
            setTotalQuestions(cur.data.totalQuestions || 20);
            setCurrentScore(cur.data.currentScore || 0);
            setSecondsLeft(cur.data.timeRemainingSeconds ?? null);
          } else if (cur.data.status === 'TIMED_OUT') {
            message.warning('Время теста истекло — сессия завершена');
          }
        }
      } catch (error) {
        setBootError(errMsg(error));
      } finally {
        setLoading(false);
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Отсчёт времени
  useEffect(() => {
    if (secondsLeft == null || !session || result) return;
    const timer = setInterval(() => setSecondsLeft((s) => (s == null ? s : Math.max(0, s - 1))), 1000);
    return () => clearInterval(timer);
  }, [session, result, secondsLeft != null]);

  const start = async () => {
    if (!spec) {
      message.warning('Выберите специализацию');
      return;
    }
    setSending(true);
    try {
      const { data } = await api.post('/testing/start', { specialization: spec, declaredGrade: declared });
      setSession(data);
      setQuestion(data.firstQuestion);
      setTotalQuestions(data.totalQuestions);
      setQuestionNumber(1);
      setCurrentScore(0);
      setSecondsLeft((data.timeLimitMinutes || 60) * 60);
      setFeedback(null);
      shownAt.current = Date.now();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSending(false);
    }
  };

  const submit = async () => {
    if (!question || !session) return;
    const body: Record<string, unknown> = {
      questionId: question.id,
      timeSpentSeconds: Math.max(1, Math.round((Date.now() - shownAt.current) / 1000)),
    };
    if (question.questionType === 'multiple_choice') {
      if (!selected) {
        message.warning('Выберите вариант ответа');
        return;
      }
      body.answer = selected;
    } else {
      if (!text.trim()) {
        message.warning('Введите ответ');
        return;
      }
      body.answerText = text;
    }
    setSending(true);
    try {
      const { data } = await api.post(`/testing/session/${session.sessionId}/answer`, body);
      setFeedback(data);
      setCurrentScore(data.currentScore ?? currentScore);
      setSelected(null);
      setText('');
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSending(false);
    }
  };

  const nextQuestion = () => {
    if (feedback?.nextQuestion) {
      setQuestion(feedback.nextQuestion);
      setQuestionNumber((n) => n + 1);
      setFeedback(null);
      shownAt.current = Date.now();
    } else {
      finish();
    }
  };

  const finish = async () => {
    if (!session) return;
    setSending(true);
    try {
      const { data } = await api.post(`/testing/session/${session.sessionId}/complete`, {});
      setResult(data);
      setSession(null);
      setQuestion(null);
      setSecondsLeft(null);
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSending(false);
    }
  };

  if (loading) {
    return (
      <AppShell requireRole="CANDIDATE">
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
        </div>
      </AppShell>
    );
  }

  // ===== Результат =====
  if (result) {
    const status = result.recommendation === 'UPGRADE' ? 'success' : result.recommendation === 'PASS' ? 'success' : 'error';
    const details = result.details
      ? [
          { key: 'easy', name: 'Лёгкие', ...result.details.easy },
          { key: 'medium', name: 'Средние', ...result.details.medium },
          { key: 'hard', name: 'Сложные', ...result.details.hard },
        ]
      : [];
    return (
      <AppShell requireRole="CANDIDATE">
        <div className="test-question">
          <Result
            status={status as any}
            title={`Результат: ${result.recommendation}`}
            subTitle={`Балл ${result.score} · θ = ${result.theta} · ${gradeLabel(result.gradeBefore)} → ${gradeLabel(result.gradeAfter)}`}
          />
          <Card>
            <Row gutter={[16, 16]}>
              <Col span={6}>
                <Statistic title="Балл" value={result.score} suffix="из 100" />
              </Col>
              <Col span={6}>
                <Statistic title="Уровень θ" value={result.theta} />
              </Col>
              <Col span={6}>
                <Statistic title="Грейд" value={`${gradeLabel(result.gradeBefore)} → ${gradeLabel(result.gradeAfter)}`} />
              </Col>
              <Col span={6}>
                <Statistic
                  title="Пересдача"
                  value={result.cooldownUntil ? dateRu(result.cooldownUntil) : '—'}
                />
              </Col>
            </Row>
            <Paragraph style={{ marginTop: 16 }}>
              <Text strong>Рекомендация: </Text>
              {result.recommendation === 'UPGRADE'
                ? 'результат подтверждает более высокий грейд — заявленный уровень будет повышен.'
                : result.recommendation === 'PASS'
                ? 'результат подтверждает заявленный уровень.'
                : 'результат ниже заявленного уровня — рекомендуем подготовиться и пройти тест повторно.'}
            </Paragraph>
          </Card>

          <Card title="Разбор по сложности вопросов" style={{ marginTop: 16 }}>
            <Table
              size="small"
              pagination={false}
              dataSource={details}
              columns={[
                { title: 'Уровень', dataIndex: 'name' },
                {
                  title: 'Верно',
                  dataIndex: 'correct',
                  render: (v: number, row: any) => `${v} из ${row.total}`,
                },
                {
                  title: 'Точность',
                  render: (_, row: any) => (row.total ? Math.round((row.correct / row.total) * 100) : 0) + '%',
                },
              ]}
            />
          </Card>

          {result.antiCheat?.isSuspicious && (
            <Alert
              style={{ marginTop: 16 }}
              type="warning"
              showIcon
              message="Сессия помечена как подозрительная"
              description={`Флаги: ${(result.antiCheat.flags || []).join(', ')} · уверенность ${Math.round(
                (result.antiCheat.confidence || 0) * 100,
              )}%`}
            />
          )}

          <Space style={{ marginTop: 16 }}>
            <Link href="/candidate">
              <Button type="primary">Вернуться в профиль</Button>
            </Link>
          </Space>
        </div>
      </AppShell>
    );
  }

  // ===== Экран запуска =====
  if (!session) {
    const cooldownActive = !!canRetakeAt && new Date(canRetakeAt).getTime() > Date.now();
    return (
      <AppShell requireRole="CANDIDATE">
        <div className="test-question">
          <h1 className="page-title">Адаптивное тестирование (CAT)</h1>
          <p className="page-sub">
            Вопросы подбираются по модели 3PL IRT: следующий вопрос выбирается по максимальной Fisher
            Information при текущей оценке θ.
          </p>

          {bootError && <Alert type="error" showIcon message={bootError} style={{ marginBottom: 16 }} />}

          <Card title="Правила">
            <ul>
              <li>
                <b>20 вопросов</b>, время — <b>60 минут</b>; сложность адаптируется к вашим ответам.
              </li>
              <li>
                Пороги грейдов: Junior θ &lt; −1.0 · Middle −1.0 · Middle+ 0.0 · Senior 0.5 · Lead 1.0 ·
                Principal 1.5.
              </li>
              <li>
                Рекомендация <Tag color="green">UPGRADE</Tag> выдаётся при балле ≥ 85 и повышении грейда.
              </li>
              <li>Повторное прохождение — через 90 дней.</li>
              <li>Анти-чит: время ответов, пул вопросов, волатильность θ и порядок вариантов.</li>
            </ul>

            {cooldownActive && (
              <Alert
                type="warning"
                showIcon
                message={`Пересдача возможна ${dateRu(canRetakeAt)}`}
                style={{ marginBottom: 16 }}
              />
            )}

            <Row gutter={12}>
              <Col span={12}>
                <div style={{ marginBottom: 8 }}>
                  <Text strong>Специализация</Text>
                </div>
                <Select
                  style={{ width: '100%' }}
                  placeholder="Backend, Frontend, DevOps…"
                  options={SPECIALIZATIONS}
                  value={spec}
                  onChange={setSpec}
                  disabled={cooldownActive}
                />
              </Col>
              <Col span={12}>
                <div style={{ marginBottom: 8 }}>
                  <Text strong>Заявленный грейд (необязательно)</Text>
                </div>
                <Select
                  style={{ width: '100%' }}
                  placeholder="Middle"
                  allowClear
                  options={GRADES}
                  value={declared}
                  onChange={setDeclared}
                  disabled={cooldownActive}
                />
              </Col>
            </Row>

            <Button
              type="primary"
              size="large"
              style={{ marginTop: 16 }}
              loading={sending}
              disabled={cooldownActive}
              onClick={start}
              icon={<ThunderboltOutlined />}
            >
              Начать тест
            </Button>
          </Card>
        </div>
      </AppShell>
    );
  }

  // ===== Вопрос =====
  const answered = questionNumber - 1;
  const percent = Math.round((answered / totalQuestions) * 100);
  const isText = question && question.questionType !== 'multiple_choice';

  return (
    <AppShell requireRole="CANDIDATE">
      <div className="test-question">
        <Card
          title={
            <Space size="large" wrap>
              <span>
                Вопрос {questionNumber} из {totalQuestions}
              </span>
              <Tag color={secondsLeft != null && secondsLeft < 300 ? 'red' : 'blue'}>
                {secondsLeft != null ? mmss(secondsLeft) : '—'}
              </Tag>
              <Tag color="green">Балл: {Math.round(currentScore)}</Tag>
              {feedback && <Tag color={feedback.correct ? 'green' : 'red'}>{feedback.correct ? 'верно' : 'неверно'}</Tag>}
            </Space>
          }
          extra={<Progress type="circle" width={56} percent={percent} size="small" />}
        >
          {question && (
            <>
              <Title level={4} style={{ marginTop: 0 }}>
                {question.text}
              </Title>
              {question.codeSnippet && (
                <pre
                  style={{
                    background: '#f6f8fa',
                    padding: 12,
                    borderRadius: 8,
                    overflowX: 'auto',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {question.codeSnippet}
                </pre>
              )}
              <div style={{ marginTop: 12 }}>
                <Tag>{question.questionType}</Tag>
                <Tag color="default">сложность {question.difficulty}</Tag>
              </div>

              {!feedback && (
                <>
                  {isText ? (
                    <>
                      <Paragraph type="secondary" style={{ marginTop: 12 }}>
                        Развернутый ответ: будут оценены ключевые слова и структура ответа (частичные
                        баллы возможны).
                      </Paragraph>
                      <textarea
                        style={{
                          width: '100%',
                          minHeight: 180,
                          padding: 12,
                          borderRadius: 8,
                          border: '1px solid #d9d9d9',
                          fontFamily: 'inherit',
                          fontSize: 14,
                        }}
                        value={text}
                        onChange={(e) => setText(e.target.value)}
                        placeholder="Опишите подход, инструменты, типичные ошибки…"
                      />
                    </>
                  ) : (
                    <Radio.Group
                      style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 12 }}
                      value={selected}
                      onChange={(e) => setSelected(e.target.value)}
                    >
                      {(question.options || []).map((option: any, index: number) => (
                        <Radio key={option.id} value={option.id}>
                          <b>{String.fromCharCode(65 + index)}.</b> {option.text}
                        </Radio>
                      ))}
                    </Radio.Group>
                  )}

                  <Button
                    type="primary"
                    size="large"
                    style={{ marginTop: 16 }}
                    loading={sending}
                    onClick={submit}
                  >
                    Ответить
                  </Button>
                </>
              )}

              {feedback && (
                <Alert
                  style={{ marginTop: 16 }}
                  type={feedback.correct ? 'success' : 'info'}
                  icon={feedback.correct ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
                  message={
                    feedback.correct
                      ? 'Ответ засчитан'
                      : `Ответ не засчитан · частичный балл ${feedback.score}`
                  }
                  description={
                    <>
                      {feedback.explanation && <div>{feedback.explanation}</div>}
                      <div style={{ marginTop: 6 }}>
                        <Text type="secondary">
                          Текущий балл: <b>{Math.round(feedback.currentScore)}</b> · θ ≈ {feedback.theta} ·
                          осталось вопросов: {feedback.questionsRemaining}
                        </Text>
                      </div>
                    </>
                  }
                />
              )}

              {feedback && (
                <Button
                  type="primary"
                  size="large"
                  style={{ marginTop: 16 }}
                  loading={sending}
                  onClick={nextQuestion}
                >
                  {feedback.isFinished || !feedback.nextQuestion ? 'Завершить тест' : 'Следующий вопрос'}
                </Button>
              )}
            </>
          )}

          {!question && (
            <Result
              status="info"
              title="Сессия завершена"
              extra={
                <Button type="primary" onClick={finish} loading={sending}>
                  Показать результат
                </Button>
              }
            />
          )}
        </Card>
      </div>
    </AppShell>
  );
}
