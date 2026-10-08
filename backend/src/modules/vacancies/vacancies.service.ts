import {
  BadRequestException,
  ConflictException,
  ForbiddenException,
  Injectable,
  NotFoundException,
} from '@nestjs/common';
import { ApplicationStatus, Grade, Specialization } from '@prisma/client';
import { candidateInclude, toCandidatePublic } from '../matching/candidate-view';
import { PrismaService } from '../../prisma/prisma.module';
import { ApplyDto, CreateVacancyDto, UpdateVacancyDto } from './dto/vacancy.dto';

@Injectable()
export class VacanciesService {
  constructor(private readonly prisma: PrismaService) {}

  /** Публичный список вакансий (доступен и кандидатам, и работодателям). */
  async list(query: {
    specialization?: Specialization;
    grade?: Grade;
    employerId?: string;
    search?: string;
    onlyActive?: string;
    limit?: number;
    offset?: number;
  }) {
    const limit = Math.min(Math.max(Number(query.limit) || 20, 1), 100);
    const offset = Math.max(Number(query.offset) || 0, 0);

    const where = {
      ...(query.specialization ? { specialization: query.specialization } : {}),
      ...(query.grade ? { grade: query.grade } : {}),
      ...(query.employerId ? { employerId: query.employerId } : {}),
      ...(query.onlyActive !== 'false' ? { isActive: true } : {}),
      ...(query.search
        ? {
            OR: [
              { title: { contains: query.search, mode: 'insensitive' as const } },
              { description: { contains: query.search, mode: 'insensitive' as const } },
            ],
          }
        : {}),
    };

    const [items, total] = await Promise.all([
      this.prisma.vacancy.findMany({
        where,
        include: { employer: { select: { id: true, companyName: true, logoUrl: true, industry: true } } },
        orderBy: { createdAt: 'desc' },
        skip: offset,
        take: limit,
      }),
      this.prisma.vacancy.count({ where }),
    ]);

    return { total, vacancies: items };
  }

  async get(id: string) {
    const vacancy = await this.prisma.vacancy.findUnique({
      where: { id },
      include: { employer: { select: { id: true, companyName: true, logoUrl: true, industry: true, website: true } } },
    });
    if (!vacancy) throw new NotFoundException('Вакансия не найдена');
    return vacancy;
  }

  async create(userId: string, dto: CreateVacancyDto) {
    const employer = await this.requireEmployer(userId);
    if (dto.salaryFrom > dto.salaryTo) {
      throw new BadRequestException('salaryFrom не может быть больше salaryTo');
    }
    return this.prisma.vacancy.create({
      data: { ...dto, employerId: employer.id },
      include: { employer: { select: { id: true, companyName: true, logoUrl: true } } },
    });
  }

  async update(userId: string, id: string, dto: UpdateVacancyDto) {
    const employer = await this.requireEmployer(userId);
    await this.requireOwnVacancy(employer.id, id);
    const data: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(dto)) {
      if (value !== undefined) data[key] = value;
    }
    return this.prisma.vacancy.update({ where: { id }, data });
  }

  async remove(userId: string, id: string) {
    const employer = await this.requireEmployer(userId);
    await this.requireOwnVacancy(employer.id, id);
    await this.prisma.vacancy.delete({ where: { id } });
    return { deleted: true };
  }

  /** Отклик кандидата на вакансию. */
  async apply(userId: string, vacancyId: string, dto: ApplyDto) {
    const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
    if (!profile) throw new NotFoundException('Профиль кандидата не найден');

    const vacancy = await this.prisma.vacancy.findUnique({ where: { id: vacancyId } });
    if (!vacancy || !vacancy.isActive) throw new NotFoundException('Вакансия не найдена или закрыта');

    const existing = await this.prisma.application.findUnique({
      where: { candidateId_vacancyId: { candidateId: profile.id, vacancyId } },
    });
    if (existing) throw new ConflictException('Отклик на эту вакансию уже отправлен');

    return this.prisma.application.create({
      data: { candidateId: profile.id, vacancyId, coverLetter: dto.coverLetter, status: ApplicationStatus.PENDING },
      include: { vacancy: { select: { id: true, title: true, salaryFrom: true, salaryTo: true } } },
    });
  }

  /** Отклики на вакансию (только работодатель-владелец). */
  async applications(userId: string, vacancyId: string, status?: ApplicationStatus) {
    const employer = await this.requireEmployer(userId);
    await this.requireOwnVacancy(employer.id, vacancyId);

    const applications = await this.prisma.application.findMany({
      where: { vacancyId, ...(status ? { status } : {}) },
      orderBy: { createdAt: 'desc' },
      include: { candidate: { include: candidateInclude } },
    });

    return {
      applications: applications.map((a) => ({
        id: a.id,
        status: a.status,
        coverLetter: a.coverLetter,
        createdAt: a.createdAt,
        candidate: toCandidatePublic(a.candidate, true),
      })),
    };
  }

  /** Статус отклика (PENDING/VIEWED/SHORTLISTED/REJECTED/WITHDRAWN). */
  async updateApplicationStatus(userId: string, applicationId: string, status: ApplicationStatus) {
    const employer = await this.requireEmployer(userId);
    const application = await this.prisma.application.findUnique({
      where: { id: applicationId },
      include: { vacancy: true },
    });
    if (!application || application.vacancy.employerId !== employer.id) {
      throw new ForbiddenException('Это не ваш отклик');
    }
    return this.prisma.application.update({ where: { id: applicationId }, data: { status } });
  }

  /** Мои отклики (кандидат). */
  async myApplications(userId: string) {
    const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
    if (!profile) throw new NotFoundException('Профиль кандидата не найден');

    const applications = await this.prisma.application.findMany({
      where: { candidateId: profile.id },
      orderBy: { createdAt: 'desc' },
      include: {
        vacancy: {
          include: { employer: { select: { id: true, companyName: true, logoUrl: true } } },
        },
      },
    });
    return { applications };
  }

  private async requireEmployer(userId: string) {
    const employer = await this.prisma.employerProfile.findUnique({ where: { userId } });
    if (!employer) throw new NotFoundException('Профиль работодателя не найден');
    return employer;
  }

  private async requireOwnVacancy(employerId: string, vacancyId: string) {
    const vacancy = await this.prisma.vacancy.findUnique({ where: { id: vacancyId } });
    if (!vacancy || vacancy.employerId !== employerId) {
      throw new ForbiddenException('Это не ваша вакансия');
    }
    return vacancy;
  }
}
