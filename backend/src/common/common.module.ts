import { Module } from '@nestjs/common';
import { APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { JwtAuthGuard, RolesGuard } from './guards/jwt.guard';
import { LoggingInterceptor } from './interceptors/logging.interceptor';
import { RedisModule } from './redis/redis.module';

/** Глобальные guard-ы и interceptors для всего приложения. */
@Module({
  imports: [RedisModule],
  providers: [
    { provide: APP_GUARD, useClass: JwtAuthGuard },
    { provide: APP_GUARD, useClass: RolesGuard },
    { provide: APP_INTERCEPTOR, useClass: LoggingInterceptor },
  ],
})
export class CommonModule {}
