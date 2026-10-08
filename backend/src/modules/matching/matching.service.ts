import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { EmployerNeed, Grade, Prisma, Specialization } from '@prisma/client';
import { createHash } from 'crypto';
import { RedisService } from '../../common/redis/redis.service';
import { PrismaService } from '../../prisma/prisma.module';
import { candidateInclude, CandidateWithRelations, toCandidatePublic, CandidatePublicDto } from './candidate-view';
import { EmbeddingService } from './embedding.service';
import { ExplanationService } from './explanation.service';
import { IndexingService } from './indexing.service';
import { EmployerNeedDto, SearchCandidatesDto, SemanticQueryDto } from './dto/matching.dto';
import { GRADE_ORDER, MatchingScoringService, Need } from './scoring.service';
import { VectorRepository } from './vector.repository';

export interface SearchResult {
  total: number;
  candidates: CandidatePublicDto[];
}

@Injectable()
export class MatchingService {
  private readonly logger = new Logger('Matching');

  constructor(
    private readonly prisma: PrismaService,
    private readonly embedding: EmbeddingService,
    private readonly vectors: VectorRepository,
    private readonly scoring: MatchingScoringService,
    private readonly explanation: ExplanationService,
    private readonly indexing: IndexingService,
    private readonly redis: RedisService,
    private readonly config: ConfigService,
  ) {}

  /** Ключевой эндпоинт: подбор кандидатов под потребность работодателя. */
  async search(dto: SearchCandidatesDto): Promise<SearchResult> {
    const limit = Math.min(Math.max(dto.limit ?? 20, 1), 100);
    const offset = Math.max(dto.offset ?? 0, 0);
    const cacheKey = this.cacheKey('search', dto, limit, offset);

    const cached = await this.redis.get(cacheKey);
    if (cached) {
      try {
        return JSON.parse(cached) as SearchResult;
      } catch {
        /* повреждённый кэш — пересчитываем */
      }
    }

    const started = Date.now();
    const pool = await this.prisma.candidateProfile.findMany({
      where: this.buildWhere(dto),
      include: candidateInclude,
      take: 3000,
    });

    const need: Need = {
      specialization: dto.specialization,
      grade: dto.grade,
      stack: dto.stack || [],
    };

    const queryText = [dto.description || '', (dto.stack || []).join(' ')].join(' ').trim();
    const needVector = await this.embedding.embed(queryText || this.defaultQueryText(need));
    const hits = await this.vectors.topBySimilarity(needVector, 3000);
    const similarityById = new Map(hits.map((h) => [h.id, h.similarity]));

    // Ленивая переиндексация кандидатов, у которых ещё нет вектора
    const unindexed = pool.filter((c) => !similarityById.has(c.id)).map((c) => c.id);
    if (unindexed.length > 0) {
      void this.indexing
        .indexBatch(unindexed.slice(0, 25))
        .catch((e) => this.logger.warn(`Ленивая индексация: ${(e as Error).message}`));
    }

    const scored = pool
      .map((candidate) => {
        const indexed = similarityById.has(candidate.id);
        const semantic = indexed ? Math.max(0, similarityById.get(candidate.id)!) : 0;
        const view = toCandidatePublic(candidate, indexed);
        const score = this.scoring.calculateMatch(
          {
            specialization: candidate.specialization,
            verifiedGrade: candidate.verifiedGrade,
            declaredGrade: candidate.declaredGrade,
            stack: candidate.stack || [],
            achievementPoints: view.achievementPoints,
          },
          need,
          semantic,
        );
        view.matchScore = score.total;
        view.matchComponents = score.components;
        view.matchExplanation = this.explanation.generate(
          {
            ...candidate,
            achievementPoints: view.achievementPoints,
            achievementNames: view.achievements.map((a) => a.name),
          },
          need,
          score,
        );
        return view;
      })
      .sort((a, b) => (b.matchScore ?? 0) - (a.matchScore ?? 0));

    const result: SearchResult = {
      total: scored.length,
      candidates: scored.slice(offset, offset + limit),
    };

    const ttl = this.config.get<number>('matching.cacheTtlSeconds') || 900;
    await this.redis.set(cacheKey, JSON.stringify(result), ttl);
    this.logger.log(`search: pool=${pool.length}, rendered=${result.candidates.length}, ${Date.now() - started}ms`);
    return result;
  }

  /** Чисто семантический поиск (без жёстких фильтров). */
  async semanticSearch(dto: SemanticQueryDto) {
    const vector = await this.embedding.embed(dto.query);
    const hits = await this.vectors.topBySimilarity(vector, Math.min((dto.limit || 10) * 5, 200));
    const ids = hits.slice(0, dto.limit || 10).map((h) => h.id);
    if (ids.length === 0) return { candidates: [] };

    const profiles = await this.prisma.candidateProfile.findMany({
      where: { id: { in: ids } },
      include: candidateInclude,
    });
    const byId = new Map<string, CandidateWithRelations>(
      profiles.map((p) => [p.id, p as unknown as CandidateWithRelations]),
    );
    const similarityById = new Map(hits.map((h) => [h.id, h.similarity]));

    const ordered: CandidateWithRelations[] = [];
    for (const id of ids) {
      const found = byId.get(id);
      if (found) ordered.push(found);
    }

    return {
      candidates: ordered.map((p) => {
        const view = toCandidatePublic(p, true);
        return {
          ...view,
          similarity: Number((similarityById.get(p.id) || 0).toFixed(4)),
          reason: p.bio ? p.bio.slice(0, 160) : `Стек: ${(p.stack || []).join(', ') || 'не указан'}`,
        };
      }),
    };
  }

  /** Рекомендации: collaborative filtering + новые кандидаты в категории работодателя. */
  async recommendations(employerId: string) {
    const employer = await this.prisma.employerProfile.findUnique({ where: { id: employerId } });
    const myInvitations = await this.prisma.invitation.findMany({
      where: { employerId },
      select: { candidateId: true },
    });
    const invitedByMe = new Set(myInvitations.map((i) => i.candidateId));

    const lastNeed = await this.prisma.employerNeed.findFirst({
      where: { employerId },
      orderBy: { createdAt: 'desc' },
    });
    const specialization: Specialization | undefined = lastNeed?.specialization ?? undefined;

    // 1. Кандидатов, которых приглашали похожие работодатели (та же индустрия)
    const similarEmployers = await this.prisma.employerProfile.findMany({
      where: {
        id: { not: employerId },
        ...(employer?.industry ? { industry: employer.industry } : {}),
      },
      select: { id: true },
      take: 20,
    });
    const peerInvitations = await this.prisma.invitation.findMany({
      where: { employerId: { in: similarEmployers.map((e) => e.id) } },
      orderBy: { createdAt: 'desc' },
      select: { candidateId: true },
      take: 100,
    });
    const peerIds = [...new Set(peerInvitations.map((i) => i.candidateId))]
      .filter((id) => !invitedByMe.has(id))
      .slice(0, 5);

    const peerCandidates = await this.prisma.candidateProfile.findMany({
      where: { id: { in: peerIds } },
      include: candidateInclude,
    });

    // 2. Новые кандидаты (последние 14 дней)
    const freshIds = await this.prisma.candidateProfile.findMany({
      where: {
        createdAt: { gte: new Date(Date.now() - 14 * 86_400_000) },
        ...(specialization ? { specialization } : {}),
        id: { notIn: peerIds },
      },
      orderBy: { createdAt: 'desc' },
      select: { id: true },
      take: 5,
    });
    const freshCandidates = await this.prisma.candidateProfile.findMany({
      where: { id: { in: freshIds.map((f) => f.id) } },
      include: candidateInclude,
    });

    return {
      recommendations: [
        {
          reason: 'Похожие работодатели приглашали этих кандидатов',
          candidates: peerCandidates.map((c) => toCandidatePublic(c)),
        },
        {
          reason: specialization
            ? `Новые кандидаты в категории ${specialization}`
            : 'Новые кандидаты на платформе',
          candidates: freshCandidates.map((c) => toCandidatePublic(c)),
        },
      ].filter((g) => g.candidates.length > 0),
    };
  }

  /** Ключевой эндпоинт работодателя: описать потребность и получить подборку. */
  async createNeed(employerId: string, dto: EmployerNeedDto) {
    const need: EmployerNeed = await this.prisma.employerNeed.create({
      data: {
        employerId,
        description: dto.description,
        specialization: dto.specialization,
        grade: dto.grade,
        stack: dto.stack || [],
      },
    });

    const vector = await this.embedding.embed(
      [dto.description, (dto.stack || []).join(' ')].join(' '),
    );
    await this.vectors.setNeedVector(need.id, vector);

    const search = await this.search({
      description: dto.description,
      specialization: dto.specialization,
      grade: dto.grade,
      stack: dto.stack || [],
      limit: 10,
      offset: 0,
    });

    return {
      id: need.id,
      description: need.description,
      specialization: need.specialization,
      grade: need.grade,
      stack: need.stack,
      candidatesCount: search.total,
      topCandidates: search.candidates,
    };
  }

  async invalidateCache(): Promise<void> {
    await this.redis.delByPattern('matching:cache:*');
  }

  /** Переиндексация одного кандидата (используется после смены профиля/теста/ФСП). */
  async reindexCandidate(candidateId: string): Promise<void> {
    await this.indexing.indexCandidate(candidateId);
  }

  private buildWhere(dto: SearchCandidatesDto): Prisma.CandidateProfileWhereInput {
    const where: Prisma.CandidateProfileWhereInput = {};
    if (dto.specialization) where.specialization = dto.specialization;
    if (dto.grade) {
      const idx = GRADE_ORDER.indexOf(dto.grade);
      const allowed = [GRADE_ORDER[idx - 1], GRADE_ORDER[idx], GRADE_ORDER[idx + 1]].filter(
        (g): g is Grade => !!g,
      );
      where.AND = [
        {
          OR: [
            { verifiedGrade: { in: allowed } },
            { verifiedGrade: null, declaredGrade: { in: allowed } },
          ],
        },
      ];
    }
    if (dto.hasAchievements) where.achievements = { some: {} };
    return where;
  }

  private defaultQueryText(need: Need): string {
    return [need.specialization, need.grade, ...(need.stack || [])].filter(Boolean).join(' ');
  }

  private cacheKey(prefix: string, payload: unknown, ...extra: unknown[]): string {
    const normalized = JSON.stringify(payload, Object.keys(payload as object).sort());
    const hash = createHash('sha1').update(normalized + JSON.stringify(extra)).digest('hex');
    return `matching:cache:${prefix}:${hash}`;
  }
}
