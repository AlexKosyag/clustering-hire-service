import { Injectable, Logger } from '@nestjs/common';
import { PrismaService } from '../../prisma/prisma.module';
import { RedisService } from '../../common/redis/redis.service';
import { EmbeddingService } from '../matching/embedding.service';

@Injectable()
export class HealthService {
  private readonly logger = new Logger('Health');
  private startedAt = Date.now();

  constructor(
    private readonly prisma: PrismaService,
    private readonly redis: RedisService,
    private readonly embedding: EmbeddingService,
  ) {}

  async check() {
    let database: 'up' | 'down' = 'down';
    try {
      await this.prisma.$queryRaw`SELECT 1`;
      database = 'up';
    } catch (e) {
      this.logger.warn(`DB check failed: ${(e as Error).message}`);
    }

    return {
      status: database === 'up' ? 'ok' : 'degraded',
      timestamp: new Date().toISOString(),
      uptimeSeconds: Math.round((Date.now() - this.startedAt) / 1000),
      services: {
        database,
        redis: this.redis.isAvailable ? 'up' : 'down (cache disabled)',
        embedding: {
          provider: this.embedding.currentProvider,
          model: this.embedding.currentModel,
          dim: this.embedding.dim,
        },
      },
    };
  }
}
