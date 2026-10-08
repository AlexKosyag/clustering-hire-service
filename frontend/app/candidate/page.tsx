'use client';

import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Progress,
  Row,
  Space,
  Spin,
  Statistic,
  Tag,
  Typography,
} from 'antd';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { api, errMsg } from '@/lib/api';
import { dateRu, gradeColor, gradeLabel, specLabel } from '@/lib/meta';
import { theme, getGradeColor } from '@/lib/theme';
import ScoreRing from '@/components/ui/ScoreRing';
import GradeBadge from '@/components/ui/GradeBadge';
import TechTag from '@/components/ui/TechTag';
import LevelProgress from '@/components/ui/LevelProgress';
import StreakCounter from '@/components/ui/StreakCounter';

const { Text, Paragraph } = Typography;

interface Profile {
  id: string;
  firstName?: string;
  lastName?: string;
  specialization?: string;
  declaredGrade?: string;
  verifiedGrade?: string;
  stack?: string[];
  testStatus?: string;
  testScore?: number | null;
  lastTestDate?: string;
  testCooldownUntil?: string;
  fspId?: string;
  achievements?: Array<{ name: string; category: string; points: number; year?: number; place?: string }>;
  achievementPoints?: number;
  email?: string;
}

interface Gamification {
  level: number;
  levelName: string;
  xp: number;
  xpToNextLevel: number;
  xpProgress: number;
  streak: number;
  longestStreak: number;
  multiplier: number;
  badges: Array<{ id: string; name: string; icon: string; rarity: string; code: string; description: string; category: string }>;
}

export default function CandidateDashboardPage() {
  const { message } = App.useApp();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [gamification, setGamification] = useState<Gamification | null>(null);
  const [invitations, setInvitations] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      try {
        const [profileRes, invitationsRes, gamificationRes] = await Promise.allSettled([
          api.get('/candidates/profile'),
          api.get('/invitations'),
          api.get('/gamification/profile'),
        ]);

        if (profileRes.status === 'fulfilled') {
          setProfile(profileRes.value.data);
        }
        if (invitationsRes.status === 'fulfilled') {
          const items = invitationsRes.value.data.invitations || invitationsRes.value.data.items || [];
          setInvitations(items.slice(0, 3));
        }
        if (gamificationRes.status === 'fulfilled') {
          setGamification(gamificationRes.value.data);
        }
      } catch (error) {
        message.error(errMsg(error));
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  if (loading) {
    return (
      <AppShell requireRole="CANDIDATE">
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
        </div>
      </AppShell>
    );
  }

  if (!profile) return null;

  const cooldown = profile.testCooldownUntil ? new Date(profile.testCooldownUntil) : null;
  const cooldownActive = !!cooldown && cooldown.getTime() > Date.now();
  const testPassed = profile.testStatus === 'PASSED' || profile.testStatus === 'UPGRADED';
  const newInvitations = invitations.filter((i) => i.status === 'SENT').length;

  return (
    <AppShell requireRole="CANDIDATE">
      <div style={{ maxWidth: 1200, margin: '0 auto' }}>
        {/* Welcome Header */}
        <div
          style={{
            background: `linear-gradient(135deg, ${theme.colors.primary}, ${theme.colors.primaryDark})`,
            borderRadius: theme.radius.lg,
            padding: '32px',
            color: 'white',
            marginBottom: 24,
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 16 }}>
            <div>
              <h1 style={{ fontSize: 28, fontWeight: 700, margin: 0, color: 'white' }}>
                Привет, {profile.firstName || 'Кандидат'}! 👋
              </h1>
              <p style={{ marginTop: 8, opacity: 0.9, fontSize: 16 }}>
                {profile.specialization ? specLabel(profile.specialization) : 'Специализация не указана'} ·{' '}
                {profile.verifiedGrade ? gradeLabel(profile.verifiedGrade) : 'Грейд не подтверждён'}
              </p>
            </div>
            <div style={{ display: 'flex', gap: 32, alignItems: 'center' }}>
              {/* Level */}
              {gamification && (
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 32 }}>⭐</div>
                  <div style={{ fontSize: 24, fontWeight: 700 }}>{gamification.level}</div>
                  <div style={{ fontSize: 12, opacity: 0.8 }}>{gamification.levelName}</div>
                </div>
              )}
              {/* Streak */}
              {gamification && (
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 32 }}>
                    {gamification.streak >= 7 ? '🔥🔥' : gamification.streak >= 3 ? '🔥' : '💫'}
                  </div>
                  <div style={{ fontSize: 24, fontWeight: 700 }}>{gamification.streak}</div>
                  <div style={{ fontSize: 12, opacity: 0.8 }}>дней стрик</div>
                </div>
              )}
              {/* XP */}
              {gamification && (
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 32 }}>✨</div>
                  <div style={{ fontSize: 24, fontWeight: 700 }}>{gamification.xp}</div>
                  <div style={{ fontSize: 12, opacity: 0.8 }}>XP</div>
                </div>
              )}
            </div>
          </div>

          {/* Level Progress Bar */}
          {gamification && (
            <div style={{ marginTop: 20, maxWidth: 400 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, marginBottom: 4 }}>
                <span>Уровень {gamification.level}</span>
                <span>{gamification.xp} / {gamification.xpToNextLevel} XP</span>
              </div>
              <div
                style={{
                  height: 6,
                  background: 'rgba(255,255,255,0.2)',
                  borderRadius: 3,
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    width: `${gamification.xpProgress || 0}%`,
                    height: '100%',
                    background: 'white',
                    borderRadius: 3,
                    transition: 'width 0.5s ease',
                  }}
                />
              </div>
            </div>
          )}
        </div>

        {/* Stats Cards */}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={12} md={6}>
            <Card
              style={{ borderRadius: theme.radius.lg, border: `1px solid ${theme.colors.border}` }}
              bodyStyle={{ padding: 20 }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div
                  style={{
                    width: 48,
                    height: 48,
                    borderRadius: theme.radius.md,
                    background: testPassed ? theme.colors.successLight : theme.colors.borderLight,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 24,
                  }}
                >
                  {testPassed ? '✅' : '📝'}
                </div>
                <div>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted }}>Тестирование</div>
                  <div style={{ fontSize: 18, fontWeight: 700, color: theme.colors.text }}>
                    {testPassed ? `${profile.testScore ?? '—'} баллов` : 'Не пройдено'}
                  </div>
                  <div style={{ fontSize: 12, color: theme.colors.textSecondary }}>
                    {profile.lastTestDate ? dateRu(profile.lastTestDate) : 'Требуется пройти'}
                  </div>
                </div>
              </div>
            </Card>
          </Col>

          <Col xs={12} md={6}>
            <Card
              style={{ borderRadius: theme.radius.lg, border: `1px solid ${theme.colors.border}` }}
              bodyStyle={{ padding: 20 }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div
                  style={{
                    width: 48,
                    height: 48,
                    borderRadius: theme.radius.md,
                    background: theme.colors.infoLight,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 24,
                  }}
                >
                  📬
                </div>
                <div>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted }}>Приглашения</div>
                  <div style={{ fontSize: 18, fontWeight: 700, color: theme.colors.text }}>
                    {invitations.length}
                  </div>
                  <div style={{ fontSize: 12, color: newInvitations > 0 ? theme.colors.primary : theme.colors.textSecondary }}>
                    {newInvitations > 0 ? `${newInvitations} новых` : 'Пока пусто'}
                  </div>
                </div>
              </div>
            </Card>
          </Col>

          <Col xs={12} md={6}>
            <Card
              style={{ borderRadius: theme.radius.lg, border: `1px solid ${theme.colors.border}` }}
              bodyStyle={{ padding: 20 }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div
                  style={{
                    width: 48,
                    height: 48,
                    borderRadius: theme.radius.md,
                    background: theme.colors.warningLight,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 24,
                  }}
                >
                  🏆
                </div>
                <div>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted }}>Достижения ФСП</div>
                  <div style={{ fontSize: 18, fontWeight: 700, color: theme.colors.text }}>
                    {profile.achievements?.length || 0}
                  </div>
                  <div style={{ fontSize: 12, color: theme.colors.textSecondary }}>
                    {profile.achievementPoints || 0} очков
                  </div>
                </div>
              </div>
            </Card>
          </Col>

          <Col xs={12} md={6}>
            <Card
              style={{ borderRadius: theme.radius.lg, border: `1px solid ${theme.colors.border}` }}
              bodyStyle={{ padding: 20 }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div
                  style={{
                    width: 48,
                    height: 48,
                    borderRadius: theme.radius.md,
                    background: theme.colors.primaryBg,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 24,
                  }}
                >
                  🛠️
                </div>
                <div>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted }}>Стек</div>
                  <div style={{ fontSize: 18, fontWeight: 700, color: theme.colors.text }}>
                    {profile.stack?.length || 0}
                  </div>
                  <div style={{ fontSize: 12, color: theme.colors.textSecondary }}>
                    технологий
                  </div>
                </div>
              </div>
            </Card>
          </Col>
        </Row>

        {/* Main Content */}
        <Row gutter={[16, 16]}>
          {/* Left Column */}
          <Col xs={24} lg={16}>
            {/* Category Card */}
            <Card
              title={
                <Space>
                  <span>📦</span>
                  <span>Моя категория</span>
                </Space>
              }
              style={{ borderRadius: theme.radius.lg, marginBottom: 16 }}
              extra={
                <Link href="/candidate/profile">
                  <Button size="small">Редактировать профиль</Button>
                </Link>
              }
            >
              <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap' }}>
                <div>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted, marginBottom: 4 }}>
                    Специализация
                  </div>
                  {profile.specialization ? (
                    <Tag color="blue" style={{ fontSize: 14, padding: '4px 12px' }}>
                      {specLabel(profile.specialization)}
                    </Tag>
                  ) : (
                    <Text type="secondary">Не указана</Text>
                  )}
                </div>
                <div>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted, marginBottom: 4 }}>
                    Заявленный грейд
                  </div>
                  {profile.declaredGrade ? (
                    <GradeBadge grade={profile.declaredGrade} size="lg" />
                  ) : (
                    <Text type="secondary">Не указан</Text>
                  )}
                </div>
                <div>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted, marginBottom: 4 }}>
                    Подтверждённый грейд
                  </div>
                  {profile.verifiedGrade ? (
                    <GradeBadge grade={profile.verifiedGrade} verified size="lg" />
                  ) : (
                    <Text type="secondary">Не подтверждён</Text>
                  )}
                </div>
              </div>

              {profile.stack && profile.stack.length > 0 && (
                <div style={{ marginTop: 16 }}>
                  <div style={{ fontSize: 12, color: theme.colors.textMuted, marginBottom: 8 }}>
                    Технологический стек
                  </div>
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                    {profile.stack.map((tech) => (
                      <TechTag key={tech} tech={tech} />
                    ))}
                  </div>
                </div>
              )}
            </Card>

            {/* Recent Invitations */}
            <Card
              title={
                <Space>
                  <span>📬</span>
                  <span>Последние приглашения</span>
                </Space>
              }
              style={{ borderRadius: theme.radius.lg, marginBottom: 16 }}
              extra={
                <Link href="/candidate/invitations">
                  <Button size="small">Все приглашения</Button>
                </Link>
              }
            >
              {invitations.length === 0 ? (
                <div style={{ textAlign: 'center', padding: 24, color: theme.colors.textMuted }}>
                  Пока нет приглашений. Работодатели найдут вас через поиск!
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                  {invitations.map((inv, idx) => (
                    <div
                      key={idx}
                      style={{
                        padding: 16,
                        background: theme.colors.bg,
                        borderRadius: theme.radius.md,
                        border: `1px solid ${theme.colors.border}`,
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <div>
                          <div style={{ fontWeight: 600, color: theme.colors.text }}>
                            {inv.employer?.companyName || 'Компания'}
                          </div>
                          <div style={{ fontSize: 13, color: theme.colors.textSecondary, marginTop: 4 }}>
                            {inv.salaryFrom && inv.salaryTo
                              ? `${(inv.salaryFrom / 1000).toFixed(0)}K – ${(inv.salaryTo / 1000).toFixed(0)}K ₽`
                              : 'ЗП не указана'}
                          </div>
                        </div>
                        <Tag
                          color={
                            inv.status === 'ACCEPTED' ? 'green'
                            : inv.status === 'REJECTED' ? 'red'
                            : inv.status === 'VIEWED' ? 'blue'
                            : 'orange'
                          }
                        >
                          {inv.status === 'SENT' ? 'Новое' :
                           inv.status === 'VIEWED' ? 'Просмотрено' :
                           inv.status === 'ACCEPTED' ? 'Принято' :
                           inv.status === 'REJECTED' ? 'Отклонено' : inv.status}
                        </Tag>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </Card>

            {/* Achievements Preview */}
            {profile.achievements && profile.achievements.length > 0 && (
              <Card
                title={
                  <Space>
                    <span>🏆</span>
                    <span>Достижения ФСП</span>
                  </Space>
                }
                style={{ borderRadius: theme.radius.lg }}
                extra={
                  <span style={{ fontSize: 13, color: theme.colors.textMuted }}>
                    {profile.achievementPoints} очков
                  </span>
                }
              >
                <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                  {profile.achievements.slice(0, 6).map((ach, idx) => (
                    <div
                      key={idx}
                      style={{
                        padding: '12px 16px',
                        background: theme.colors.warningLight,
                        borderRadius: theme.radius.md,
                        border: `1px solid ${theme.colors.warning}30`,
                        minWidth: 150,
                      }}
                    >
                      <div style={{ fontSize: 20, marginBottom: 4 }}>
                        {ach.category === 'competition' ? '🏆' :
                         ach.category === 'certification' ? '📜' :
                         ach.category === 'event' ? '🎉' : '⭐'}
                      </div>
                      <div style={{ fontWeight: 600, fontSize: 13, color: theme.colors.text }}>
                        {ach.name}
                      </div>
                      <div style={{ fontSize: 12, color: theme.colors.textSecondary, marginTop: 2 }}>
                        {ach.year && `${ach.year} · `}
                        +{ach.points} очков
                      </div>
                    </div>
                  ))}
                </div>
              </Card>
            )}
          </Col>

          {/* Right Column */}
          <Col xs={24} lg={8}>
            {/* Test CTA */}
            <Card
              style={{
                borderRadius: theme.radius.lg,
                marginBottom: 16,
                background: testPassed
                  ? `linear-gradient(135deg, ${theme.colors.successLight}, ${theme.colors.surface})`
                  : theme.colors.surface,
                border: `1px solid ${testPassed ? theme.colors.success : theme.colors.border}`,
              }}
            >
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: 48, marginBottom: 12 }}>
                  {testPassed ? '✅' : '📝'}
                </div>
                <div style={{ fontSize: 18, fontWeight: 600, color: theme.colors.text, marginBottom: 8 }}>
                  {testPassed ? 'Тест пройден!' : 'Пройдите тестирование'}
                </div>
                {testPassed ? (
                  <>
                    <ScoreRing score={profile.testScore || 0} size={80} strokeWidth={6} />
                    <div style={{ marginTop: 12, color: theme.colors.textSecondary }}>
                      Подтверждённый грейд: <GradeBadge grade={profile.verifiedGrade} verified />
                    </div>
                  </>
                ) : (
                  <Paragraph type="secondary" style={{ marginBottom: 16 }}>
                    Пройдите адаптивный тест, чтобы подтвердить свой грейд и стать видимым для работодателей
                  </Paragraph>
                )}
                {cooldownActive ? (
                  <Alert
                    type="warning"
                    showIcon
                    message={`Пересдача ${cooldown!.toLocaleDateString('ru-RU')}`}
                    style={{ marginTop: 12 }}
                  />
                ) : (
                  <Link href="/candidate/test">
                    <Button
                      type="primary"
                      size="large"
                      block
                      style={{ marginTop: 12 }}
                    >
                      {testPassed ? 'Пройти заново' : 'Начать тестирование'}
                    </Button>
                  </Link>
                )}
              </div>
            </Card>

            {/* FSP Integration */}
            <Card
              title={
                <Space>
                  <span>🔗</span>
                  <span>ФСП интеграция</span>
                </Space>
              }
              style={{ borderRadius: theme.radius.lg, marginBottom: 16 }}
            >
              {profile.fspId ? (
                <div>
                  <Tag color="green" style={{ marginBottom: 12 }}>
                    ✓ Подключено
                  </Tag>
                  <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
                    ID: {profile.fspId}
                  </div>
                </div>
              ) : (
                <div>
                  <Paragraph type="secondary" style={{ fontSize: 13 }}>
                    Подключите ФСП ID, чтобы автоматически подтвердить ваши достижения на соревнованиях
                  </Paragraph>
                  <Link href="/candidate/profile">
                    <Button block>Подключить ФСП</Button>
                  </Link>
                </div>
              )}
            </Card>

            {/* Privacy Notice */}
            <Card
              style={{
                borderRadius: theme.radius.lg,
                background: theme.colors.infoLight,
                border: `1px solid ${theme.colors.info}30`,
              }}
              bodyStyle={{ padding: 16 }}
            >
              <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
                <span style={{ fontSize: 20 }}>🔒</span>
                <div>
                  <div style={{ fontWeight: 600, fontSize: 13, color: theme.colors.text, marginBottom: 4 }}>
                    Ваша приватность защищена
                  </div>
                  <div style={{ fontSize: 12, color: theme.colors.textSecondary }}>
                    Контакты видны работодателю только после того, как вы примете приглашение
                  </div>
                </div>
              </div>
            </Card>
          </Col>
        </Row>
      </div>
    </AppShell>
  );
}