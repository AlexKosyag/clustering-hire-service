import { BadRequestException, Injectable, Logger, NotFoundException } from '@nestjs/common';
import { CandidateProfile } from '@prisma/client';
import { IndexingService } from '../matching/indexing.service';
import { MatchingService } from '../matching/matching.service';
import { achievementPoints, candidateInclude, toCandidatePublic } from '../matching/candidate-view';
import { PrismaService } from '../../prisma/prisma.module';
import { StorageService } from '../storage/storage.service';
import { UpdateCandidateProfileDto } from './dto/candidate.dto';
import { PdfParsingService, ParsedResume } from './pdf-parsing.service';

const MAX_RESUME_BYTES = 5 * 1024 * 1024;
const MAX_IMAGE_BYTES = 2 * 1024 * 1024;

@Injectable()
export class CandidatesService {
  private readonly logger = new Logger('Candidates');

  constructor(
    private readonly prisma: PrismaService,
    private readonly matching: MatchingService,
    private readonly indexing: IndexingService,
    private readonly storage: StorageService,
    private readonly pdfParsing: PdfParsingService,
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
      resumeParsed: profile.resumeParsed,
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

  /** Текущая категория/грейд кандидата. */
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

  /**
   * Upload and parse resume PDF.
   * Extracts skills, experience, education, contacts.
   */
  async uploadResume(userId: string, file: any) {
    if (!file) throw new BadRequestException('Файл не передан');
    if (file.mimetype !== 'application/pdf') {
      throw new BadRequestException('Резюме должно быть PDF');
    }
    if (file.size > MAX_RESUME_BYTES) {
      throw new BadRequestException('Максимальный размер — 5 МБ');
    }

    const profile = await this.requireProfile(userId);

    // Upload file to storage
    const url = await this.storage.upload(
      'resumes',
      profile.id,
      'resume.pdf',
      file.buffer,
      'application/pdf',
    );

    // Parse PDF
    let parsed: ParsedResume | null = null;
    try {
      parsed = await this.pdfParsing.parseResume(file.buffer);
      this.logger.log(
        `Parsed resume for ${profile.id}: ${parsed.skills.length} skills, ` +
        `${parsed.experience.length} experience entries`,
      );
    } catch (e) {
      this.logger.warn(`PDF parsing failed for ${profile.id}: ${(e as Error).message}`);
    }

    // Update profile with parsed data
    const updateData: Record<string, unknown> = {
      resumePdfUrl: url,
    };

    if (parsed) {
      // Store parsed data
      updateData.resumeParsed = parsed as any;

      // Auto-fill profile fields if empty
      if (!profile.city && parsed.city) {
        updateData.city = parsed.city;
      }
      if (!profile.phone && parsed.phone) {
        updateData.phone = parsed.phone;
      }
      if (!profile.firstName && parsed.name) {
        const nameParts = parsed.name.split(' ');
        updateData.firstName = nameParts[0];
        if (nameParts[1]) updateData.lastName = nameParts[1];
        if (nameParts[2]) updateData.middleName = nameParts[2];
      }

      // Merge skills (don't overwrite existing)
      if (parsed.skills.length > 0) {
        const existingSkills = profile.stack || [];
        const newSkills = parsed.skills.filter(
          (s) => !existingSkills.some((e) => e.toLowerCase() === s.toLowerCase()),
        );
        if (newSkills.length > 0) {
          updateData.stack = [...existingSkills, ...newSkills];
        }
      }

      // Store bio from summary
      if (!profile.bio && parsed.summary) {
        updateData.bio = parsed.summary;
      }
    }

    await this.prisma.candidateProfile.update({
      where: { id: profile.id },
      data: updateData,
    });

    // Re-index for matching
    await this.indexing.indexCandidate(profile.id).catch(() => undefined);

    return {
      resumePdfUrl: url,
      parsed: parsed
        ? {
            name: parsed.name,
            email: parsed.email,
            phone: parsed.phone,
            city: parsed.city,
            skills: parsed.skills,
            experienceCount: parsed.experience.length,
            educationCount: parsed.education.length,
            summary: parsed.summary?.slice(0, 200),
          }
        : null,
      autoFilled: parsed
        ? {
            city: !profile.city && parsed.city,
            phone: !profile.phone && parsed.phone,
            name: !profile.firstName && parsed.name,
            skills: parsed.skills.length > 0,
          }
        : null,
    };
  }

  async uploadAvatar(userId: string, file: any) {
    if (!file) throw new BadRequestException('Файл не передан');
    if (!String(file.mimetype || '').startsWith('image/')) {
      throw new BadRequestException('Нужно изображение');
    }
    if (file.size > MAX_IMAGE_BYTES) {
      throw new BadRequestException('Максимальный размер — 2 МБ');
    }

    const profile = await this.requireProfile(userId);
    const ext = (String(file.mimetype).split('/')[1] || 'png').replace('jpeg', 'jpg');
    const url = await this.storage.upload(
      'avatars',
      profile.id,
      `avatar.${ext}`,
      file.buffer,
      file.mimetype,
    );
    await this.prisma.candidateProfile.update({
      where: { id: profile.id },
      data: { avatarUrl: url },
    });
    return { avatarUrl: url };
  }

  private async requireProfile(
    userId: string,
  ): Promise<CandidateProfile & { achievements: any[]; user?: any }> {
    const profile = await this.prisma.candidateProfile.findUnique({
      where: { userId },
      include: candidateInclude,
    });
    if (!profile) throw new NotFoundException('Профиль не найден');
    return profile as any;
  }
}