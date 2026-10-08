import { BadRequestException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Grade, Specialization } from '@prisma/client';
import { PrismaService } from '../../prisma/prisma.module';
import { CreateVacancyDto, UpdateVacancyDto } from './dto/vacancy.dto';

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
