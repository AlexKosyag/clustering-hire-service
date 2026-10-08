import { Injectable, Logger } from '@nestjs/common';
import { HttpService } from '@nestjs/axios';
import { firstValueFrom } from 'rxjs';
import { AxiosResponse } from 'axios';

export interface ParsedQuery {
  specialization: string | null;
  seniority: string | null;
  requiredSkills: string[];
  preferredSkills: string[];
  context: string | null;
  intent: string;
}

export interface QueryUnderstanding {
  originalQuery: string;
  parsed: ParsedQuery;
  embedding: number[];
  embeddingProvider: string;
  latencyMs: number;
}

export interface RerankResult {
  index: number;
  score: number;
  document: string;
}

// API Response types
interface HealthResponse {
  status: string;
  models: {
    embedding: { ready: boolean; name: string; dim: number };
    reranker: { ready: boolean; name: string };
    ner: { ready: boolean };
  };
  cache: { size: number; max_size: number };
}

interface EmbedResponse {
  embedding: number[];
  dim: number;
  provider: string;
  cached: boolean;
  latency_ms: number;
}

interface EmbedBatchResponse {
  embeddings: number[][];
  dim: number;
  provider: string;
  latency_ms: number;
}

interface ParseQueryResponse {
  parsed: ParsedQuery;
  embedding: number[];
  embedding_provider: string;
  latency_ms: number;
}

interface RerankApiResponse {
  results: RerankResult[];
  latency_ms: number;
}

interface NERApiResponse {
  entities: Array<{ text: string; label: string; start: number; end: number }>;
  latency_ms: number;
}

/**
 * Client for Python ML Service (FastAPI).
 *
 * Provides:
 * - Embeddings (768-dim, multilingual MiniLM)
 * - Cross-encoder re-ranking
 * - NER + query parsing
 * - TF-IDF fallback when ML unavailable
 */
@Injectable()
export class MlServiceClient {
  private readonly logger = new Logger('MlClient');
  private readonly baseUrl: string;
  private readonly timeoutMs: number;
  private _available: boolean | null = null;
  private _lastHealthCheck = 0;
  private readonly healthCheckIntervalMs = 30000;

  // Local TF-IDF fallback (when ML service unavailable)
  private tfidfVocab: Map<string, number> = new Map();
  private tfidfIdf: Map<string, number> = new Map();
  private tfidfFitted = false;

  constructor(private readonly http: HttpService) {
    this.baseUrl = process.env.ML_SERVICE_URL || 'http://localhost:8000';
    this.timeoutMs = parseInt(process.env.ML_SERVICE_TIMEOUT || '5000', 10);
  }

  get available(): boolean {
    return this._available ?? false;
  }

  async checkHealth(force = false): Promise<boolean> {
    const now = Date.now();
    if (!force && this._available !== null && now - this._lastHealthCheck < this.healthCheckIntervalMs) {
      return this._available;
    }

    try {
      const response = await firstValueFrom<AxiosResponse<HealthResponse>>(
        this.http.get<HealthResponse>(`${this.baseUrl}/health`, { timeout: 2000 }),
      );
      this._available = response.data?.status === 'ok';
      this._lastHealthCheck = now;

      if (this._available) {
        const models = response.data?.models || { embedding: { ready: false }, reranker: { ready: false }, ner: { ready: false } };
        this.logger.log(
          `ML Service available — embedding=${models.embedding?.ready}, reranker=${models.reranker?.ready}`,
        );
      }
      return this._available;
    } catch {
      this._available = false;
      this._lastHealthCheck = now;
      return false;
    }
  }

  /**
   * Generate embedding for text.
   * Tries ML service first, falls back to local TF-IDF.
   */
  async embed(text: string): Promise<{ embedding: number[]; provider: string; latencyMs: number }> {
    const t0 = Date.now();

    // Try ML service
    if (await this.checkHealth()) {
      try {
        const response = await firstValueFrom<AxiosResponse<EmbedResponse>>(
          this.http.post<EmbedResponse>(
            `${this.baseUrl}/embed`,
            { text },
            { timeout: this.timeoutMs },
          ),
        );
        return {
          embedding: response.data.embedding,
          provider: response.data.provider,
          latencyMs: Date.now() - t0,
        };
      } catch (e) {
        this.logger.warn(`ML embed failed: ${(e as Error).message}, using TF-IDF fallback`);
      }
    }

    // Fallback: local TF-IDF
    const embedding = this.tfidfEmbed(text);
    return {
      embedding,
      provider: 'tfidf-local',
      latencyMs: Date.now() - t0,
    };
  }

  /**
   * Batch embedding (more efficient).
   */
  async embedBatch(texts: string[]): Promise<{ embeddings: number[][]; provider: string }> {
    if (await this.checkHealth()) {
      try {
        const response = await firstValueFrom<AxiosResponse<EmbedBatchResponse>>(
          this.http.post<EmbedBatchResponse>(
            `${this.baseUrl}/embed/batch`,
            { texts, normalize: true },
            { timeout: Math.min(this.timeoutMs * texts.length, 30000) },
          ),
        );
        return {
          embeddings: response.data.embeddings,
          provider: response.data.provider,
        };
      } catch (e) {
        this.logger.warn(`ML batch embed failed: ${(e as Error).message}`);
      }
    }

    // Fallback
    return {
      embeddings: texts.map((t) => this.tfidfEmbed(t)),
      provider: 'tfidf-local',
    };
  }

  /**
   * Full query understanding: parse + embed.
   */
  async parseQuery(query: string): Promise<QueryUnderstanding> {
    const t0 = Date.now();

    if (await this.checkHealth()) {
      try {
        const response = await firstValueFrom<AxiosResponse<ParseQueryResponse>>(
          this.http.post<ParseQueryResponse>(
            `${this.baseUrl}/parse-query`,
            { query },
            { timeout: this.timeoutMs },
          ),
        );
        return {
          originalQuery: query,
          parsed: response.data.parsed,
          embedding: response.data.embedding,
          embeddingProvider: response.data.embedding_provider,
          latencyMs: Date.now() - t0,
        };
      } catch (e) {
        this.logger.warn(`ML parse query failed: ${(e as Error).message}`);
      }
    }

    // Fallback: local parsing + TF-IDF embedding
    const parsed = this.localParseQuery(query);
    const embedding = this.tfidfEmbed(query);
    return {
      originalQuery: query,
      parsed,
      embedding,
      embeddingProvider: 'tfidf-local',
      latencyMs: Date.now() - t0,
    };
  }

  /**
   * Cross-encoder re-ranking.
   * Returns re-ranked indices with scores.
   */
  async rerank(
    query: string,
    documents: string[],
    topK?: number,
  ): Promise<{ results: RerankResult[]; available: boolean }> {
    if (await this.checkHealth()) {
      try {
        const response = await firstValueFrom<AxiosResponse<RerankApiResponse>>(
          this.http.post<RerankApiResponse>(
            `${this.baseUrl}/rerank`,
            { query, documents, top_k: topK },
            { timeout: 10000 },
          ),
        );
        return {
          results: response.data.results,
          available: true,
        };
      } catch (e) {
        this.logger.warn(`ML rerank failed: ${(e as Error).message}`);
      }
    }

    // Fallback: no re-ranking (return original order)
    return {
      results: documents.map((doc, i) => ({
        index: i,
        score: 0.5,
        document: doc,
      })),
      available: false,
    };
  }

  /**
   * Named Entity Recognition.
   */
  async ner(text: string): Promise<Array<{ text: string; label: string }>> {
    if (await this.checkHealth()) {
      try {
        const response = await firstValueFrom<AxiosResponse<NERApiResponse>>(
          this.http.post<NERApiResponse>(`${this.baseUrl}/ner`, { text }, { timeout: this.timeoutMs }),
        );
        return response.data.entities;
      } catch {
        // Fallback below
      }
    }

    // Simple keyword-based fallback
    return this.localNer(text);
  }

  // ─────────────────────────── Local Fallbacks ───────────────────────────

  /**
   * Local TF-IDF embedding (when ML service unavailable).
   * Uses feature hashing for consistent 768-dim output.
   */
  private tfidfEmbed(text: string): number[] {
    const tokens = this.tokenize(text);
    const vector = new Array(768).fill(0);

    for (const token of tokens) {
      let idx: number;
      if (this.tfidfVocab.has(token)) {
        idx = this.tfidfVocab.get(token)!;
      } else {
        // Feature hashing
        idx = this.hashString(token) % 768;
      }
      const weight = this.tfidfIdf.get(token) ?? 1.0;
      vector[idx] += weight;
    }

    // Normalize
    const norm = Math.sqrt(vector.reduce((sum, v) => sum + v * v, 0));
    if (norm > 0) {
      return vector.map((v) => v / norm);
    }
    return vector;
  }

  private tokenize(text: string): string[] {
    return text
      .toLowerCase()
      .replace(/[^\w\s]/g, ' ')
      .split(/\s+/)
      .filter((t) => t.length > 1);
  }

  private hashString(str: string): number {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      hash = (hash * 31 + str.charCodeAt(i)) | 0;
    }
    return Math.abs(hash);
  }

  /**
   * Local query parsing (fallback).
   */
  private localParseQuery(query: string): ParsedQuery {
    const lower = query.toLowerCase();

    // Simple specialization detection
    const specKeywords: Record<string, string[]> = {
      BACKEND: ['backend', 'бэкенд', 'server', 'api', 'микросервис'],
      FRONTEND: ['frontend', 'фронтенд', 'react', 'vue', 'angular'],
      FULLSTACK: ['fullstack', 'full-stack', 'полный стек'],
      MOBILE: ['mobile', 'мобайл', 'ios', 'android', 'flutter'],
      DEVOPS: ['devops', 'инфраструктура', 'kubernetes', 'docker'],
      DATA_SCIENCE: ['data science', 'ml', 'machine learning', 'аналитик'],
      QA: ['qa', 'тестировщик', 'quality'],
      SECURITY: ['security', 'безопасность'],
    };

    let specialization: string | null = null;
    for (const [spec, keywords] of Object.entries(specKeywords)) {
      if (keywords.some((k) => lower.includes(k))) {
        specialization = spec;
        break;
      }
    }

    // Simple grade detection
    const gradeKeywords: Record<string, string[]> = {
      JUNIOR: ['junior', 'джуниор', 'начинающий'],
      MIDDLE: ['middle', 'мидл'],
      SENIOR: ['senior', 'синьор', 'опытный'],
      LEAD: ['lead', 'лид', 'team lead'],
    };

    let seniority: string | null = null;
    for (const [grade, keywords] of Object.entries(gradeKeywords)) {
      if (keywords.some((k) => lower.includes(k))) {
        seniority = grade;
        break;
      }
    }

    // Extract tech skills (common ones)
    const techSkills = [
      'node.js', 'nodejs', 'javascript', 'typescript', 'react', 'vue', 'angular',
      'python', 'django', 'flask', 'fastapi', 'java', 'spring', 'go', 'golang',
      'c++', 'c#', '.net', 'rust', 'php', 'ruby', 'swift', 'kotlin',
      'postgresql', 'mysql', 'mongodb', 'redis', 'elasticsearch',
      'docker', 'kubernetes', 'aws', 'gcp', 'azure',
      'graphql', 'rest', 'grpc', 'kafka', 'rabbitmq',
      'microservices', 'ci/cd', 'git',
    ];

    const requiredSkills = techSkills.filter((skill) => lower.includes(skill));

    return {
      specialization,
      seniority,
      requiredSkills,
      preferredSkills: [],
      context: query.slice(0, 200),
      intent: 'hire',
    };
  }

  /**
   * Local NER (fallback).
   */
  private localNer(text: string): Array<{ text: string; label: string }> {
    const results: Array<{ text: string; label: string }> = [];
    const lower = text.toLowerCase();

    // Tech entities
    const techTerms = [
      'node.js', 'react', 'python', 'docker', 'kubernetes', 'postgresql',
      'redis', 'kafka', 'grpc', 'graphql', 'typescript', 'javascript',
    ];
    for (const term of techTerms) {
      if (lower.includes(term)) {
        results.push({ text: term, label: 'TECH' });
      }
    }

    return results;
  }
}