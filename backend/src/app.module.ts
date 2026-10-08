import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import configuration from './config/configuration';
import { PrismaModule } from './prisma/prisma.module';
import { AuthModule } from './modules/auth/auth.module';
import { CandidatesModule } from './modules/candidates/candidates.module';
import { EmployersModule } from './modules/employers/employers.module';
import { InvitationsModule } from './modules/invitations/invitations.module';
import { VacanciesModule } from './modules/vacancies/vacancies.module';
import { TestingModule } from './modules/testing/testing.module';
import { FspModule } from './modules/fsp/fsp.module';
import { MatchingModule } from './modules/matching/matching.module';
import { NotificationsModule } from './modules/notifications/notifications.module';
import { StorageModule } from './modules/storage/storage.module';
import { HealthModule } from './modules/health/health.module';
import { GamificationModule } from './modules/gamification/gamification.module';
import { CommonModule } from './common/common.module';

@Module({
  imports: [
    ConfigModule.forRoot({ isGlobal: true, load: [configuration] }),
    CommonModule,
    PrismaModule,
    StorageModule,
    AuthModule,
    CandidatesModule,
    EmployersModule,
    InvitationsModule,
    VacanciesModule,
    TestingModule,
    FspModule,
    MatchingModule,
    NotificationsModule,
    HealthModule,
    GamificationModule,
  ],
})
export class AppModule {}
