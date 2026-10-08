'use client';

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
import { theme } from '@/lib/theme';
import GradeBadge from '@/components/ui/GradeBadge';

const { Title, Paragraph, Text } = Typography;

const mmss = (seconds: number) => {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
};

const TASK_TYPE_LABELS: Record<string, { label: string; icon: string; color: string }> = {
  multiple_choice: { label: 'Выбор ответа', icon: '🔘', color: 'blue' },
  code_execution: { label: 'Задача с кодом', icon: '💻', color: 'green' },
  llm_open: { label: 'Развернутый ответ', icon: '📝', color: 'purple' },
  code_review: { label: 'Code Review', icon: '🔍', color: 'orange' },
  system_design: { label: 'Проектирование', icon: '🏗️', color: 'red' },
};

interface BehavioralData {
  timeSpentSeconds: number;
  copyPasteEvents: number;
  tabSwitches: number;
  keystrokeCount: number;
  idleTimeSeconds: number;
}

export default function TestPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [session, setSession] = useState<any>(null);
  const [task, setTask] = useState<any>(null);
  const [taskNumber, setTaskNumber] = useState(1);
  const [totalTasks, setTotalTasks] = useState(15);
  const [currentScore, setCurrentScore] = useState(0);
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [canRetakeAt, setCanRetakeAt] = useState<string | null>(null);
  const [spec, setSpec] = useState<string | undefined>();
  const [declared, setDeclared] = useState<string | undefined>();
  const [v2Mode, setV2Mode] = useState(true);

  // Answer state per task type
  const [selectedOption, setSelectedOption] = useState<string | null>(null);
  const [answerText, setAnswerText] = useState('');
  const [code, setCode] = useState('');

  // Behavioral tracking
  const behavioralRef = useRef<BehavioralData>({
    timeSpentSeconds: 0,
    copyPasteEvents: 0,
    tabSwitches: 0,
    keystrokeCount: 0,
    idleTimeSeconds: 0,
  });
  const taskShownAt = useRef(Date.now());

  // Check active session on load
  useEffect(() => {
    (async () => {
      try {
        const { data } = await api.get('/testing/active');
        setCanRetakeAt(data.canRetakeAt ?? null);
        if (data.session) {
          const cur = await api.get(`/testing/v2/session/${data.session.id}/current`);
          if (cur.data.task) {
            setSession({ sessionId: data.session.id, timeLimitMinutes: 90 });
            setTask(cur.data.task);
            setTaskNumber(cur.data.taskNumber || 1);
            setTotalTasks(cur.data.totalTasks || 15);
            setCurrentScore(cur.data.currentScore || 0);
            setSecondsLeft(cur.data.timeRemainingSeconds ?? null);
            resetAnswerState();
          }
        }
      } catch {
        // No active session
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  // Timer countdown
  useEffect(() => {
    if (secondsLeft == null || !session || result) return;
    const timer = setInterval(
      () => setSecondsLeft((s) => (s == null ? s : Math.max(0, s - 1))),
      1000,
    );
    return () => clearInterval(timer);
  }, [session, result, secondsLeft != null]);

  // Behavioral tracking
  useEffect(() => {
    const handleVisibility = () => {
      if (document.hidden) {
        behavioralRef.current.tabSwitches++;
      }
    };
    const handlePaste = () => {
      behavioralRef.current.copyPasteEvents++;
    };
    const handleKeyDown = () => {
      behavioralRef.current.keystrokeCount++;
    };

    document.addEventListener('visibilitychange', handleVisibility);
    document.addEventListener('paste', handlePaste);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('visibilitychange', handleVisibility);
      document.removeEventListener('paste', handlePaste);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const resetAnswerState = () => {
    setSelectedOption(null);
    setAnswerText('');
    setCode('');
    behavioralRef.current = {
      timeSpentSeconds: 0,
      copyPasteEvents: 0,
      tabSwitches: 0,
      keystrokeCount: 0,
      idleTimeSeconds: 0,
    };
    taskShownAt.current = Date.now();
  };

  const startTest = async () => {
    if (!spec) {
      message.warning('Выберите специализацию');
      return;
    }
    setSending(true);
    try {
      const endpoint = v2Mode ? '/testing/v2/start' : '/testing/start';
      const { data } = await api.post(endpoint, {
        specialization: spec,
        declaredGrade: declared,
      });
      setSession(data);
      setTask(data.firstTask || data.firstQuestion);
      setTaskNumber(1);
      setTotalTasks(data.totalTasks || data.totalQuestions || 15);
      setCurrentScore(0);
      setSecondsLeft((data.timeLimitMinutes || 90) * 60);
      resetAnswerState();
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setSending(false);
    }
  };

  const submitAnswer = async () => {
    if (!session || !task) return;

    // Validate based on task type
    const taskType = task.type || 'multiple_choice';
    if (taskType === 'multiple_choice' && !selectedOption) {
      message.warning('Выберите вариант ответа');
      return;
    }
    if (taskType === 'code_execution' && !code.trim()) {
      message.warning('Напишите код');
      return;
    }
    if ((taskType === 'llm_open' || taskType === 'system_design') && !answerText.trim()) {
      message.warning('Введите ответ');
      return;
    }

    // Calculate time spent
    behavioralRef.current.timeSpentSeconds = Math.round(
      (Date.now() - taskShownAt.current) / 1000,
    );

    setSending(true);
    try {
      const endpoint = v2Mode
        ? `/testing/v2/session/${session.sessionId}/answer`
        : `/testing/session/${session.sessionId}/answer`;

      const payload: any = {
        questionId: task.id,
        timeSpentSeconds: behavioralRef.current.timeSpentSeconds,
      };

      if (v2Mode) {
        payload.behavioral = behavioralRef.current;
      }

      switch (taskType) {
        case 'multiple_choice':
          payload.selectedOptionId = selectedOption;
          break;
        case 'code_execution':
          payload.code = code;
          break;
        case 'llm_open':
        case 'system_design':
          payload.answerText = answerText;
          payload.designDescription = answerText;
          break;
        case 'code_review':
          payload.answerText = answerText;
          payload.fixedCode = answerText;
          break;
      }

      const { data } = await api.post(endpoint, payload);

      // Show brief feedback
      if (data.correct !== undefined) {
        message.open({
          type: data.correct ? 'success' : 'info',
          content: data.correct ? '✓ Верно!' : data.explanation || 'Ответ принят',
          duration: 2,
        });
      }

      setCurrentScore(data.currentScore || 0);

      if (data.isFinished || data.tasksRemaining === 0) {
        // Complete the test
        const completeEndpoint = v2Mode
          ? `/testing/v2/session/${session.sessionId}/complete`
          : `/testing/session/${session.sessionId}/complete`;
        const { data: resultData } = await api.post(completeEndpoint);
        setResult(resultData);
      } else {
        setTask(data.nextTask || data.nextQuestion);
        setTaskNumber((n) => n + 1);
        resetAnswerState();
      }
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

  // ─── Result View ───
  if (result) {
    return (
      <AppShell requireRole="CANDIDATE">
        <div style={{ maxWidth: 800, margin: '0 auto' }}>
          <Result
            status={
              result.recommendation === 'PASS' || result.recommendation === 'UPGRADE'
                ? 'success'
                : 'error'
            }
            icon={
              result.recommendation === 'FAIL' ? '❌' : result.recommendation === 'UPGRADE' ? '🚀' : '✅'
            }
            title={
              result.recommendation === 'UPGRADE'
                ? 'Грейд повышен!'
                : result.recommendation === 'PASS'
                ? 'Тест пройден!'
                : 'Тест не пройден'
            }
            subTitle={
              result.recommendation === 'UPGRADE'
                ? `Ваш грейд повышен до ${result.gradeAfter}`
                : result.recommendation === 'PASS'
                ? `Ваш грейд подтверждён: ${result.gradeAfter}`
                : `Рекомендуемый грейд: ${result.gradeAfter}. Попробуйте пройти тест на грейд ниже.`
            }
          >
            <div style={{ display: 'flex', justifyContent: 'center', gap: 32, marginBottom: 24 }}>
              <Statistic title="Балл" value={result.score} suffix="/ 100" />
              <Statistic
                title="Грейд"
                value={result.gradeAfter}
                valueStyle={{ color: theme.colors.primary }}
              />
              <Statistic title="Theta" value={result.theta} precision={2} />
            </div>

            {/* Task Breakdown */}
            {result.taskBreakdown && Object.keys(result.taskBreakdown).length > 0 && (
              <Card
                style={{ marginBottom: 16, textAlign: 'left' }}
                title="Результаты по типам задач"
              >
                {Object.entries(result.taskBreakdown).map(([type, data]: [string, any]) => (
                  <div
                    key={type}
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      padding: '8px 0',
                      borderBottom: `1px solid ${theme.colors.borderLight}`,
                    }}
                  >
                    <Space>
                      <span>{TASK_TYPE_LABELS[type]?.icon || '📄'}</span>
                      <span>{TASK_TYPE_LABELS[type]?.label || type}</span>
                    </Space>
                    <Space>
                      <Tag color={data.passed === data.total ? 'green' : 'orange'}>
                        {data.passed}/{data.total}
                      </Tag>
                      <Text type="secondary">{data.avgScore}%</Text>
                    </Space>
                  </div>
                ))}
              </Card>
            )}

            {/* Anti-cheat */}
            {result.antiCheat && (
              <Card style={{ marginBottom: 16, textAlign: 'left' }} title="Анти-чит анализ">
                <Space direction="vertical" style={{ width: '100%' }}>
                  <div>
                    Статус:{' '}
                    <Tag color={result.antiCheat.isSuspicious ? 'red' : 'green'}>
                      {result.antiCheat.isSuspicious ? 'Подозрительно' : 'Честный проход'}
                    </Tag>
                  </div>
                  {result.antiCheat.flags?.length > 0 && (
                    <div>
                      Флаги: {result.antiCheat.flags.map((f: string) => (
                        <Tag key={f} color="orange" style={{ marginRight: 4 }}>
                          {f}
                        </Tag>
                      ))}
                    </div>
                  )}
                </Space>
              </Card>
            )}

            <Space>
              <Link href="/candidate">
                <Button type="primary">На дашборд</Button>
              </Link>
              <Link href="/candidate/profile">
                <Button>Мой профиль</Button>
              </Link>
            </Space>
          </Result>
        </div>
      </AppShell>
    );
  }

  // ─── Test Intro (no session yet) ───
  if (!session) {
    const cooldown = canRetakeAt ? new Date(canRetakeAt) : null;
    const cooldownActive = !!cooldown && cooldown.getTime() > Date.now();

    return (
      <AppShell requireRole="CANDIDATE">
        <div style={{ maxWidth: 600, margin: '0 auto' }}>
          <Card style={{ borderRadius: theme.radius.lg }}>
            <div style={{ textAlign: 'center', marginBottom: 32 }}>
              <div style={{ fontSize: 64, marginBottom: 16 }}>📝</div>
              <Title level={2} style={{ marginBottom: 8 }}>
                Тестирование
              </Title>
              <Paragraph type="secondary">
                Адаптивный тест определит ваш грейд. Задания подбираются на основе ваших ответов.
              </Paragraph>
            </div>

            {cooldownActive ? (
              <Alert
                type="warning"
                showIcon
                message={`Пересдача возможна ${cooldown!.toLocaleDateString('ru-RU')}`}
                style={{ marginBottom: 24 }}
              />
            ) : (
              <>
                <div style={{ marginBottom: 24 }}>
                  <div style={{ fontWeight: 600, marginBottom: 8 }}>Специализация:</div>
                  <Select
                    style={{ width: '100%' }}
                    placeholder="Выберите специализацию"
                    value={spec}
                    onChange={setSpec}
                    options={SPECIALIZATIONS}
                  />
                </div>

                <div style={{ marginBottom: 24 }}>
                  <div style={{ fontWeight: 600, marginBottom: 8 }}>Заявленный грейд:</div>
                  <Select
                    style={{ width: '100%' }}
                    placeholder="Выберите грейд"
                    value={declared}
                    onChange={setDeclared}
                    options={GRADES}
                  />
                </div>

                <div style={{ marginBottom: 24 }}>
                  <div style={{ fontWeight: 600, marginBottom: 8 }}>Режим теста:</div>
                  <Radio.Group
                    value={v2Mode}
                    onChange={(e) => setV2Mode(e.target.value)}
                    style={{ width: '100%' }}
                  >
                    <Radio.Button value={true} style={{ width: '100%', marginBottom: 8 }}>
                      V2 — Мультиформатный (код, вопросы, проектирование)
                    </Radio.Button>
                    <Radio.Button value={false} style={{ width: '100%' }}>
                      V1 — Классический (только вопросы)
                    </Radio.Button>
                  </Radio.Group>
                </div>

                <div
                  style={{
                    padding: 16,
                    background: theme.colors.bg,
                    borderRadius: theme.radius.md,
                    marginBottom: 24,
                  }}
                >
                  <div style={{ fontWeight: 600, marginBottom: 8 }}>Что вас ждёт:</div>
                  <ul style={{ margin: 0, paddingLeft: 20, color: theme.colors.textSecondary }}>
                    <li>15-20 заданий разного формата</li>
                    <li>Время: 60-90 минут</li>
                    <li>Результат определяет ваш грейд</li>
                    <li>Повторное прохождение — через 3 месяца</li>
                  </ul>
                </div>

                <Button
                  type="primary"
                  size="large"
                  block
                  loading={sending}
                  onClick={startTest}
                >
                  Начать тестирование
                </Button>
              </>
            )}
          </Card>
        </div>
      </AppShell>
    );
  }

  // ─── Active Test ───
  const taskType = task?.type || 'multiple_choice';
  const typeInfo = TASK_TYPE_LABELS[taskType] || TASK_TYPE_LABELS.multiple_choice;
  const progress = ((taskNumber - 1) / totalTasks) * 100;

  return (
    <AppShell requireRole="CANDIDATE">
      <div style={{ maxWidth: 800, margin: '0 auto' }}>
        {/* Header */}
        <Card
          style={{ marginBottom: 16, borderRadius: theme.radius.lg }}
          bodyStyle={{ padding: 16 }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
            <div>
              <Space>
                <Tag color={typeInfo.color}>
                  {typeInfo.icon} {typeInfo.label}
                </Tag>
                <Text strong>
                  Задача {taskNumber} из {totalTasks}
                </Text>
              </Space>
            </div>
            <Space size="large">
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  Текущий балл:
                </Text>{' '}
                <Text strong>{currentScore}</Text>
              </div>
              {secondsLeft != null && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    Осталось:
                  </Text>{' '}
                  <Text
                    strong
                    style={{
                      color: secondsLeft < 300 ? theme.colors.error : theme.colors.text,
                      fontFamily: 'monospace',
                    }}
                  >
                    {mmss(secondsLeft)}
                  </Text>
                </div>
              )}
            </Space>
          </div>
          <Progress
            percent={progress}
            showInfo={false}
            size="small"
            style={{ marginTop: 8 }}
          />
        </Card>

        {/* Task Content */}
        <Card style={{ borderRadius: theme.radius.lg, marginBottom: 16 }}>
          <Title level={4} style={{ marginBottom: 16 }}>
            {task?.title || `Задача ${taskNumber}`}
          </Title>
          <Paragraph style={{ fontSize: 15, whiteSpace: 'pre-wrap' }}>
            {task?.description || task?.question || ''}
          </Paragraph>

          {/* ─── Multiple Choice ─── */}
          {taskType === 'multiple_choice' && task?.multipleChoice && (
            <div style={{ marginTop: 24 }}>
              <Radio.Group
                value={selectedOption}
                onChange={(e) => setSelectedOption(e.target.value)}
                style={{ width: '100%' }}
              >
                {task.multipleChoice.options?.map((opt: any) => (
                  <Radio.Button
                    key={opt.id}
                    value={opt.id}
                    style={{
                      display: 'block',
                      width: '100%',
                      marginBottom: 8,
                      height: 'auto',
                      padding: '12px 16px',
                      whiteSpace: 'normal',
                    }}
                  >
                    {opt.text}
                  </Radio.Button>
                ))}
              </Radio.Group>
            </div>
          )}

          {/* ─── Code Execution ─── */}
          {taskType === 'code_execution' && task?.codeExecution && (
            <div style={{ marginTop: 24 }}>
              {task.codeExecution.starterCode && (
                <div
                  style={{
                    padding: 16,
                    background: '#1e1e1e',
                    borderRadius: theme.radius.md,
                    marginBottom: 16,
                    fontFamily: 'monospace',
                    fontSize: 13,
                    color: '#d4d4d4',
                    overflow: 'auto',
                  }}
                >
                  <pre style={{ margin: 0 }}>{task.codeExecution.starterCode}</pre>
                </div>
              )}
              <div
                style={{
                  border: `1px solid ${theme.colors.border}`,
                  borderRadius: theme.radius.md,
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    padding: '8px 12px',
                    background: theme.colors.bg,
                    borderBottom: `1px solid ${theme.colors.border}`,
                    fontSize: 12,
                    color: theme.colors.textSecondary,
                  }}
                >
                  Ваш код:
                </div>
                <textarea
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder="// Напишите решение здесь..."
                  style={{
                    width: '100%',
                    minHeight: 200,
                    padding: 16,
                    border: 'none',
                    outline: 'none',
                    resize: 'vertical',
                    fontFamily: 'monospace',
                    fontSize: 14,
                    background: '#1e1e1e',
                    color: '#d4d4d4',
                  }}
                />
              </div>
            </div>
          )}

          {/* ─── LLM Open / System Design ─── */}
          {(taskType === 'llm_open' || taskType === 'system_design') && (
            <div style={{ marginTop: 24 }}>
              <div
                style={{
                  border: `1px solid ${theme.colors.border}`,
                  borderRadius: theme.radius.md,
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    padding: '8px 12px',
                    background: theme.colors.bg,
                    borderBottom: `1px solid ${theme.colors.border}`,
                    fontSize: 12,
                    color: theme.colors.textSecondary,
                  }}
                >
                  Ваш ответ:
                </div>
                <textarea
                  value={answerText}
                  onChange={(e) => setAnswerText(e.target.value)}
                  placeholder={
                    taskType === 'system_design'
                      ? 'Опишите вашу архитектуру, компоненты, и компромиссы...'
                      : 'Разверните ответ, приведите примеры...'
                  }
                  style={{
                    width: '100%',
                    minHeight: 200,
                    padding: 16,
                    border: 'none',
                    outline: 'none',
                    resize: 'vertical',
                    fontSize: 14,
                    lineHeight: 1.6,
                  }}
                />
              </div>
              <div style={{ fontSize: 12, color: theme.colors.textMuted, marginTop: 8 }}>
                Оценка: точность, полнота, структурированность, примеры
              </div>
            </div>
          )}

          {/* ─── Code Review ─── */}
          {taskType === 'code_review' && task?.codeReview && (
            <div style={{ marginTop: 24 }}>
              <div
                style={{
                  padding: 16,
                  background: '#1e1e1e',
                  borderRadius: theme.radius.md,
                  marginBottom: 16,
                  fontFamily: 'monospace',
                  fontSize: 13,
                  color: '#d4d4d4',
                  overflow: 'auto',
                }}
              >
                <pre style={{ margin: 0 }}>{task.codeReview.buggyCode}</pre>
              </div>
              <div style={{ fontWeight: 600, marginBottom: 8 }}>
                Исправьте код и опишите найденные баги:
              </div>
              <textarea
                value={answerText}
                onChange={(e) => setAnswerText(e.target.value)}
                placeholder="1. Найденный баг: ...&#10;2. Исправление: ..."
                style={{
                  width: '100%',
                  minHeight: 150,
                  padding: 16,
                  border: `1px solid ${theme.colors.border}`,
                  borderRadius: theme.radius.md,
                  outline: 'none',
                  resize: 'vertical',
                  fontSize: 14,
                  lineHeight: 1.6,
                }}
              />
            </div>
          )}
        </Card>

        {/* Submit */}
        <div style={{ display: 'flex', gap: 12 }}>
          <Button
            type="primary"
            size="large"
            block
            loading={sending}
            onClick={submitAnswer}
          >
            Отправить ответ
          </Button>
        </div>

        {/* Anti-cheat notice */}
        <div
          style={{
            marginTop: 16,
            padding: 12,
            background: theme.colors.warningLight,
            borderRadius: theme.radius.md,
            fontSize: 12,
            color: theme.colors.textSecondary,
            textAlign: 'center',
          }}
        >
          ⚠️ Тест записывает время ответов, копирование и переключение вкладок для обеспечения честности
        </div>
      </div>
    </AppShell>
  );
}