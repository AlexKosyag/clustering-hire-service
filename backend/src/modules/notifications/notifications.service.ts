import { Injectable, NotFoundException } from '@nestjs/common';
import { InvitationStatus, UserRole } from '@prisma/client';
import { PrismaService } from '../../prisma/prisma.module';

interface NotificationItem {
  id: string;
  type: 'INVITATION_RECEIVED' | 'INVITATION_STATUS' | 'APPLICATION_RECEIVED' | 'APPLICATION_STATUS' | 'TEST_RESULT';
  title: string;
  body: string;
  createdAt: Date;
  read: boolean;
}

/**
 * In-app уведомления (MVP): собираются из приглашений, откликов и результатов тестов.
 * Отдельная таблица уведомлений не нужна — источник истины это события предметной области.
 */
@Injectable()
export class NotificationsService {
  constructor(private readonly prisma: PrismaService) {}

  async list(userId: string, role: UserRole, limit = 20): Promise<{ items: NotificationItem[]; total: number }> {
    const items: NotificationItem[] = [];

    if (role === UserRole.CANDIDATE) {
      const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
      if (!profile) throw new NotFoundException('Профиль не найден');

      const invitations = await this.prisma.invitation.findMany({
        where: { candidateId: profile.id },
        orderBy: { statusUpdatedAt: 'desc' },
        take: 30,
        include: { employer: { select: { companyName: true } } },
      });

      for (const invitation of invitations) {
        if (invitation.status === InvitationStatus.SENT) {
          items.push({
            id: invitation.id,
            type: 'INVITATION_RECEIVED',
            title: `Новое приглашение от «${invitation.employer.companyName}»`,
            body: `Предложение: ${invitation.salaryFrom.toLocaleString('ru-RU')} — ${invitation.salaryTo.toLocaleString('ru-RU')} ₽`,
            createdAt: invitation.createdAt,
            read: false,
          });
        } else {
          const terminal: InvitationStatus[] = [
            InvitationStatus.ACCEPTED,
            InvitationStatus.REJECTED,
            InvitationStatus.WITHDRAWN,
          ];
          if (!terminal.includes(invitation.status)) continue;
          items.push({
            id: invitation.id,
            type: 'INVITATION_STATUS',
            title: `Приглашение: ${invitation.status}`,
            body: `«${invitation.employer.companyName}» — статус обновлён`,
            createdAt: invitation.statusUpdatedAt,
            read: true,
          });
        }
      }

      const session = await this.prisma.testSession.findFirst({
        where: { candidateId: profile.id, status: 'COMPLETED' as any },
        orderBy: { completedAt: 'desc' },
      });
      if (session?.completedAt) {
        items.push({
          id: session.id,
          type: 'TEST_RESULT',
          title: `Тестирование завершено: ${session.score} баллов`,
          body: `Грейд: ${session.gradeBefore ?? '—'} → ${session.gradeAfter}, рекомендация ${session.recommendation}`,
          createdAt: session.completedAt,
          read: true,
        });
      }
    } else {
      const employer = await this.prisma.employerProfile.findUnique({ where: { userId } });
      if (!employer) throw new NotFoundException('Профиль не найден');

      const invitations = await this.prisma.invitation.findMany({
        where: { employerId: employer.id, status: { not: InvitationStatus.SENT } },
        orderBy: { statusUpdatedAt: 'desc' },
        take: 30,
        include: { candidate: { select: { firstName: true, lastName: true } } },
      });

      for (const invitation of invitations) {
        const name = [invitation.candidate.firstName, invitation.candidate.lastName].filter(Boolean).join(' ') || 'Кандидат';
        items.push({
          id: invitation.id,
          type: 'INVITATION_STATUS',
          title: `${name}: ${invitation.status}`,
          body: invitation.status === InvitationStatus.ACCEPTED
            ? 'Кандидат принял приглашение — контакты открыты в профиле'
            : 'Статус приглашения обновлён',
          createdAt: invitation.statusUpdatedAt,
          read: invitation.status !== InvitationStatus.ACCEPTED,
        });
      }

      const applications = await this.prisma.application.findMany({
        where: { vacancy: { employerId: employer.id } },
        orderBy: { createdAt: 'desc' },
        take: 30,
        include: { vacancy: { select: { title: true } } },
      });
      for (const application of applications) {
        items.push({
          id: application.id,
          type: 'APPLICATION_RECEIVED',
          title: `Новый отклик: ${application.vacancy.title}`,
          body: `Статус: ${application.status}`,
          createdAt: application.createdAt,
          read: application.status !== 'PENDING',
        });
      }
    }

    items.sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime());
    const total = items.length;
    return { items: items.slice(0, limit), total };
  }
}
