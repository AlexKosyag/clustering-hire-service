import { BadRequestException, Injectable, NotFoundException } from '@nestjs/common';
import { Grade, Specialization, TestStatus } from '@prisma/client';
import { candidateInclude, toCandidatePublic } from '../matching/candidate-view';
import { MatchingService } from '../matching/matching.service';
import { PrismaService } from '../../prisma/prisma.module';

export interface FspAchievementDto {
  name: string;
  description?: string | null;
  category: string;
  year?: number | null;
  place?: string | null;
  points?: number;
}

/**
 * FSP Integration (MVP — mock).
 * Имитирует внешний сервис ФСП: по FSP ID возвращает список достижений
 * (конкурсы, олимпиады, сертификаты) из локального справочника.
 */
@Injectable()
export class FspService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly matching: MatchingService,
  ) {}

  /** Подключение аккаунта ФСП. */
  async connect(userId: string, fspId: string) {
    const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
    if (!profile) throw new NotFoundException('Профиль кандидата не найден');

    const normalized = fspId.trim();
    if (!/^[A-Za-z0-9_-]{4,40}$/.test(normalized)) {
      throw new BadRequestException('Некорректный формат FSP ID (4-40 символов: буквы, цифры, -, _)');
    }

    const taken = await this.prisma.candidateProfile.findUnique({ where: { fspId: normalized } });
    if (taken && taken.id !== profile.id) {
      throw new BadRequestException('Этот FSP ID уже привязан к другому аккаунту');
    }

    const achievements = await this.mockFetchAchievements(normalized);

    const linked = await this.prisma.$transaction(async (tx) => {
      await tx.candidateProfile.update({
        where: { id: profile.id },
        data: { fspId: normalized, fspConnectedAt: new Date() },
      });

      const catalog = await tx.fSPAchievement.findMany();
      const byKey = new Map(catalog.map((a) => [a.name, a.id]));

      for (const achievement of achievements) {
        const id = byKey.get(achievement.name);
        if (!id) continue;
        await tx.candidateAchievement.upsert({
          where: { candidateId_achievementId: { candidateId: profile.id, achievementId: id } },
          create: { candidateId: profile.id, achievementId: id },
          update: {},
        });
      }
      return true;
    });

    await this.matching.invalidateCache().catch(() => undefined);
    await this.matching.reindexCandidate(profile.id).catch(() => undefined);

    return {
      fspId: normalized,
      connected: linked,
      achievementsFound: achievements.length,
      achievements: achievements,
      note: 'Mock-интеграция: достижения сгенерированы детерминированно по FSP ID',
    };
  }

  async disconnect(userId: string) {
    const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
    if (!profile) throw new NotFoundException('Профиль кандидата не найден');

    await this.prisma.candidateProfile.update({
      where: { id: profile.id },
      data: { fspId: null, fspConnectedAt: null },
    });
    await this.prisma.candidateAchievement.deleteMany({ where: { candidateId: profile.id } });
    await this.matching.invalidateCache().catch(() => undefined);

    return { fspId: null, disconnected: true };
  }

  async achievements(userId: string) {
    const profile = await this.prisma.candidateProfile.findUnique({
      where: { userId },
      include: candidateInclude,
    });
    if (!profile) throw new NotFoundException('Профиль кандидата не найден');
    return {
      fspId: profile.fspId,
      fspConnectedAt: profile.fspConnectedAt,
      achievements: profile.achievements.map((a) => ({
        ...a.achievement,
        verifiedAt: a.verifiedAt,
      })),
      points: profile.achievements.reduce((sum, a) => sum + a.achievement.points, 0),
    };
  }

  /** Справочник достижений (mock-каталог, заполняется seed-ом). */
  async catalog() {
    const items = await this.prisma.fSPAchievement.findMany({ orderBy: [{ category: 'asc' }, { points: 'desc' }] });
    return { total: items.length, achievements: items };
  }

  /** Детерминированный «запрос» к внешнему сервису ФСП (mock). */
  async mockFetchAchievements(fspId: string): Promise<FspAchievementDto[]> {
    const catalog = await this.prisma.fSPAchievement.findMany({ orderBy: { points: 'desc' } });
    if (catalog.length === 0) return [];

    let hash = 2166136261;
    for (let i = 0; i < fspId.length; i++) {
      hash ^= fspId.charCodeAt(i);
      hash = Math.imul(hash, 16777619);
    }
    let seed = hash >>> 0;
    const random = () => {
      seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
      return seed / 4294967296;
    };

    const count = 2 + Math.floor(random() * 3); // 2-4 достижения
    const picked = new Set<number>();
    while (picked.size < Math.min(count, catalog.length)) {
      picked.add(Math.floor(random() * catalog.length));
    }

    return [...picked].map((i) => ({
      name: catalog[i].name,
      description: catalog[i].description,
      category: catalog[i].category,
      year: catalog[i].year ?? 2024,
      place: catalog[i].place,
      points: catalog[i].points,
    }));
  }
}
