import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { EmployerNeed, Grade, Prisma, Specialization } from '@prisma/client';
import { createHash } from 'crypto';
import { RedisService } from '../../common/redis/redis.service';
import { PrismaService } from '../../prisma/prisma.module';
import {
  candidateInclude,
  CandidateWithRelations,
  toCandidatePublic,
  CandidatePublicDto,
} from './candidate-view';
import { EmbeddingService } from './embedding.service';
import { ExplanationService } from './explanation.service';
import { IndexingService } from './indexing.service';
import { EmployerNeedDto, SearchCandidatesDto, SemanticQueryDto } from './dto/matching.dto';
import { GRADE_ORDER } from './scoring.service';
import { MatchingScoringServiceV2, MatchScore as MatchScoreV2 } from './scoring-v2.service';
import { MlServiceClient, QueryUnderstanding } from './ml-client.service';
import { QueryUnderstandingService } from './query-understanding.service';
import { VectorRepository } from './vector.repository';

// ─────────────────────────── V2 Types ───────────────────────────

export interface CandidateMatchV2 extends CandidatePublicDto {
  // V2 enhancements
  matchScoreV2: number;
  matchBreakdown: Record<string, { score: number; weight: number; contribution: number }>;
  weightsSource: string;
  semanticSimilarity: number;
  rerankerScore: number | null;
  retrievalRank: number;
  finalRank: number;
}

export interface SearchResultV2 {
  total: number;
  candidates: CandidateMatchV2[];
  queryUnderstanding: {
    originalQuery: string;
    parsedSpecialization: string | null;
    parsedSeniority: string | null;
    parsedSkills: string[];
    embeddingProvider: string;
  };
  timing: {
    retrieval: number;
    scoring: number;
    reranking: number;
    total: number;
  };
}

// ─────────────────────────── Service ───────────────────────────

/**
 * Matching Engine V2.
 *
 * Key improvements over V1:
 * 1. ML-powered query understanding (NER, intent, skills extraction)
 * 2. Expanded scoring (7 features vs 5)
 * 3. Importance-weighted stack matching
 * 4. Cross-encoder re-ranking (optional)
 * 5. Query understanding transparency
 * 6. Detailed timing metrics
 */
@Injectable()
export class MatchingServiceV2 {
  private readonly logger = new Logger('MatchingV2');

  constructor(
    private readonly prisma: PrismaService,
    private readonly embedding: EmbeddingService,
    private readonly vectors: VectorRepository,
    private readonly scoring: MatchingScoringServiceV2,
    private readonly explanation: ExplanationService,
    private readonly indexing: IndexingService,
    private readonly mlClient: MlServiceClient,
    private readonly queryUnderstanding: QueryUnderstandingService,
    private readonly redis: RedisService,
    private readonly config: ConfigService,
  ) {}

  /**
   * V2: Main search endpoint with ML-powered query understanding.
   */
  async search(dto: SearchCandidatesDto): Promise<SearchResultV2> {
    const limit = Math.min(Math.max(dto.limit ?? 20, 1), 100);
    const offset = Math.max(dto.offset ?? 0, 0);
    const totalStart = Date.now();

    // Check cache
    const cacheKey = this.cacheKey('search-v2', dto, limit, offset);
    const cached = await this.redis.get(cacheKey);
    if (cached) {
      try {
        return JSON.parse(cached) as SearchResultV2;
      } catch {
        /* corrupted cache — recompute */
      }
    }

    // ─── Step 1: Query Understanding ───
    const queryStart = Date.now();
    const queryText = [dto.description || '', (dto.stack || []).join(' ')]
      .join(' ')
      .trim();

    const parsedNeed = await this.queryUnderstanding.parseNeed(
      queryText || this.defaultQueryText(dto),
      dto.specialization,
      dto.grade,
      dto.stack,
    );
    const queryTime = Date.now() - queryStart;

    // ─── Step 2: Retrieval ───
    const retrievalStart = Date.now();

    // Hard filters (Prisma)
    const pool = await this.prisma.candidateProfile.findMany({
      where: this.buildWhere(dto),
      include: candidateInclude,
      take: 3000,
    });

    // Semantic retrieval via vector similarity
    const needVector = parsedNeed.embedding;
    const hits = await this.vectors.topBySimilarity(needVector, 3000);
    const similarityById = new Map(hits.map((h) => [h.id, h.similarity]));

    // Lazy re-indexing for unindexed candidates
    const unindexed = pool
      .filter((c) => !similarityById.has(c.id))
      .map((c) => c.id);
    if (unindexed.length > 0) {
      void this.indexing
        .indexBatch(unindexed.slice(0, 25))
        .catch((e) =>
          this.logger.warn(`Lazy indexing: ${(e as Error).message}`),
        );
    }

    const retrievalTime = Date.now() - retrievalStart;

    // ─── Step 3: Scoring ───
    const scoringStart = Date.now();

    const need = {
      specialization: parsedNeed.specialization || dto.specialization,
      grade: (parsedNeed.grade || dto.grade) as Grade | null | undefined,
      stack: parsedNeed.requiredSkills.length > 0
        ? parsedNeed.requiredSkills
        : dto.stack || [],
      requiredSkills: parsedNeed.requiredSkills,
      preferredSkills: parsedNeed.preferredSkills,
    };

    const scoredCandidates: Array<{
      view: CandidateMatchV2;
      candidate: CandidateWithRelations;
      semantic: number;
    }> = [];

    for (const candidate of pool) {
      const indexed = similarityById.has(candidate.id);
      const semantic = indexed
        ? Math.max(0, similarityById.get(candidate.id)!)
        : 0;

      const view = toCandidatePublic(candidate, indexed) as CandidateMatchV2;
      const candidateText = this.queryUnderstanding.buildCandidateText(candidate);

      const score = await this.scoring.calculateMatch(
        {
          specialization: candidate.specialization,
          verifiedGrade: candidate.verifiedGrade,
          declaredGrade: candidate.declaredGrade,
          stack: candidate.stack || [],
          achievementPoints: view.achievementPoints,
          testScore: candidate.lastTestScore,
          experienceYears: candidate.experienceYears,
        },
        need,
        semantic,
        candidateText,
        queryText,
      );

      view.matchScoreV2 = score.total;
      view.matchScore = score.total;  // For backward compatibility
      view.matchBreakdown = {};
      view.weightsSource = score.weightsSource;
      view.semanticSimilarity = semantic;
      view.rerankerScore = null;
      view.retrievalRank = 0;
      view.finalRank = 0;

      // Build breakdown from contributions
      for (const [key, contribution] of Object.entries(score.contributions)) {
        view.matchBreakdown[key] = {
          score: score.components[key as keyof typeof score.components] || 0,
          weight: this.getWeight(key),
          contribution,
        };
      }

      // Build explanation (using V1 explanation for now, enhanced in future)
      view.matchExplanation = this.explanation.generate(
        {
          ...candidate,
          achievementPoints: view.achievementPoints,
          achievementNames: view.achievements.map((a) => a.name),
        },
        need,
        {
          total: score.total,
          components: {
            categoryMatch: score.components.specializationMatch,
            stackOverlap: score.components.stackOverlap,
            semanticSimilarity: score.components.semanticSimilarity,
            gradeMatch: score.components.gradeMatch,
            achievementBoost: score.components.achievementBoost,
          },
        },
      );

      scoredCandidates.push({ view, candidate, semantic });
    }

    const scoringTime = Date.now() - scoringStart;

    // ─── Step 4: Sort by score ───
    scoredCandidates.sort((a, b) => (b.view.matchScoreV2 || 0) - (a.view.matchScoreV2 || 0));

    // Assign retrieval ranks
    scoredCandidates.forEach((item, idx) => {
      item.view.retrievalRank = idx + 1;
    });

    // ─── Step 5: Optional re-ranking (top candidates only) ───
    const rerankingStart = Date.now();
    const rerankTopK = Math.min(limit + 20, 50);  // Re-rank a bit more than needed
    const topCandidates = scoredCandidates.slice(0, rerankTopK);

    let rerankingTime = 0;
    if (queryText && topCandidates.length > 1 && this.mlClient.available) {
      try {
        const documents = topCandidates.map((item) =>
          this.queryUnderstanding.buildCandidateText(item.candidate),
        );

        const rerankResult = await this.mlClient.rerank(queryText, documents, rerankTopK);

        if (rerankResult.available) {
          // Apply re-ranking scores
          for (const result of rerankResult.results) {
            const item = topCandidates[result.index];
            if (item) {
              // Normalize cross-encoder score (typically -10 to 10) to 0-100
              const normalizedScore = Math.max(0, Math.min(100, (result.score + 10) * 5));
              item.view.rerankerScore = result.score;

              // Blend: 70% original score + 30% re-ranker score
              const blendedScore =
                item.view.matchScoreV2 * 0.7 + normalizedScore * 0.3;
              item.view.matchScoreV2 = Math.round(blendedScore * 10) / 10;
              item.view.matchScore = item.view.matchScoreV2;
            }
          }

          // Re-sort after re-ranking
          topCandidates.sort(
            (a, b) => (b.view.matchScoreV2 || 0) - (a.view.matchScoreV2 || 0),
          );

          this.logger.log(
            `Re-ranked ${rerankResult.results.length} candidates`,
          );
        }
      } catch (e) {
        this.logger.warn(`Re-ranking failed: ${(e as Error).message}`);
      }
    }
    rerankingTime = Date.now() - rerankingStart;

    // Assign final ranks
    topCandidates.forEach((item, idx) => {
      item.view.finalRank = idx + 1;
    });

    // Merge top candidates back into full list
    const rerankedIds = new Set(topCandidates.map((t) => t.view.id));
    const remaining = scoredCandidates.filter((item) => !rerankedIds.has(item.view.id));
    const finalCandidates = [...topCandidates, ...remaining];

    // ─── Step 6: Build result ───
    const result: SearchResultV2 = {
      total: finalCandidates.length,
      candidates: finalCandidates
        .slice(offset, offset + limit)
        .map((item) => item.view),
      queryUnderstanding: {
        originalQuery: queryText,
        parsedSpecialization: parsedNeed.specialization,
        parsedSeniority: parsedNeed.grade,
        parsedSkills: parsedNeed.requiredSkills,
        embeddingProvider: parsedNeed.embeddingProvider,
      },
      timing: {
        retrieval: retrievalTime,
        scoring: scoringTime,
        reranking: rerankingTime,
        total: Date.now() - totalStart,
      },
    };

    // Cache result
    const ttl = this.config.get<number>('matching.cacheTtlSeconds') || 900;
    await this.redis.set(cacheKey, JSON.stringify(result), ttl);

    this.logger.log(
      `search-v2: pool=${pool.length}, rendered=${result.candidates.length}, ` +
      `query=${queryTime}ms, retrieval=${retrievalTime}ms, ` +
      `scoring=${scoringTime}ms, reranking=${rerankingTime}ms, ` +
      `total=${result.timing.total}ms`,
    );

    return result;
  }

  /**
   * V2: Semantic search with ML embeddings.
   */
  async semanticSearch(dto: SemanticQueryDto) {
    const parsed = await this.mlClient.parseQuery(dto.query);
    const vector = parsed.embedding;

    const hits = await this.vectors.topBySimilarity(
      vector,
      Math.min((dto.limit || 10) * 5, 200),
    );
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
      queryUnderstanding: {
        originalQuery: dto.query,
        parsedSpecialization: parsed.parsed.specialization,
        parsedSeniority: parsed.parsed.seniority,
        parsedSkills: parsed.parsed.requiredSkills,
        embeddingProvider: parsed.embeddingProvider,
      },
      candidates: ordered.map((p) => {
        const view = toCandidatePublic(p, true);
        return {
          ...view,
          similarity: Number((similarityById.get(p.id) || 0).toFixed(4)),
          reason: p.bio
            ? p.bio.slice(0, 160)
            : `Стек: ${(p.stack || []).join(', ') || 'не указан'}`,
        };
      }),
    };
  }

  /**
   * V2: Create employer need with query understanding.
   */
  async createNeed(employerId: string, dto: EmployerNeedDto) {
    // Parse query with ML
    const parsedNeed = await this.queryUnderstanding.parseNeed(
      dto.description,
      dto.specialization,
      dto.grade,
      dto.stack,
    );

    // Store need
    const need: EmployerNeed = await this.prisma.employerNeed.create({
      data: {
        employerId,
        description: dto.description,
        specialization: parsedNeed.specialization || dto.specialization,
        grade: (parsedNeed.grade || dto.grade) as Grade | null | undefined,
        stack: parsedNeed.requiredSkills.length > 0
          ? parsedNeed.requiredSkills
          : dto.stack || [],
      },
    });

    // Store vector
    await this.vectors.setNeedVector(need.id, parsedNeed.embedding);

    // Run search
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
      queryUnderstanding: search.queryUnderstanding,
      candidatesCount: search.total,
      topCandidates: search.candidates,
      timing: search.timing,
    };
  }

  // ─────────────────────────── Helpers ───────────────────────────

  private buildWhere(dto: SearchCandidatesDto): Prisma.CandidateProfileWhereInput {
    const where: Prisma.CandidateProfileWhereInput = {};
    if (dto.specialization) where.specialization = dto.specialization;
    if (dto.grade) {
      const idx = GRADE_ORDER.indexOf(dto.grade);
      const allowed = [
        GRADE_ORDER[idx - 1],
        GRADE_ORDER[idx],
        GRADE_ORDER[idx + 1],
      ].filter((g): g is Grade => !!g);
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

  private defaultQueryText(dto: SearchCandidatesDto): string {
    return [
      dto.specialization,
      dto.grade,
      ...(dto.stack || []),
    ].filter(Boolean).join(' ');
  }

  private getWeight(key: string): number {
    const weights: Record<string, number> = {
      specialization: 0.18,
      stack: 0.15,
      semantic: 0.20,
      grade: 0.12,
      achievement: 0.08,
      experience: 0.12,
      testScore: 0.15,
      rerank: 0.10,
    };
    return weights[key] || 0;
  }

  private cacheKey(prefix: string, payload: unknown, ...extra: unknown[]): string {
    const normalized = JSON.stringify(
      payload,
      Object.keys(payload as object).sort(),
    );
    const hash = createHash('sha1')
      .update(normalized + JSON.stringify(extra))
      .digest('hex');
    return `matching:cache:${prefix}:${hash}`;
  }

  async invalidateCache(): Promise<void> {
    await this.redis.delByPattern('matching:cache:*');
  }

  async reindexCandidate(candidateId: string): Promise<void> {
    await this.indexing.indexCandidate(candidateId);
  }
}