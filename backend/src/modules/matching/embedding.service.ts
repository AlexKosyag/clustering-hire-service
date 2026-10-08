import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import {
  EMBEDDING_DIM,
  TfidfVectorizer,
  normalize,
  unitVector,
} from './embedding/tfidf';

export type EmbeddingProvider = 'tfidf' | 'transformers';

/**
 * Каскадный embedding-сервис:
 *   1. Xenova/paraphrase-multilingual-MiniLM-L12-v2 (русский, 384 dim)
 *   2. Xenova/all-MiniLM-L6-v2 (английский, 384 dim)
 *   3. TF-IDF с feature-hashing (384 dim, офлайн)
 * Гарантия: embed() никогда не бросает исключение и всегда возвращает 384 finite-числа.
 */
@Injectable()
export class EmbeddingService implements OnModuleInit {
  private readonly logger = new Logger('Embedding');
  private readonly tfidf = new TfidfVectorizer();
  private pipe: any = null;
  private provider: EmbeddingProvider = 'tfidf';
  private modelName: string | null = null;
  private switching = false;
  private readyCallbacks: Array<() => void> = [];

  private static readonly MODELS = [
    'Xenova/paraphrase-multilingual-MiniLM-L12-v2',
    'Xenova/all-MiniLM-L6-v2',
  ];

  constructor(private readonly config: ConfigService) {}

  onModuleInit() {
    const mode = this.config.get<string>('embedding.provider') || 'auto';
    if (mode === 'tfidf') {
      this.logger.log('EMBEDDING_PROVIDER=tfidf — семантика отключена, используется лексический fallback');
      return;
    }
    // Загрузка модели идёт в фоне: запросы в это время обслуживаются TF-IDF
    void this.initTransformers(mode === 'transformers');
  }

  private async initTransformers(required: boolean): Promise<void> {
    if (this.switching) return;
    this.switching = true;
    try {
      let transformers: any;
      try {
        transformers = await import('@xenova/transformers');
      } catch (e) {
        this.logger.warn(`Пакет @xenova/transformers недоступен: ${(e as Error).message}. Работаем на TF-IDF.`);
        return;
      }

      for (const model of EmbeddingService.MODELS) {
        const started = Date.now();
        try {
          this.logger.log(`Загрузка модели ${model} ...`);
          this.pipe = await transformers.pipeline('feature-extraction', model, { quantized: true });
          this.modelName = model;
          this.provider = 'transformers';
          this.logger.log(`Модель готова: ${model} (${Date.now() - started} ms)`);
          this.notifyReady();
          return;
        } catch (e) {
          this.logger.warn(`Не удалось загрузить ${model}: ${(e as Error).message}`);
        }
      }
      if (required) {
        this.logger.error('Ни одна модель не загрузилась — включён лексический fallback (TF-IDF).');
      }
    } finally {
      this.switching = false;
    }
  }

  /** Регистрация callback-а: вызывается при переходе TF-IDF -> transformers (нужна переиндексация). */
  onProviderReady(cb: () => void): void {
    this.readyCallbacks.push(cb);
    if (this.provider === 'transformers') cb();
  }

  private notifyReady(): void {
    for (const cb of this.readyCallbacks) {
      try {
        cb();
      } catch (e) {
        this.logger.warn(`Ошибка переиндексации после загрузки модели: ${(e as Error).message}`);
      }
    }
  }

  get currentProvider(): EmbeddingProvider {
    return this.provider;
  }

  get currentModel(): string | null {
    return this.modelName;
  }

  get dim(): number {
    return EMBEDDING_DIM;
  }

  /** Обучение TF-IDF на корпусе (вызывается при индексации). */
  fitCorpus(texts: string[]): void {
    this.tfidf.fit(texts);
  }

  get corpusFitted(): boolean {
    return this.tfidf.isFitted;
  }

  async embed(text: string): Promise<number[]> {
    if (this.provider === 'transformers' && this.pipe) {
      try {
        const output = await this.pipe(text || '', { pooling: 'mean', normalize: true });
        const raw = Array.from(output.data as Float32Array | number[]) as number[];
        if (raw.length === EMBEDDING_DIM && raw.some((v) => v !== 0)) {
          return normalize(raw);
        }
      } catch (e) {
        this.logger.warn(`transformers embed failed, fallback to TF-IDF: ${(e as Error).message}`);
        this.provider = 'tfidf';
        this.pipe = null;
      }
    }
    return this.tfidf.embed(text);
  }

  /** Синхронный лексический вектор — для построения текстовых представлений без I/O. */
  embedLexicalSync(text: string): number[] {
    return this.tfidf.embed(text);
  }

  unit(): number[] {
    return unitVector();
  }
}
