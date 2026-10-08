import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import Redis from 'ioredis';

/**
 * Redis с деградацией: если хранилище недоступно — операции прозрачно пропускаются,
 * запросы не падают, а выполняются без кэша.
 */
@Injectable()
export class RedisService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger('Redis');
  private client: Redis | null = null;
  private available = false;
  private blockedUntil = 0;

  constructor(private readonly config: ConfigService) {}

  onModuleInit() {
    const url = this.config.get<string>('redisUrl') || 'redis://localhost:6379';
    try {
      this.client = new Redis(url, {
        maxRetriesPerRequest: 1,
        enableOfflineQueue: false,
        retryStrategy: (times) => (times > 5 ? null : Math.min(times * 500, 3000)),
        connectTimeout: 2000,
      });
      this.client.on('ready', () => {
        this.available = true;
        this.blockedUntil = 0;
        this.logger.log('Redis подключён');
      });
      this.client.on('error', (err) => {
        if (this.available) this.logger.warn(`Redis недоступен: ${err.message} — работаем без кэша`);
        this.available = false;
        this.blockedUntil = Date.now() + 30_000;
      });
    } catch (e) {
      this.available = false;
      this.logger.warn(`Redis не инициализирован: ${(e as Error).message}`);
    }
  }

  async onModuleDestroy() {
    try {
      await this.client?.quit();
    } catch {
      /* ignore */
    }
  }

  get isAvailable(): boolean {
    return this.available && Date.now() >= this.blockedUntil && !!this.client;
  }

  async get(key: string): Promise<string | null> {
    if (!this.isAvailable) return null;
    try {
      return await this.client!.get(key);
    } catch {
      this.available = false;
      return null;
    }
  }

  async set(key: string, value: string, ttlSeconds?: number): Promise<void> {
    if (!this.isAvailable) return;
    try {
      if (ttlSeconds) await this.client!.set(key, value, 'EX', ttlSeconds);
      else await this.client!.set(key, value);
    } catch {
      this.available = false;
    }
  }

  async delByPattern(pattern: string): Promise<void> {
    if (!this.isAvailable) return;
    try {
      let cursor = '0';
      do {
        const [next, keys] = await this.client!.scan(cursor, 'MATCH', pattern, 'COUNT', 200);
        cursor = next;
        if (keys.length) await this.client!.del(...keys);
      } while (cursor !== '0');
    } catch {
      this.available = false;
    }
  }
}
