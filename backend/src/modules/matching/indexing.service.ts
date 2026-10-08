import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { CandidateProfile } from '@prisma/client';
import { PrismaService } from '../../prisma/prisma.module';
import { EmbeddingService } from './embedding.service';
import { VectorRepository } from './vector.repository';

/** Индексация кандидатов: построение текстового представления + запись вектора в pgvector. */
@Injectable()
export class IndexingService implements OnModuleInit {
  private readonly logger = new Logger('Indexing');
  private reindexing = false;

  constructor(
    private readonly prisma: PrismaService,
    private readonly embedding: EmbeddingService,
    private readonly vectors: VectorRepository,
  ) {}

  onModuleInit() {
    // После загрузки ONNX-модели все вектора пересчитываются заново (совместимость размерностей/семантики)
    this.embedding.onProviderReady(() => {
      void this.reindexAll().catch((e) => this.logger.warn(`Переиндексация не удалась: ${e.message}`));
    });
    // Начальная индексация при старте (фоном, чтобы не задерживать boot)
    this.scheduleInitialIndex();
  }

  buildText(profile: CandidateProfile, achievementNames: string[] = []): string {
    return [
      profile.specialization,
      profile.verifiedGrade || profile.declaredGrade,
      profile.stack?.join(' '),
      profile.roles?.join(' '),
      profile.softSkills?.join(' '),
      profile.city,
      profile.experienceYears != null ? `${profile.experienceYears} лет опыта` : '',
      profile.bio,
      achievementNames.join(' '),
    ]
      .filter(Boolean)
      .join(' ');
  }

  /** Обучает TF-IDF на корпусе профилей (нужен только лексическому fallback-у). */
  async ensureCorpus(): Promise<void> {
    if (this.embedding.currentProvider === 'transformers') return;
    if (this.embedding.corpusFitted) return;
    const profiles = await this.prisma.candidateProfile.findMany({
      include: { achievements: { include: { achievement: true } } },
      take: 5000,
    });
    const texts = profiles.map((p) =>
      this.buildText(p, p.achievements.map((a) => a.achievement.name)),
    );
    this.embedding.fitCorpus(texts);
  }

  async indexCandidate(candidateId: string): Promise<void> {
    await this.ensureCorpus();
    const profile = await this.prisma.candidateProfile.findUnique({
      where: { id: candidateId },
      include: { achievements: { include: { achievement: true } } },
    });
    if (!profile) return;
    const text = this.buildText(profile, profile.achievements.map((a) => a.achievement.name));
    const vector = await this.embedding.embed(text);
    await this.vectors.setCandidateVector(profile.id, vector);
  }

  async indexBatch(ids: string[]): Promise<void> {
    if (!ids.length) return;
    await this.ensureCorpus();
    for (const id of ids) {
      try {
        await this.indexCandidate(id);
      } catch (e) {
        this.logger.warn(`Индексация ${id} не удалась: ${(e as Error).message}`);
      }
    }
  }

  async reindexAll(): Promise<void> {
    if (this.reindexing) return;
    this.reindexing = true;
    const started = Date.now();
    try {
      const profiles = await this.prisma.candidateProfile.findMany({
        include: { achievements: { include: { achievement: true } } },
        take: 5000,
      });
      const texts = profiles.map((p) => this.buildText(p, p.achievements.map((a) => a.achievement.name)));
      this.embedding.fitCorpus(texts);

      for (let i = 0; i < profiles.length; i++) {
        const profile = profiles[i];
        try {
          const vector = await this.embedding.embed(texts[i]);
          await this.vectors.setCandidateVector(profile.id, vector);
        } catch (e) {
          this.logger.warn(`Индексация ${profile.id} не удалась: ${(e as Error).message}`);
        }
      }
      this.logger.log(`Проиндексировано кандидатов: ${profiles.length} за ${Date.now() - started}ms`);
    } finally {
      this.reindexing = false;
    }
  }

  /** Запускает полную индексацию в фоне (один раз при старте). */
  scheduleInitialIndex(): void {
    setTimeout(() => {
      void this.reindexAll().catch((e) => this.logger.warn(`Начальная индексация: ${e.message}`));
    }, 2000);
  }
}
