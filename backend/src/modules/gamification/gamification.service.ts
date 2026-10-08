import { Injectable, Logger } from '@nestjs/common';
import { PrismaService } from '../../prisma/prisma.module';

// ─────────────────────────── Types ───────────────────────────

export interface GamificationProfile {
  candidateId: string;
  xp: number;
  level: number;
  streak: number;
  longestStreak: number;
  lastActiveDate: Date | null;
  multiplier: number;
  badges: Badge[];
  levelName: string;
  xpToNextLevel: number;
  xpProgress: number; // 0-100
}

export interface Badge {
  id: string;
  code: string;
  name: string;
  description: string;
  icon: string;
  category: string;
  rarity: string;
  xpReward?: number;
  earnedAt?: Date;
}

export interface XpEvent {
  id: string;
  action: string;
  xp: number;
  metadata?: Record<string, unknown>;
  createdAt: Date;
}

export interface LeaderboardEntry {
  rank: number;
  candidateId: string;
  displayName: string;
  xp: number;
  level: number;
  badges: number;
}

// ─────────────────────────── Constants ───────────────────────────

export const XP_REWARDS: Record<string, number> = {
  TEST_COMPLETED: 100,
  TEST_PASSED: 200,
  TEST_UPGRADED: 300,
  PROFILE_COMPLETE: 50,
  DAILY_TASK_DONE: 25,
  INVITATION_RECEIVED: 10,
  INVITATION_ACCEPTED: 50,
  FSP_CONNECTED: 100,
  FIRST_LOGIN: 10,
  STREAK_3_DAYS: 50,
  STREAK_7_DAYS: 150,
  STREAK_30_DAYS: 500,
};

export const LEVEL_THRESHOLDS: Array<{ level: number; xpRequired: number; name: string }> = [
  { level: 1, xpRequired: 0, name: 'Новичок' },
  { level: 2, xpRequired: 100, name: 'Ученик' },
  { level: 3, xpRequired: 300, name: 'Практик' },
  { level: 4, xpRequired: 600, name: 'Специалист' },
  { level: 5, xpRequired: 1000, name: 'Опытный' },
  { level: 6, xpRequired: 1500, name: 'Мастер' },
  { level: 7, xpRequired: 2200, name: 'Эксперт' },
  { level: 8, xpRequired: 3000, name: 'Профи' },
  { level: 9, xpRequired: 4000, name: 'Ветеран' },
  { level: 10, xpRequired: 5500, name: 'Легенда' },
];

export const BADGE_DEFINITIONS: Array<Omit<Badge, 'id' | 'earnedAt'>> = [
  // Testing badges
  { code: 'FIRST_TEST', name: 'Первый тест', description: 'Прошёл первое тестирование', icon: '🎯', category: 'testing', rarity: 'common', xpReward: 50 },
  { code: 'TEST_PASSED', name: 'Тест пройден', description: 'Успешно прошёл тестирование', icon: '✅', category: 'testing', rarity: 'common', xpReward: 100 },
  { code: 'GRADE_JUNIOR', name: 'Junior', description: 'Подтвердил грейд Junior', icon: '🥉', category: 'testing', rarity: 'common', xpReward: 50 },
  { code: 'GRADE_MIDDLE', name: 'Middle', description: 'Подтвердил грейд Middle', icon: '🥈', category: 'testing', rarity: 'rare', xpReward: 150 },
  { code: 'GRADE_MIDDLE_PLUS', name: 'Middle+', description: 'Подтвердил грейд Middle+', icon: '🥈', category: 'testing', rarity: 'rare', xpReward: 200 },
  { code: 'GRADE_SENIOR', name: 'Senior', description: 'Подтвердил грейд Senior', icon: '🥇', category: 'testing', rarity: 'epic', xpReward: 300 },
  { code: 'GRADE_LEAD', name: 'Lead', description: 'Подтвердил грейд Lead', icon: '👑', category: 'testing', rarity: 'epic', xpReward: 500 },
  { code: 'GRADE_PRINCIPAL', name: 'Principal', description: 'Подтвердил грейд Principal', icon: '🏆', category: 'testing', rarity: 'legendary', xpReward: 1000 },

  // Profile badges
  { code: 'PROFILE_COMPLETE', name: 'Полный профиль', description: 'Заполнил все поля профиля', icon: '📋', category: 'profile', rarity: 'common', xpReward: 50 },
  { code: 'STACK_MASTER', name: 'Мастер стека', description: 'Указал 5+ технологий', icon: '🛠️', category: 'profile', rarity: 'common', xpReward: 30 },
  { code: 'BIO_WRITER', name: 'Автор', description: 'Написал описание о себе', icon: '✍️', category: 'profile', rarity: 'common', xpReward: 20 },

  // FSP badges
  { code: 'FSP_CONNECTED', name: 'Олимпиец', description: 'Подключил ФСП ID', icon: '🔗', category: 'fsp', rarity: 'rare', xpReward: 100 },
  { code: 'FSP_ACHIEVEMENT', name: 'Чемпион', description: 'Получил достижение ФСП', icon: '🏆', category: 'fsp', rarity: 'rare', xpReward: 150 },

  // Activity badges
  { code: 'STREAK_3', name: 'Разогрев', description: '3 дня активности подряд', icon: '🔥', category: 'activity', rarity: 'common', xpReward: 50 },
  { code: 'STREAK_7', name: 'Неделя силы', description: '7 дней активности подряд', icon: '🔥', category: 'activity', rarity: 'rare', xpReward: 150 },
  { code: 'STREAK_30', name: 'Марафонец', description: '30 дней активности подряд', icon: '🔥', category: 'activity', rarity: 'epic', xpReward: 500 },

  // Invitation badges
  { code: 'FIRST_INVITATION', name: 'Востребованный', description: 'Получил первое приглашение', icon: '📬', category: 'activity', rarity: 'common', xpReward: 30 },
  { code: 'INVITATION_ACCEPTED', name: 'Выбранный', description: 'Принял приглашение', icon: '🤝', category: 'activity', rarity: 'rare', xpReward: 100 },
];

// ─────────────────────────── Service ───────────────────────────

@Injectable()
export class GamificationService {
  private readonly logger = new Logger('Gamification');

  constructor(private readonly prisma: PrismaService) {}

  /**
   * Get or create gamification profile for a candidate.
   */
  async getProfile(candidateId: string): Promise<GamificationProfile> {
    let profile = await this.prisma.gamificationProfile.findUnique({
      where: { candidateId },
      include: {
        badges: { include: { badge: true } },
      },
    });

    if (!profile) {
      profile = await this.prisma.gamificationProfile.create({
        data: { candidateId },
        include: {
          badges: { include: { badge: true } },
        },
      });
    }

    return this.toGamificationProfile(profile);
  }

  /**
   * Award XP for an action.
   */
  async awardXp(
    candidateId: string,
    action: string,
    metadata?: Record<string, unknown>,
  ): Promise<{ xpAwarded: number; totalXp: number; levelUp: boolean; newLevel: number }> {
    const baseXp = XP_REWARDS[action] || 0;
    if (baseXp === 0) {
      return { xpAwarded: 0, totalXp: 0, levelUp: false, newLevel: 1 };
    }

    const profile = await this.getOrCreateProfile(candidateId);
    const streakMultiplier = profile.streak >= 7 ? 2.0 : profile.streak >= 3 ? 1.5 : 1.0;
    const xpAwarded = Math.round(baseXp * streakMultiplier);

    // Update profile
    const newXp = profile.xp + xpAwarded;
    const newLevel = this.calculateLevel(newXp);
    const levelUp = newLevel > profile.level;

    await this.prisma.gamificationProfile.update({
      where: { candidateId },
      data: {
        xp: newXp,
        level: newLevel,
      },
    });

    // Log event
    await this.prisma.xpEvent.create({
      data: {
        candidateId,
        action,
        xp: xpAwarded,
        metadata: metadata as any,
      },
    });

    this.logger.log(
      `Awarded ${xpAwarded} XP to ${candidateId} for ${action} (total: ${newXp}, level: ${newLevel})`,
    );

    return { xpAwarded, totalXp: newXp, levelUp, newLevel };
  }

  /**
   * Update streak (call on daily activity).
   */
  async updateStreak(candidateId: string): Promise<{ streak: number; multiplier: number }> {
    const profile = await this.getOrCreateProfile(candidateId);
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());

    if (profile.lastActiveDate) {
      const lastActive = new Date(profile.lastActiveDate);
      const lastActiveDay = new Date(lastActive.getFullYear(), lastActive.getMonth(), lastActive.getDate());
      const diffDays = Math.floor((today.getTime() - lastActiveDay.getTime()) / 86_400_000);

      if (diffDays === 0) {
        // Already active today, no change
        return { streak: profile.streak, multiplier: profile.multiplier };
      } else if (diffDays === 1) {
        // Consecutive day
        const newStreak = profile.streak + 1;
        const newMultiplier = newStreak >= 7 ? 2.0 : newStreak >= 3 ? 1.5 : 1.0;

        await this.prisma.gamificationProfile.update({
          where: { candidateId },
          data: {
            streak: newStreak,
            longestStreak: Math.max(newStreak, profile.longestStreak),
            lastActiveDate: now,
            multiplier: newMultiplier,
          },
        });

        // Check streak badges
        await this.checkStreakBadges(candidateId, newStreak);

        return { streak: newStreak, multiplier: newMultiplier };
      } else {
        // Streak broken
        await this.prisma.gamificationProfile.update({
          where: { candidateId },
          data: {
            streak: 1,
            lastActiveDate: now,
            multiplier: 1.0,
          },
        });

        return { streak: 1, multiplier: 1.0 };
      }
    } else {
      // First activity
      await this.prisma.gamificationProfile.update({
        where: { candidateId },
        data: {
          streak: 1,
          lastActiveDate: now,
          multiplier: 1.0,
        },
      });

      return { streak: 1, multiplier: 1.0 };
    }
  }

  /**
   * Award a badge to a candidate.
   */
  async awardBadge(candidateId: string, badgeCode: string): Promise<boolean> {
    const badge = await this.prisma.badge.findUnique({
      where: { code: badgeCode },
    });
    if (!badge) return false;

    // Check if already earned
    const existing = await this.prisma.candidateBadge.findUnique({
      where: {
        candidateId_badgeId: { candidateId, badgeId: badge.id },
      },
    });
    if (existing) return false;

    // Award badge
    await this.prisma.candidateBadge.create({
      data: { candidateId, badgeId: badge.id },
    });

    // Award XP for badge
    if (badge.xpReward > 0) {
      await this.awardXp(candidateId, `BADGE_${badge.code}`, { badgeName: badge.name });
    }

    this.logger.log(`Awarded badge ${badgeCode} to ${candidateId}`);
    return true;
  }

  /**
   * Get all badges with earned status.
   */
  async getBadges(candidateId: string): Promise<Badge[]> {
    const allBadges = await this.prisma.badge.findMany({
      orderBy: { category: 'asc' },
    });

    const earned = await this.prisma.candidateBadge.findMany({
      where: { candidateId },
      include: { badge: true },
    });

    const earnedMap = new Map(earned.map((e) => [e.badgeId, e.earnedAt]));

    return allBadges.map((badge) => ({
      ...badge,
      earnedAt: earnedMap.get(badge.id),
    }));
  }

  /**
   * Get leaderboard.
   */
  async getLeaderboard(
    type: 'global' | 'specialization' = 'global',
    specialization?: string,
    limit = 20,
  ): Promise<LeaderboardEntry[]> {
    const where = type === 'specialization' && specialization
      ? { candidate: { specialization: specialization as any } }
      : {};

    const profiles = await this.prisma.gamificationProfile.findMany({
      where,
      include: {
        candidate: {
          include: {
            user: { select: { email: true } },
          },
        },
        badges: { select: { id: true } },
      },
      orderBy: { xp: 'desc' },
      take: limit,
    });

    return profiles.map((p, idx) => ({
      rank: idx + 1,
      candidateId: p.candidateId,
      displayName: p.candidate.firstName
        ? `${p.candidate.firstName} ${p.candidate.lastName?.charAt(0) || ''}.`
        : `Кандидат #${p.candidateId.slice(0, 4)}`,
      xp: p.xp,
      level: p.level,
      badges: p.badges?.length || 0,
    }));
  }

  /**
   * Get XP history.
   */
  async getXpHistory(candidateId: string, limit = 50): Promise<XpEvent[]> {
    const events = await this.prisma.xpEvent.findMany({
      where: { candidateId },
      orderBy: { createdAt: 'desc' },
      take: limit,
    });

    return events.map((e) => ({
      id: e.id,
      action: e.action,
      xp: e.xp,
      metadata: (e.metadata as Record<string, unknown>) || undefined,
      createdAt: e.createdAt,
    }));
  }

  // ─────────────────────────── Private Helpers ───────────────────────────

  private async getOrCreateProfile(candidateId: string) {
    let profile = await this.prisma.gamificationProfile.findUnique({
      where: { candidateId },
    });
    if (!profile) {
      profile = await this.prisma.gamificationProfile.create({
        data: { candidateId },
      });
    }
    return profile;
  }

  private calculateLevel(xp: number): number {
    let level = 1;
    for (const threshold of LEVEL_THRESHOLDS) {
      if (xp >= threshold.xpRequired) {
        level = threshold.level;
      }
    }
    return level;
  }

  private getLevelInfo(level: number): { name: string; xpRequired: number } {
    const threshold = LEVEL_THRESHOLDS.find((t) => t.level === level) || LEVEL_THRESHOLDS[0];
    return { name: threshold.name, xpRequired: threshold.xpRequired };
  }

  private toGamificationProfile(profile: any): GamificationProfile {
    const levelInfo = this.getLevelInfo(profile.level);
    const nextLevel = LEVEL_THRESHOLDS.find((t) => t.level === profile.level + 1);
    const xpToNextLevel = nextLevel ? nextLevel.xpRequired - profile.xpRequired : 0;
    const xpProgress = xpToNextLevel > 0
      ? Math.min(100, ((profile.xp - levelInfo.xpRequired) / xpToNextLevel) * 100)
      : 100;

    return {
      candidateId: profile.candidateId,
      xp: profile.xp,
      level: profile.level,
      streak: profile.streak,
      longestStreak: profile.longestStreak,
      lastActiveDate: profile.lastActiveDate,
      multiplier: profile.multiplier,
      levelName: levelInfo.name,
      xpToNextLevel: nextLevel?.xpRequired || profile.xp,
      xpProgress: Math.round(xpProgress),
      badges: (profile.badges || []).map((bb: any) => ({
        id: bb.badge.id,
        code: bb.badge.code,
        name: bb.badge.name,
        description: bb.badge.description,
        icon: bb.badge.icon,
        category: bb.badge.category,
        rarity: bb.badge.rarity,
        earnedAt: bb.earnedAt,
      })),
    };
  }

  private async checkStreakBadges(candidateId: string, streak: number): Promise<void> {
    if (streak >= 3) await this.awardBadge(candidateId, 'STREAK_3');
    if (streak >= 7) await this.awardBadge(candidateId, 'STREAK_7');
    if (streak >= 30) await this.awardBadge(candidateId, 'STREAK_30');
  }

  /**
   * Seed badges into database.
   */
  async seedBadges(): Promise<number> {
    let count = 0;
    for (const badgeDef of BADGE_DEFINITIONS) {
      const existing = await this.prisma.badge.findUnique({
        where: { code: badgeDef.code },
      });
      if (!existing) {
        await this.prisma.badge.create({ data: badgeDef });
        count++;
      }
    }
    this.logger.log(`Seeded ${count} badges`);
    return count;
  }
}