import { AntiCheatService } from './services/anti-cheat.service';
import { BehavioralAntiCheatService } from './services/behavioral-anti-cheat.service';
import { CATService } from './services/cat.service';
import { QuestionPoolService } from './services/question-pool.service';
import { ScoringService } from './services/scoring.service';
import { TaskGeneratorService } from './services/task-generator.service';
import { TestingServiceV2 } from './testing-v2.service';

// Mock @nestjs/axios to avoid ESM issues
jest.mock('@nestjs/axios', () => ({
  HttpService: class MockHttpService {
    get = jest.fn();
    post = jest.fn();
  },
}));

/**
 * Регресс на баг «409 Ответ на этот вопрос уже записан на втором вопросе» (Testing V2):
 * generateTask() раньше всегда передавала в CAT пустой список ответов, поэтому
 * следующая задача повторяла уже отвеченную, и её повторная отправка падала 409.
 * Проверяем: задачи не повторяются, поток start -> answer -> answer(nextTask)
 * проходит без 409, повторная отправка идемпотентна.
 */

type QuestionRow = {
  id: string;
  categoryId: string;
  questionType: string;
  text: string;
  codeSnippet: string | null;
  options: Array<{ id: string; text: string; isCorrect: boolean }> | null;
  correctAnswer: string | null;
  explanation: string | null;
  difficulty: number;
  discrimination: number;
  guessing: number;
  isUnique: boolean;
  poolTag: string | null;
};

const CATEGORY_ID = 'cat-1';

const mcq = (id: string, difficulty: number): QuestionRow => ({
  id,
  categoryId: CATEGORY_ID,
  questionType: 'multiple_choice',
  text: `Вопрос ${id}`,
  codeSnippet: null,
  options: [
    { id: 'a', text: 'Верный', isCorrect: true },
    { id: 'b', text: 'Неверный 1', isCorrect: false },
    { id: 'c', text: 'Неверный 2', isCorrect: false },
    { id: 'd', text: 'Неверный 3', isCorrect: false },
  ],
  correctAnswer: 'a',
  explanation: 'Пояснение',
  difficulty,
  discrimination: 1.2,
  guessing: 0.25,
  isUnique: false,
  poolTag: 'backend_concept',
});

const textQuestion = (id: string, difficulty: number): QuestionRow => ({
  id,
  categoryId: CATEGORY_ID,
  questionType: 'coding',
  text: `Практический вопрос ${id}`,
  codeSnippet: null,
  options: null,
  correctAnswer: 'SELECT|JOIN|ON',
  explanation: 'Пояснение',
  difficulty,
  discrimination: 1.3,
  guessing: 0.1,
  isUnique: true,
  poolTag: 'backend_practice',
});

// 12 вопросов: start() требует пул >= 10
const QUESTIONS: QuestionRow[] = [
  ...Array.from({ length: 12 }, (_, i) =>
    mcq(`q-${String(i + 1).padStart(2, '0')}`, Math.round((0.15 + i * 0.06) * 100) / 100),
  ),
  // вопросы банка без вариантов (как coding/open_text в сиде)
  textQuestion('q-13', 0.12),
  textQuestion('q-14', 0.95),
];

/** In-memory Prisma, повторяющая запросы TestingServiceV2/QuestionPoolService. */
function createV2Context(totalTasks = 5) {
  process.env.TESTING_TASKS_PER_TEST = String(totalTasks);

  const db = {
    categories: [{ id: CATEGORY_ID, name: 'Backend Basic', description: null, specialization: 'BACKEND' }],
    questions: QUESTIONS,
    sessions: [] as any[],
    answers: [] as any[],
    profiles: [
      {
        id: 'profile-1',
        userId: 'user-1',
        testCooldownUntil: null,
        declaredGrade: 'MIDDLE',
        verifiedGrade: null,
        specialization: 'BACKEND',
        testStatus: 'NOT_STARTED',
      },
    ],
  };

  const answerWithQuestion = (a: any) => ({
    ...a,
    question: db.questions.find((q) => q.id === a.questionId),
  });

  const prisma: any = {
    testCategory: {
      findFirst: async ({ where }: any = {}) =>
        db.categories.find((c) => !where || !('specialization' in where) || c.specialization === where.specialization) ??
        null,
    },
    testQuestion: {
      findMany: async ({ where, select, orderBy }: any) => {
        let list = db.questions.filter((q) => q.categoryId === where.categoryId);
        if (orderBy && 'id' in orderBy) {
          list = [...list].sort((a, b) => (orderBy.id === 'asc' ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id)));
        }
        return list.map((q) => {
          const row: Record<string, unknown> = {};
          for (const key of Object.keys(select)) row[key] = (q as any)[key];
          return row;
        });
      },
    },
    testSession: {
      findFirst: async ({ where }: any) =>
        db.sessions.find((s) => s.candidateId === where.candidateId && s.status === where.status) ?? null,
      findUnique: async ({ where, include }: any) => {
        const session: any = db.sessions.find((s) => s.id === where.id);
        if (!session) return null;
        if (!include?.answers) return { ...session };
        let answers = db.answers.filter((a) => a.sessionId === session.id).map(answerWithQuestion);
        if (include.answers.orderBy && 'answeredAt' in include.answers.orderBy) {
          const dir = include.answers.orderBy.answeredAt === 'asc' ? 1 : -1;
          answers = [...answers].sort((a: any, b: any) => dir * (a.answeredAt.getTime() - b.answeredAt.getTime()));
        }
        return { ...session, answers };
      },
      create: async ({ data }: any) => {
        const session = {
          id: `session-${db.sessions.length + 1}`,
          status: 'IN_PROGRESS',
          startedAt: new Date(),
          completedAt: null,
          timeLimitMinutes: 90,
          score: null,
          rawScore: null,
          maxScore: null,
          gradeBefore: null,
          gradeAfter: null,
          recommendation: null,
          cheatFlags: [],
          isSuspicious: false,
          currentDifficulty: 0.5,
          thetaEstimate: null,
          ...data,
        };
        db.sessions.push(session);
        return { ...session };
      },
      update: async ({ where, data }: any) => {
        const session: any = db.sessions.find((s) => s.id === where.id);
        Object.assign(session, data);
        return { ...session };
      },
    },
    testAnswer: {
      create: async ({ data }: any) => {
        const duplicate = db.answers.find((a) => a.sessionId === data.sessionId && a.questionId === data.questionId);
        if (duplicate) throw new Error('Unique constraint failed on (session_id, question_id)');
        const answer = { id: `answer-${db.answers.length + 1}`, answeredAt: new Date(), ...data };
        db.answers.push(answer);
        return { ...answer };
      },
    },
    candidateProfile: {
      findUnique: async ({ where }: any) => db.profiles.find((p) => p.userId === where.userId) ?? null,
      update: async ({ where, data }: any) => {
        const profile: any = db.profiles.find((p) => p.id === where.id);
        Object.assign(profile, data);
        return { ...profile };
      },
    },
  };

  const gamificationStub: any = {
    updateStreak: async () => undefined,
    awardXp: async () => undefined,
    awardBadge: async () => undefined,
    getProfile: async () => ({ xp: 100, level: 1, levelName: 'Новичок', streak: 1 }),
  };

  const service = new TestingServiceV2(
    prisma,
    new QuestionPoolService(prisma),
    new CATService(),
    new ScoringService(),
    new AntiCheatService(),
    new BehavioralAntiCheatService(),
    { executeWithTests: async () => ({ score: 80, passedTests: 4, totalTests: 5 }) } as any,
    {
      evaluate: async () => ({ score: 50, breakdown: {}, strengths: [], weaknesses: [], feedback: '' }),
    } as any,
    new TaskGeneratorService(),
    { invalidateCache: async () => undefined } as any,
    { indexCandidate: async () => undefined } as any,
    gamificationStub,
  );

  return { service, db };
}

/** Ответ клиента на задачу (multiple_choice в фикстурах; 'a' — верный вариант). */
function answerFor(task: any, correct = true) {
  return {
    questionId: task.id,
    selectedOptionId: correct ? 'a' : 'b',
    timeSpentSeconds: 10,
  } as any;
}

describe('TestingServiceV2 — поток ответов (регресс 409 «Ответ на этот вопрос уже записан»)', () => {
  it('следующая задача не совпадает с только что отвеченной (корневая причина 409)', async () => {
    const { service, db } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });

    const res = await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask));

    expect(res.nextTask).toBeTruthy();
    expect(res.nextTask!.id).not.toBe(start.firstTask!.id);
    expect(res.duplicate).toBe(false);
    expect(db.answers).toHaveLength(1);
  });

  it('ответ на nextTask из предыдущего ответа принимается без 409 (сценарий из баг-репорта)', async () => {
    const { service, db } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });

    const first = await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask));
    // раньше этот шаг падал ConflictException на «втором вопросе»
    const second = await service.submitAnswer('user-1', start.sessionId, answerFor(first.nextTask));

    expect(second.duplicate).toBe(false);
    expect(second.nextTask).toBeTruthy();
    expect(second.nextTask!.id).not.toBe(first.nextTask!.id);
    expect(db.answers).toHaveLength(2);
    expect(new Set(db.answers.map((a: any) => a.questionId)).size).toBe(2);
  });

  it('повторная отправка ответа идемпотентна и не блокирует тест', async () => {
    const { service, db } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });

    const first = await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask));
    const repeat = await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask));

    expect(repeat.duplicate).toBe(true);
    expect(repeat.correct).toBe(first.correct);
    expect(repeat.score).toBe(first.score);
    expect(repeat.tasksRemaining).toBe(first.tasksRemaining);
    expect(repeat.nextTask!.id).toBe(first.nextTask!.id);
    expect(db.answers).toHaveLength(1);

    // тест продолжается, а не застревает на 409
    const next = await service.submitAnswer('user-1', start.sessionId, answerFor(repeat.nextTask));
    expect(next.duplicate).toBe(false);
    expect(db.answers).toHaveLength(2);
  });

  it('повторная отправка не перезаписывает записанный ответ (первый выигрывает)', async () => {
    const { service, db } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });

    await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask, true));
    const repeat = await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask, false));

    expect(repeat.correct).toBe(true); // остаётся результат первой записи
    expect(db.answers).toHaveLength(1);
    expect(db.answers[0].isCorrect).toBe(true);
  });

  it('currentTask не возвращает отвеченную задачу', async () => {
    const { service } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });
    await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask));

    const current = await service.currentTask('user-1', start.sessionId);

    expect(current.task).toBeTruthy();
    expect(current.task!.id).not.toBe(start.firstTask!.id);
    expect((current as any).taskNumber).toBe(2);
  });

  it('selectedOptionId учитывается при оценке multiple_choice', async () => {
    const { service } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });

    const correct = await service.submitAnswer('user-1', start.sessionId, answerFor(start.firstTask, true));
    expect(correct.correct).toBe(true);
    expect(correct.score).toBe(1);

    const wrong = await service.submitAnswer('user-1', start.sessionId, answerFor(correct.nextTask, false));
    expect(wrong.correct).toBe(false);
    expect(wrong.score).toBe(0);
  });

  it('ответ оценивается по формату показанной задачи, а не по типу исходного вопроса', async () => {
    const { service } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });
    const task = start.firstTask!;

    // Викторина показана не как multiple_choice: клиент заполняет поля своего формата,
    // оценщик не должен обнулять их «оценщиком вариантов».
    if (task.type === 'multiple_choice') {
      expect(task.multipleChoice!.options.length).toBeGreaterThan(0);
      return;
    }

    const payload: any = {
      questionId: task.id,
      timeSpentSeconds: 10,
      answerText: 'Развёрнутый ответ по существу задачи',
    };
    if (task.type === 'code_execution') {
      payload.code = 'function solve() { return 42; }';
      delete payload.answerText;
    } else if (task.type === 'system_design') {
      payload.designDescription = payload.answerText;
    } else if (task.type === 'code_review') {
      payload.fixedCode = 'исправленный код';
    }

    const res = await service.submitAnswer('user-1', start.sessionId, payload);
    expect(res.duplicate).toBe(false);
    expect(res.score).toBeGreaterThan(0); // раньше всегда было 0 из-за выбора оценщика по типу исходного вопроса
  });

  it('задача из вопроса без вариантов не пропадает (нет null-задачи посреди теста)', () => {
    const generator = new TaskGeneratorService();
    for (let index = 0; index < 20; index++) {
      const task = generator.createTaskFromQuestion(
        { id: 'text-q', questionType: 'coding', text: 'Практический вопрос без вариантов', difficulty: 0.5 },
        'MIDDLE' as any,
        index,
      );
      expect(task).not.toBeNull(); // раньше на multiple_choice-слотах возвращался null
    }
  });

  it('nextTask не пропадает посреди теста, пока он не завершён', async () => {
    const { service } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });

    let task = start.firstTask;
    for (let step = 0; step < 20; step++) {
      const res = await service.submitAnswer('user-1', start.sessionId, answerFor(task));
      if (res.isFinished || res.tasksRemaining === 0) break;
      expect(res.nextTask).not.toBeNull();
      task = res.nextTask!;
    }
  });

  it('полный прогон фронтенд-потоком: все задачи уникальны, тест завершается', async () => {
    const { service, db } = createV2Context();
    const start = await service.start('user-1', { specialization: 'BACKEND' });

    const seen = new Set<string>();
    let task = start.firstTask;
    for (let step = 0; step < 20; step++) {
      expect(seen.has(task!.id)).toBe(false);
      seen.add(task!.id);
      const res = await service.submitAnswer('user-1', start.sessionId, answerFor(task));
      expect(res.duplicate).toBe(false);
      if (!res.nextTask) break;
      task = res.nextTask;
    }

    expect(seen.size).toBe(5); // TESTING_TASKS_PER_TEST = 5 в контексте
    expect(db.answers).toHaveLength(5);

    const result = await service.complete('user-1', start.sessionId);
    expect(['PASS', 'FAIL', 'UPGRADE']).toContain(result.recommendation);
    expect(result.status).toBe('COMPLETED');
  });
});
