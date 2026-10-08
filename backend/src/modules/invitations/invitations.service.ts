import {
  BadRequestException,
  ConflictException,
  ForbiddenException,
  Injectable,
  NotFoundException,
} from '@nestjs/common';
import { InvitationStatus, UserRole } from '@prisma/client';
import { candidateInclude, toCandidatePublic } from '../matching/candidate-view';
import { MatchingService } from '../matching/matching.service';
import { PrismaService } from '../../prisma/prisma.module';
import { CreateInvitationDto, InvitationStatusDto } from './dto/invitation.dto';

@Injectable()
export class InvitationsService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly matching: MatchingService,
  ) {}

  /** Работодатель приглашает кандидата (EMPLOYER-first: работодатель выходит на контакт). */
  async create(userId: string, dto: CreateInvitationDto) {
    const employer = await this.prisma.employerProfile.findUnique({ where: { userId } });
    if (!employer) throw new NotFoundException('Профиль работодателя не найден');

    const candidate = await this.prisma.candidateProfile.findUnique({
      where: { id: dto.candidateId },
    });
    if (!candidate) throw new NotFoundException('Кандидат не найден');

    if (dto.vacancyId) {
      const vacancy = await this.prisma.vacancy.findFirst({
        where: { id: dto.vacancyId, employerId: employer.id },
      });
      if (!vacancy) throw new BadRequestException('Вакансия не найдена у этого работодателя');
    }

    const duplicate = await this.prisma.invitation.findFirst({
      where: {
        employerId: employer.id,
        candidateId: candidate.id,
        status: { in: [InvitationStatus.SENT, InvitationStatus.VIEWED, InvitationStatus.ACCEPTED] },
      },
    });
    if (duplicate) throw new ConflictException('Кандидат уже приглашён по этому приглашению');

    if (dto.salaryFrom > dto.salaryTo) {
      throw new BadRequestException('salaryFrom не может быть больше salaryTo');
    }

    const invitation = await this.prisma.invitation.create({
      data: {
        employerId: employer.id,
        candidateId: candidate.id,
        vacancyId: dto.vacancyId ?? null,
        message: dto.message,
        salaryFrom: dto.salaryFrom,
        salaryTo: dto.salaryTo,
      },
      include: { candidate: { include: candidateInclude } },
    });

    await this.matching.invalidateCache().catch(() => undefined);

    return {
      ...invitation,
      candidate: toCandidatePublic(invitation.candidate),
    };
  }

  /** Список приглашений: кандидат видит входящие, работодатель — исходящие. */
  async list(userId: string, role: UserRole, status?: InvitationStatus) {
    const where = await this.scopeWhere(userId, role);
    const invitations = await this.prisma.invitation.findMany({
      where: { ...where, ...(status ? { status } : {}) },
      orderBy: { createdAt: 'desc' },
      take: 100,
      include: {
        candidate: { include: candidateInclude },
        employer: true,
        vacancy: true,
      },
    });

    return {
      invitations: invitations.map((i) => ({
        id: i.id,
        status: i.status,
        statusUpdatedAt: i.statusUpdatedAt,
        createdAt: i.createdAt,
        message: i.message,
        salaryFrom: i.salaryFrom,
        salaryTo: i.salaryTo,
        vacancy: i.vacancy
          ? { id: i.vacancy.id, title: i.vacancy.title, grade: i.vacancy.grade }
          : null,
        employer:
          role === UserRole.CANDIDATE
            ? {
                id: i.employer.id,
                companyName: i.employer.companyName,
                industry: i.employer.industry,
                logoUrl: i.employer.logoUrl,
              }
            : undefined,
        candidate:
          role === UserRole.EMPLOYER ? toCandidatePublic(i.candidate) : undefined,
      })),
    };
  }

  async get(userId: string, role: UserRole, id: string) {
    const where = await this.scopeWhere(userId, role);
    const invitation = await this.prisma.invitation.findFirst({
      where: { id, ...where },
      include: {
        candidate: { include: candidateInclude },
        employer: true,
        vacancy: true,
      },
    });
    if (!invitation) throw new NotFoundException('Приглашение не найдено');

    // Отметка «просмотрено» — входящее приглашение кандидатом
    if (role === UserRole.CANDIDATE && invitation.status === InvitationStatus.SENT) {
      await this.prisma.invitation.update({
        where: { id: invitation.id },
        data: { status: InvitationStatus.VIEWED, statusUpdatedAt: new Date() },
      });
      invitation.status = InvitationStatus.VIEWED;
    }

    const accepted = invitation.status === InvitationStatus.ACCEPTED;
    return {
      id: invitation.id,
      status: invitation.status,
      message: invitation.message,
      salaryFrom: invitation.salaryFrom,
      salaryTo: invitation.salaryTo,
      createdAt: invitation.createdAt,
      statusUpdatedAt: invitation.statusUpdatedAt,
      vacancy: invitation.vacancy,
      employer: {
        id: invitation.employer.id,
        companyName: invitation.employer.companyName,
        industry: invitation.employer.industry,
        logoUrl: invitation.employer.logoUrl,
        website: invitation.employer.website,
      },
      candidate:
        role === UserRole.EMPLOYER
          ? {
              ...toCandidatePublic(invitation.candidate),
              contacts: accepted
                ? {
                    email: invitation.candidate.user?.email ?? null,
                    phone: invitation.candidate.phone,
                  }
                : null,
              contactsLocked: !accepted,
            }
          : undefined,
    };
  }

  /** Принять / отклонить — только кандидат. */
  async respond(userId: string, id: string, dto: InvitationStatusDto) {
    const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
    if (!profile) throw new NotFoundException('Профиль кандидата не найден');

    const invitation = await this.prisma.invitation.findUnique({ where: { id } });
    if (!invitation || invitation.candidateId !== profile.id) {
      throw new NotFoundException('Приглашение не найдено');
    }
    const openStatuses: InvitationStatus[] = [InvitationStatus.SENT, InvitationStatus.VIEWED];
    if (!openStatuses.includes(invitation.status)) {
      throw new ConflictException(`Приглашение уже обработано: ${invitation.status}`);
    }

    const status =
      dto.status === 'ACCEPTED' ? InvitationStatus.ACCEPTED : InvitationStatus.REJECTED;

    const updated = await this.prisma.invitation.update({
      where: { id },
      data: { status, statusUpdatedAt: new Date() },
      include: { candidate: { include: candidateInclude } },
    });

    await this.matching.invalidateCache().catch(() => undefined);

    return {
      ...updated,
      candidate: toCandidatePublic(updated.candidate),
      message: status === InvitationStatus.ACCEPTED
        ? 'Приглашение принято — работодатель получил ваши контакты'
        : 'Приглашение отклонено',
    };
  }

  /** Отзыв приглашения — только работодатель. */
  async withdraw(userId: string, id: string) {
    const employer = await this.prisma.employerProfile.findUnique({ where: { userId } });
    if (!employer) throw new NotFoundException('Профиль работодателя не найден');

    const invitation = await this.prisma.invitation.findUnique({ where: { id } });
    if (!invitation || invitation.employerId !== employer.id) {
      throw new ForbiddenException('Это не ваше приглашение');
    }
    if (invitation.status === InvitationStatus.ACCEPTED) {
      throw new ConflictException('Принятое приглашение нельзя отозвать');
    }

    return this.prisma.invitation.update({
      where: { id },
      data: { status: InvitationStatus.WITHDRAWN, statusUpdatedAt: new Date() },
    });
  }

  private async scopeWhere(userId: string, role: UserRole) {
    if (role === UserRole.CANDIDATE) {
      const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
      if (!profile) throw new NotFoundException('Профиль кандидата не найден');
      return { candidateId: profile.id };
    }
    const employer = await this.prisma.employerProfile.findUnique({ where: { userId } });
    if (!employer) throw new NotFoundException('Профиль работодателя не найден');
    return { employerId: employer.id };
  }
}
