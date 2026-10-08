import { Injectable } from '@nestjs/common';
import { PrismaService } from '../../../prisma/prisma.module';

export interface PoolQuestion {
  id: string;
  questionType: string;
  text: string;
  codeSnippet: string | null;
  options: Array<{ id: string; text: string; isCorrect?: boolean }> | null;
  correctAnswer: string | null;
  explanation: string | null;
  difficulty: number;
  discrimination: number;
  guessing: number;
  poolTag: string | null;
  isUnique: boolean;
}

/** Банк вопросов: загрузка пула по категории специализации. */
@Injectable()
export class QuestionPoolService {
  constructor(private readonly prisma: PrismaService) {}

  async poolFor(categoryId: string): Promise<PoolQuestion[]> {
    const questions = await this.prisma.testQuestion.findMany({
      where: { categoryId },
      select: {
        id: true,
        questionType: true,
        text: true,
        codeSnippet: true,
        options: true,
        correctAnswer: true,
        explanation: true,
        difficulty: true,
        discrimination: true,
        guessing: true,
        poolTag: true,
        isUnique: true,
      },
      orderBy: { id: 'asc' },
    });

    return questions.map((q) => ({
      ...q,
      options: (q.options as PoolQuestion['options']) ?? null,
    }));
  }

  async categoryForSpecialization(specialization: string) {
    const bySpec = await this.prisma.testCategory.findFirst({
      where: { specialization: specialization as any },
    });
    if (bySpec) return bySpec;
    return this.prisma.testCategory.findFirst({ orderBy: { name: 'asc' } });
  }
}
