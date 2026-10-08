import { BadRequestException, Injectable, NotFoundException } from '@nestjs/common';
import { EmployerProfile, InvitationStatus } from '@prisma/client';
import { candidateInclude, toCandidatePublic } from '../matching/candidate-view';
import { MatchingService } from '../matching/matching.service';
import { PrismaService } from '../../prisma/prisma.module';
import { StorageService } from '../storage/storage.service';
import { EmployerSearchQueryDto, UpdateEmployerProfileDto } from './dto/employer.dto';

@Injectable()
export class EmployersService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly matching: MatchingService,
    private readonly storage: StorageService,
  ) {}

  async profile(userId: string) {
    const profile = await this.requireProfile(userId);
    const { id, userId: _userId, ...safe } = profile as EmployerProfile;
    return { id, ...safe, user: undefined };
  }

  async updateProfile(userId: string, dto: UpdateEmployerProfileDto) {
    await this.requireProfile(userId);
    const data: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(dto)) {
      if (value !== undefined) data[key] = value;
    }
    await this.prisma.employerProfile.update({ where: { userId }, data });
    await this.matching.invalidateCache().catch(() => undefined);
    return this.profile(userId);
  }

  async uploadLogo(userId: string, file: any) {
    if (!file) throw new BadRequestException('Файл не передан');
    if (!String(file.mimetype || '').startsWith('image/')) throw new BadRequestException('Нужно изображение');
    if (file.size > 2 * 1024 * 1024) throw new BadRequestException('Максимальный размер — 2 МБ');

    const profile = await this.requireProfile(userId);
    const ext = (String(file.mimetype).split('/')[1] || 'png').replace('jpeg', 'jpg');
    const url = await this.storage.upload('logos', profile.id, `logo.${ext}`, file.buffer, file.mimetype);
    await this.prisma.employerProfile.update({ where: { id: profile.id }, data: { logoUrl: url } });
    return { logoUrl: url };
  }

  /** Описание потребности -> ранжированная подборка. */
  async createNeed(userId: string, dto: any) {
    const profile = await this.requireProfile(userId);
    return this.matching.createNeed(profile.id, dto);
  }

  async searchCandidates(userId: string, query: EmployerSearchQueryDto) {
    await this.requireProfile(userId);
    return this.matching.search(query);
  }

  /**
   * Профиль кандидата глазами работодателя.
   * Контакты раскрываются ТОЛЬКО после ACCEPTED (Employer-first модель).
   */
  async candidateDetail(userId: string, candidateId: string) {
    const employer = await this.requireProfile(userId);
    const candidate = await this.prisma.candidateProfile.findUnique({
      where: { id: candidateId },
      include: candidateInclude,
    });
    if (!candidate) throw new NotFoundException('Кандидат не найден');

    const invitation = await this.prisma.invitation.findFirst({
      where: { employerId: employer.id, candidateId: candidate.id },
      orderBy: { createdAt: 'desc' },
    });

    const accepted = invitation?.status === InvitationStatus.ACCEPTED;
    const view = toCandidatePublic(candidate, true);

    return {
      ...view,
      email: candidate.user?.email ?? null,
      invitationStatus: invitation?.status ?? null,
      contactsLocked: !accepted,
      contacts: accepted
        ? {
            fullName: [candidate.firstName, candidate.middleName, candidate.lastName].filter(Boolean).join(' '),
            email: candidate.user?.email ?? null,
            phone: candidate.phone,
            city: candidate.city,
          }
        : null,
      contactsHint: accepted
        ? 'Кандидат принял приглашение — контакты открыты'
        : 'Контактные данные будут доступны после принятия приглашения',
    };
  }

  async dashboard(userId: string) {
    const employer = await this.requireProfile(userId);
    const [invitations, vacancies, needs] = await Promise.all([
      this.prisma.invitation.findMany({
        where: { employerId: employer.id },
        orderBy: { createdAt: 'desc' },
        take: 10,
        include: { candidate: { include: candidateInclude } },
      }),
      this.prisma.vacancy.findMany({ where: { employerId: employer.id }, orderBy: { createdAt: 'desc' }, take: 10 }),
      this.prisma.employerNeed.findFirst({ where: { employerId: employer.id }, orderBy: { createdAt: 'desc' } }),
    ]);

    const byStatus = invitations.reduce<Record<string, number>>((acc, i) => {
      acc[i.status] = (acc[i.status] || 0) + 1;
      return acc;
    }, {});

    const recommendations = needs
      ? await this.matching.search({
          description: needs.description,
          specialization: needs.specialization ?? undefined,
          grade: needs.grade ?? undefined,
          stack: needs.stack ?? [],
          limit: 5,
          offset: 0,
        })
      : { total: 0, candidates: [] };

    return {
      stats: {
        invitations: invitations.length,
        invitationsByStatus: byStatus,
        vacancies: vacancies.length,
        candidatesMatched: recommendations.total,
      },
      recentInvitations: invitations.map((i) => ({
        id: i.id,
        status: i.status,
        createdAt: i.createdAt,
        candidate: toCandidatePublic(i.candidate),
      })),
      vacancies: vacancies.map((v) => ({
        id: v.id,
        title: v.title,
        specialization: v.specialization,
        grade: v.grade,
        salaryFrom: v.salaryFrom,
        salaryTo: v.salaryTo,
        isActive: v.isActive,
        createdAt: v.createdAt,
      })),
      topCandidates: recommendations.candidates,
    };
  }

  private async requireProfile(userId: string): Promise<EmployerProfile & { user?: any }> {
    const profile = await this.prisma.employerProfile.findUnique({ where: { userId } });
    if (!profile) throw new NotFoundException('Профиль работодателя не найден');
    return profile as any;
  }
}
