import { Injectable, Logger } from '@nestjs/common';
import { PrismaService } from '../../prisma/prisma.module';
import { EMBEDDING_DIM } from './embedding/tfidf';

export interface VectorHit {
  id: string;
  similarity: number;
}

/**
 * Работа с pgvector-колонками.
 * Поле объявлено как Unsupported("vector(384)") — Prisma-клиент его не видит,
 * поэтому весь доступ к векторам идёт через raw SQL (изолировано здесь).
 */
@Injectable()
export class VectorRepository {
  private readonly logger = new Logger('VectorRepo');
  private sqlBroken = false;

  constructor(private readonly prisma: PrismaService) {}

  /** Безопасная литеральная форма вектора: только finite-числа, фиксированная размерность. */
  private literal(vector: number[]): string {
    const safe: number[] = [];
    for (let i = 0; i < EMBEDDING_DIM; i++) {
      const v = vector?.[i];
      safe.push(Number.isFinite(v) ? Number(Number(v).toFixed(6)) : 0);
    }
    return `[${safe.join(',')}]`;
  }

  async setCandidateVector(candidateId: string, vector: number[]): Promise<void> {
    const lit = this.literal(vector);
    await this.prisma.$executeRaw`UPDATE candidate_profiles SET profile_embedding = ${lit}::vector WHERE id = ${candidateId}`;
  }

  async setNeedVector(needId: string, vector: number[]): Promise<void> {
    const lit = this.literal(vector);
    await this.prisma.$executeRaw`UPDATE employer_needs SET need_embedding = ${lit}::vector WHERE id = ${needId}`;
  }

  /** KNN-поиск по косинусному расстоянию. При ошибке SQL — fallback на чтение векторов в приложение. */
  async topBySimilarity(vector: number[], limit = 5000): Promise<VectorHit[]> {
    const lit = this.literal(vector);
    if (!this.sqlBroken) {
      try {
        const rows = await this.prisma.$queryRaw<Array<{ id: string; similarity: number }>>`
          SELECT id, (1 - (profile_embedding <=> ${lit}::vector))::float8 AS similarity
          FROM candidate_profiles
          WHERE profile_embedding IS NOT NULL
          ORDER BY profile_embedding <=> ${lit}::vector
          LIMIT ${Number(limit) || 100}`;
        return rows.map((r) => ({ id: String(r.id), similarity: Number(r.similarity) || 0 }));
      } catch (e) {
        this.sqlBroken = true;
        this.logger.error(`KNN-запрос не выполнен, включён fallback: ${(e as Error).message}`);
      }
    }
    return this.topBySimilarityInMemory(vector, limit);
  }

  /** Fallback: читает вектора как текст и сортирует в приложении. */
  async topBySimilarityInMemory(vector: number[], limit: number): Promise<VectorHit[]> {
    const rows = await this.prisma.$queryRaw<Array<{ id: string; v: string }>>`
      SELECT id, profile_embedding::text AS v FROM candidate_profiles WHERE profile_embedding IS NOT NULL`;
    const target = JSON.parse(this.literal(vector)) as number[];
    const hits: VectorHit[] = [];
    for (const row of rows) {
      try {
        const stored = JSON.parse(row.v) as number[];
        hits.push({ id: String(row.id), similarity: dot(target, stored) });
      } catch {
        /* пропускаем битые записи */
      }
    }
    hits.sort((a, b) => b.similarity - a.similarity);
    return hits.slice(0, limit);
  }

  async indexedIds(ids: string[]): Promise<Set<string>> {
    if (ids.length === 0) return new Set();
    try {
      const rows = await this.prisma.$queryRaw<Array<{ id: string }>>`
        SELECT id FROM candidate_profiles WHERE profile_embedding IS NOT NULL`;
      const all = new Set(rows.map((r) => String(r.id)));
      return new Set(ids.filter((id) => all.has(id)));
    } catch {
      return new Set();
    }
  }
}

/** Косинусная близость (вектора L2-нормированы, поэтому dot == cosine). */
function dot(a: number[], b: number[]): number {
  if (a.length !== b.length) return 0;
  let sum = 0;
  for (let i = 0; i < a.length; i++) sum += a[i] * b[i];
  return Number.isFinite(sum) ? sum : 0;
}
