import { Injectable, Logger, ServiceUnavailableException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as Minio from 'minio';

export type StorageKind = 'resumes' | 'avatars' | 'logos';

/**
 * Файловое хранилище (MinIO/S3). Если хранилище недоступно — явная 503,
 * чтобы не потерять данные молча.
 */
@Injectable()
export class StorageService {
  private readonly logger = new Logger('Storage');
  private client: Minio.Client | null = null;
  private bucket: string;
  private publicBase: string;
  private initTried = false;

  constructor(config: ConfigService) {
    this.bucket = config.get('minio.bucket') || 'fsp-platform';
    const raw = config.get('minio.endpoint') || 'http://localhost:9000';
    const publicRaw = process.env.MINIO_PUBLIC_ENDPOINT || 'http://localhost:9000';
    this.publicBase = publicRaw.replace(/\/$/, '');

    try {
      const url = new URL(raw);
      this.client = new Minio.Client({
        endPoint: url.hostname,
        port: url.port ? Number(url.port) : url.protocol === 'https:' ? 443 : 80,
        useSSL: url.protocol === 'https:',
        accessKey: config.get('minio.accessKey') || 'minioadmin',
        secretKey: config.get('minio.secretKey') || 'minioadmin',
      });
    } catch (e) {
      this.logger.warn(`MinIO не инициализирован: ${(e as Error).message}`);
      this.client = null;
    }
  }

  private async ensureBucket(): Promise<void> {
    if (!this.client) throw new ServiceUnavailableException('Файловое хранилище не настроено');
    if (this.initTried) return;
    try {
      const exists = await this.client.bucketExists(this.bucket);
      if (!exists) await this.client.makeBucket(this.bucket);
      this.initTried = true;
    } catch (e) {
      this.logger.warn(`MinIO bucket недоступен: ${(e as Error).message}`);
      throw new ServiceUnavailableException('Файловое хранилище недоступно');
    }
  }

  async upload(kind: StorageKind, owner: string, filename: string, buffer: Buffer, contentType: string): Promise<string> {
    await this.ensureBucket();
    const key = `${kind}/${owner}/${filename}`;
    try {
      await this.client!.putObject(this.bucket, key, buffer, buffer.length, { 'Content-Type': contentType });
      return `${this.publicBase}/${this.bucket}/${key}`;
    } catch (e) {
      this.logger.error(`Ошибка загрузки ${key}: ${(e as Error).message}`);
      throw new ServiceUnavailableException('Не удалось сохранить файл');
    }
  }
}
