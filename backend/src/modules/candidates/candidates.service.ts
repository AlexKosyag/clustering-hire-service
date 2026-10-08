import { BadRequestException, Injectable, NotFoundException } from '@nestjs/common';
import { CandidateProfile } from '@prisma/client';
import { IndexingService } from '../matching/indexing.service';
import { MatchingService } from '../matching/matching.service';
import { achievementPoints, candidateInclude, toCandidatePublic } from '../matching/candidate-view';
import { PrismaService } from '../../prisma/prisma.module';
import { StorageService } from '../storage/storage.service';
import { UpdateCandidateProfileDto } from './dto/candidate.dto';

const MAX_RESUME_BYTES = 5 * 1024 * 1024;
const MAX_IMAGE_BYTES = 2 * 1024 * 1024;

@Injectable()
export class CandidatesService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly matching: MatchingService,
    private readonly indexing: IndexingService,
    private readonly storage: StorageService,
  ) {}

  async profile(userId: string) {
    const profile = await this.prisma.candidateProfile.findUnique({
      where: { userId },
      include: candidateInclude,
    });
    if (!profile) throw new NotFoundException('Профиль не найден');
    return {
      ...toCandidatePublic(profile),
      email: profile.user?.email ?? null,
      phone: profile.phone,
      middleName: profile.middleName,
      avatarUrl: profile.avatarUrl,
      fspId: profile.fspId,
      fspConnectedAt: profile.fspConnectedAt,
      resumePdfUrl: profile.resumePdfUrl,
      showEmail: profile.showEmail,
      showPhone: profile.showPhone,
      showFullName: profile.showFullName,
      consentToProcessing: profile.consentToProcessing,
      testCooldownUntil: profile.testCooldownUntil,
      createdAt: profile.createdAt,
    };
  }

  async updateProfile(userId: string, dto: UpdateCandidateProfileDto) {
    const data: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(dto)) {
      if (value !== undefined) data[key] = value;
    }

    const updated = await this.prisma.candidateProfile.update({
      where: { userId },
      data,
      include: candidateInclude,
    });

    await this.matching.invalidateCache().catch(() => undefined);
    await this.indexing.indexCandidate(updated.id).catch(() => undefined);

    return this.profile(userId);
  }

  /** Текущая категория/грейд кандидата (docs: GET /api/candidates/category). */
  async category(userId: string) {
    const profile = await this.requireProfile(userId);
    const sessions = await this.prisma.testSession.count({
      where: { candidateId: profile.id, status: 'COMPLETED' as any },
    });
    return {
      specialization: profile.specialization,
      declaredGrade: profile.declaredGrade,
      verifiedGrade: profile.verifiedGrade,
      testStatus: profile.testStatus,
      lastTestDate: profile.lastTestDate,
      lastTestScore: profile.lastTestScore,
      canRetakeAt: profile.testCooldownUntil,
      completedTests: sessions,
      achievements: profile.achievements.map((a) => a.achievement),
      achievementPoints: achievementPoints(profile),
      fspId: profile.fspId,
    };
  }

  async uploadResume(userId: string, file: any) {
    if (!file) throw new BadRequestException('Файл не передан');
    if (file.mimetype !== 'application/pdf') throw new BadRequestException('Резюме должно быть PDF');
    if (file.size > MAX_RESUME_BYTES) throw new BadRequestException('Максимальный размер — 5 МБ');

    const profile = await this.requireProfile(userId);
    const url = await this.storage.upload('resumes', profile.id, 'resume.pdf', file.buffer, 'application/pdf');
    await this.prisma.candidateProfile.update({ where: { id: profile.id }, data: { resumePdfUrl: url } });
    return { resumePdfUrl: url, parsed: null, note: 'PDF сохранён. Автопарсинг — после MVP (concepts).' };
  }

  async uploadAvatar(userId: string, file: any) {
    if (!file) throw new BadRequestException('Файл не передан');
    if (!String(file.mimetype || '').startsWith('image/')) throw new BadRequestException('Нужно изображение');
    if (file.size > MAX_IMAGE_BYTES) throw new BadRequestException('Максимальный размер — 2 МБ');

    const profile = await this.requireProfile(userId);
    const ext = (String(file.mimetype).split('/')[1] || 'png').replace('jpeg', 'jpg');
    const url = await this.storage.upload('avatars', profile.id, `avatar.${ext}`, file.buffer, file.mimetype);
    await this.prisma.candidateProfile.update({ where: { id: profile.id }, data: { avatarUrl: url } });
    return { avatarUrl: url };
  }

  private async requireProfile(userId: string): Promise<CandidateProfile & { achievements: any[]; user?: any }> {
    const profile = await this.prisma.candidateProfile.findUnique({
      where: { userId },
      include: candidateInclude,
    });
    if (!profile) throw new NotFoundException('Профиль не найден');
    return profile as any;
  }
}
