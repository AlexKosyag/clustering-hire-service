import {
  BadRequestException,
  ConflictException,
  ForbiddenException,
  Injectable,
  Logger,
  NotFoundException,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { CandidateProfile, Grade, SessionStatus, TestSession, TestStatus } from '@prisma/client';
import { IndexingService } from '../matching/indexing.service';
import { MatchingService } from '../matching/matching.service';
import { PrismaService } from '../../prisma/prisma.module';
import { IrtAnswer } from './services/cat.service';
import { CATService } from './services/cat.service';
import { AntiCheatService } from './services/anti-cheat.service';
import { PoolQuestion, QuestionPoolService } from './services/question-pool.service';
import { AnswerStat, ScoringService } from './services/scoring.service';
import { StartTestDto, SubmitAnswerDto } from './dto/testing.dto';

type SessionWithAnswers = TestSession & {
  answers: Array<{ questionId: string; isCorrect: boolean | null; score: number | null; timeSpentSeconds: number | null; answerText: string | null; answeredAt: Date; question: { id: string; difficulty: number; discrimination: number; guessing: number; poolTag: string | null } }>;
};

@Injectable()
export class TestingService {
  private readonly logger = new Logger('Testing');
  private readonly totalQuestions: number;
  private readonly cooldownDays: number;

  constructor(
    private readonly prisma: PrismaService,
    private readonly pool: QuestionPoolService,
    private readonly cat: CATService,
    private readonly scoring: ScoringService,
    private readonly antiCheat: AntiCheatService,
    private readonly matching: MatchingService,
    private readonly indexing: IndexingService,
    private readonly config: ConfigService,
  ) {
    this.totalQuestions = this.config.get<number>('testing.questionsPerTest') || 20;
    this.cooldownDays = this.config.get<number>('testing.cooldownDays') || 90;
  }

  async start(userId: string, dto: StartTestDto) {
    const profile = await this.profileOf(userId);

    if (profile.testCooldownUntil && profile.testCooldownUntil.getTime() > Date.now()) {
      throw new BadRequestException(
        `Пересдача возможна не раньше ${profile.testCooldownUntil.toISOString()}`,
      );
    }

    const active = await this.prisma.testSession.findFirst({
      where: { candidateId: profile.id, status: SessionStatus.IN_PROGRESS },
    });
    if (active) {
      if (!this.isExpired(active)) {
        throw new ConflictException('Тестирование уже начато');
      }
      await this.prisma.testSession.update({
        where: { id: active.id },
        data: { status: SessionStatus.TIMED_OUT, completedAt: new Date() },
      });
    }

    const category = await this.pool.categoryForSpecialization(dto.specialization);
    if (!category) throw new BadRequestException('Для этой специализации нет теста');

    const poolQuestions = await this.pool.poolFor(category.id);
    if (poolQuestions.length < this.totalQuestions) {
      throw new BadRequestException('Банк вопросов неполный — тест временно недоступен');
    }

    const declared = dto.declaredGrade || profile.declaredGrade;
    const session = await this.prisma.testSession.create({
      data: {
        candidateId: profile.id,
        categoryId: category.id,
        gradeBefore: declared,
        timeLimitMinutes: 60,
      },
    });

    await this.prisma.candidateProfile.update({
      where: { id: profile.id },
      data: { testStatus: TestStatus.IN_PROGRESS },
    });
    await this.matching.invalidateCache().catch(() => undefined);

    const first = this.cat.selectNext(poolQuestions, [], session.id);
    return {
      sessionId: session.id,
      totalQuestions: this.totalQuestions,
      timeLimitMinutes: session.timeLimitMinutes,
      specialization: category.specialization,
      gradeBefore: declared,
      firstQuestion: first ? this.viewQuestion(first, session.id) : null,
    };
  }

  async retake(userId: string, specialization: StartTestDto['specialization']) {
    return this.start(userId, { specialization });
  }

  /** Активная сессия (для восстановления теста после перезагрузки страницы). */
  async active(userId: string) {
    const profile = await this.profileOf(userId);
    const session = await this.prisma.testSession.findFirst({
      where: { candidateId: profile.id, status: SessionStatus.IN_PROGRESS },
      orderBy: { startedAt: 'desc' },
    });
    if (!session) return { session: null, canRetakeAt: profile.testCooldownUntil };
    if (this.isExpired(session)) {
      await this.prisma.testSession.update({
        where: { id: session.id },
        data: { status: SessionStatus.TIMED_OUT, completedAt: new Date() },
      });
      return { session: null, canRetakeAt: profile.testCooldownUntil };
    }
    return { session: { id: session.id, startedAt: session.startedAt }, canRetakeAt: profile.testCooldownUntil };
  }

  async current(userId: string, sessionId: string) {
    const { session, profile } = await this.loadOwned(userId, sessionId);

    if (session.status !== SessionStatus.IN_PROGRESS) {
      return { sessionId: session.id, status: session.status, question: null, isFinished: true };
    }
    if (this.isExpired(session)) {
      await this.prisma.testSession.update({
        where: { id: session.id },
        data: { status: SessionStatus.TIMED_OUT, completedAt: new Date() },
      });
      return { sessionId: session.id, status: SessionStatus.TIMED_OUT, question: null, isFinished: true };
    }

    const answered = session.answers;
    const irt = this.toIrt(answered);
    const poolQuestions = await this.pool.poolFor(session.categoryId);
    const next = this.cat.selectNext(poolQuestions, irt, session.id);

    return {
      sessionId: session.id,
      status: session.status,
      questionNumber: answered.length + 1,
      totalQuestions: this.totalQuestions,
      currentScore: this.scoring.scoreFromAnswers(this.toStats(answered)),
      timeRemainingSeconds: this.timeRemaining(session),
      isFinished: answered.length >= this.totalQuestions,
      question: next ? this.viewQuestion(next, session.id) : null,
    };
  }

  async answer(userId: string, sessionId: string, dto: SubmitAnswerDto) {
    const { session } = await this.loadOwned(userId, sessionId);

    if (session.status !== SessionStatus.IN_PROGRESS) {
      throw new BadRequestException('Сессия тестирования уже завершена');
    }
    if (this.isExpired(session)) {
      throw new BadRequestException('Время тестирования истекло');
    }
    if (session.answers.some((a) => a.questionId === dto.questionId)) {
      throw new ConflictException('Ответ на этот вопрос уже записан');
    }

    const poolQuestions = await this.pool.poolFor(session.categoryId);
    const question = poolQuestions.find((q) => q.id === dto.questionId);
    if (!question) throw new NotFoundException('Вопрос не найден в этом тесте');

    const graded = this.gradeAnswer(question, dto);

    await this.prisma.testAnswer.create({
      data: {
        sessionId: session.id,
        questionId: question.id,
        answerText: graded.answerText,
        isCorrect: graded.isCorrect,
        score: graded.score,
        timeSpentSeconds: dto.timeSpentSeconds ?? null,
      },
    });

    const updated = await this.prisma.testSession.findUnique({
      where: { id: session.id },
      include: { answers: { include: { question: true }, orderBy: { answeredAt: 'asc' } } },
    });
    const answered = updated!.answers;
    const irt = this.toIrt(answered);
    const theta = this.cat.estimateTheta(irt);
    const nextQuestion = this.cat.selectNext(poolQuestions, irt, session.id);

    await this.prisma.testSession.update({
      where: { id: session.id },
      data: {
        thetaEstimate: theta,
        currentDifficulty: nextQuestion?.difficulty ?? 1,
      },
    });

    const remaining = Math.max(0, this.totalQuestions - answered.length);

    return {
      correct: graded.isCorrect,
      score: Math.round(graded.score * 100) / 100,
      explanation: question.explanation || null,
      currentScore: this.scoring.scoreFromAnswers(this.toStats(answered)),
      theta: Math.round(theta * 1000) / 1000,
      questionsRemaining: remaining,
      isFinished: remaining === 0,
      nextQuestion: remaining > 0 && nextQuestion ? this.viewQuestion(nextQuestion, session.id) : null,
    };
  }

  async complete(userId: string, sessionId: string) {
    const { session, profile } = await this.loadOwned(userId, sessionId);

    if (session.status === SessionStatus.COMPLETED) {
      return this.resultView(session);
    }
    if (session.status !== SessionStatus.IN_PROGRESS) {
      throw new BadRequestException('Сессию нельзя завершить');
    }

    const expired = this.isExpired(session);
    if (session.answers.length < this.totalQuestions && !expired) {
      throw new BadRequestException(
        `Осталось неотвеченных вопросов: ${this.totalQuestions - session.answers.length}`,
      );
    }

    const answered = session.answers;
    const irt = this.toIrt(answered);
    const stats = this.toStats(answered);

    const theta = this.cat.estimateTheta(irt);
    const score = this.scoring.scoreFromAnswers(stats);
    const declaredGrade = session.gradeBefore ?? profile.declaredGrade;
    const gradeAfter = this.scoring.calculateGrade(theta, profile.specialization);
    const recommendation = this.scoring.getRecommendation(declaredGrade, gradeAfter, score);
    const details = this.scoring.detailsByDifficulty(stats);

    const history = this.antiCheat.thetaHistory(irt, (prefix) => this.cat.estimateTheta(prefix));
    const analysis = this.antiCheat.analyze(
      answered.map((a) => ({
        timeSpentSeconds: a.timeSpentSeconds,
        difficulty: a.question.difficulty,
        poolTag: a.question.poolTag,
        isCorrect: a.isCorrect === true,
      })),
      history,
    );

    const cooldownUntil = new Date(Date.now() + this.cooldownDays * 86_400_000);
    const status: SessionStatus = expired ? SessionStatus.TIMED_OUT : SessionStatus.COMPLETED;
    const testStatus: TestStatus =
      recommendation === 'FAIL' ? TestStatus.FAILED : recommendation === 'UPGRADE' ? TestStatus.UPGRADED : TestStatus.PASSED;

    const updated = await this.prisma.testSession.update({
      where: { id: session.id },
      data: {
        status,
        completedAt: new Date(),
        score,
        rawScore: score,
        maxScore: 100,
        gradeAfter,
        recommendation,
        thetaEstimate: theta,
        cheatFlags: analysis.flags,
        isSuspicious: analysis.isSuspicious,
      },
    });

    await this.prisma.candidateProfile.update({
      where: { id: profile.id },
      data: {
        verifiedGrade: gradeAfter,
        testStatus,
        lastTestDate: new Date(),
        testCooldownUntil: cooldownUntil,
        lastTestScore: score,
      },
    });

    await this.matching.invalidateCache().catch(() => undefined);
    await this.indexing.indexCandidate(profile.id).catch((e) => this.logger.warn(`Переиндексация: ${e.message}`));

    return {
      ...this.resultView(updated),
      details,
      cooldownUntil,
      theta: Math.round(theta * 1000) / 1000,
      antiCheat: { isSuspicious: analysis.isSuspicious, flags: analysis.flags, confidence: analysis.confidence },
    };
  }

  // ===== Вспомогательные методы =====

  private async profileOf(userId: string): Promise<CandidateProfile> {
    const profile = await this.prisma.candidateProfile.findUnique({ where: { userId } });
    if (!profile) throw new NotFoundException('Профиль кандидата не найден');
    return profile;
  }

  private async loadOwned(userId: string, sessionId: string) {
    const profile = await this.profileOf(userId);
    const session = (await this.prisma.testSession.findUnique({
      where: { id: sessionId },
      include: { answers: { include: { question: true }, orderBy: { answeredAt: 'asc' } } },
    })) as SessionWithAnswers | null;
    if (!session) throw new NotFoundException('Сессия не найдена');
    if (session.candidateId !== profile.id) throw new ForbiddenException('Это чужая сессия');
    return { session, profile };
  }

  private isExpired(session: TestSession): boolean {
    const deadline = session.startedAt.getTime() + session.timeLimitMinutes * 60_000;
    return Date.now() > deadline;
  }

  private timeRemaining(session: TestSession): number {
    const deadline = session.startedAt.getTime() + session.timeLimitMinutes * 60_000;
    return Math.max(0, Math.round((deadline - Date.now()) / 1000));
  }

  private toIrt(answers: SessionWithAnswers['answers']): IrtAnswer[] {
    return answers.map((a) => ({
      question: {
        id: a.question.id,
        difficulty: a.question.difficulty,
        discrimination: a.question.discrimination,
        guessing: a.question.guessing,
      },
      isCorrect: a.isCorrect === true,
    }));
  }

  private toStats(answers: SessionWithAnswers['answers']): AnswerStat[] {
    return answers.map((a) => ({
      isCorrect: a.isCorrect === true,
      difficulty: a.question.difficulty,
      score: a.score,
    }));
  }

  /** Вопрос для клиента: без правильного ответа и объяснения, варианты в сессионном порядке. */
  private viewQuestion(question: PoolQuestion, sessionId: string) {
    const options = (question.options || []).map((o) => ({ id: o.id, text: o.text }));
    return {
      id: question.id,
      text: question.text,
      codeSnippet: question.codeSnippet,
      questionType: question.questionType,
      options: this.antiCheat.shuffleOptions(sessionId, question.id, options),
      difficulty: question.difficulty,
    };
  }

  private gradeAnswer(question: PoolQuestion, dto: SubmitAnswerDto): { isCorrect: boolean; score: number; answerText: string } {
    if (question.questionType === 'multiple_choice') {
      const options = question.options || [];
      const selected = options.find((o) => o.id === dto.answer);
      if (!selected) throw new BadRequestException('Не выбран вариант ответа');
      const isCorrect = selected.isCorrect === true;
      return { isCorrect, score: isCorrect ? 1 : 0, answerText: selected.id };
    }

    const text = (dto.answerText || '').trim();
    if (!text) throw new BadRequestException('Ответ пустой');

    const reference = question.correctAnswer;
    if (!reference) {
      // Без эталона — частичные баллы за объём, но не засчитываем как верный ответ
      const score = Math.min(0.6, Math.round((text.length / 500) * 100) / 100);
      return { isCorrect: false, score, answerText: text };
    }

    const score = this.referenceCoverage(reference, text);
    return { isCorrect: score >= 0.6, score: Math.round(score * 100) / 100, answerText: text };
  }

  /**
   * Покрытие эталона: либо набор ключевых фраз через '|', либо пересечение токенов.
   * Детерминированно и не требует внешних моделей.
   */
  private referenceCoverage(reference: string, answer: string): number {
    const parts = reference.includes('|')
      ? reference.split('|').map((p) => normalize(p)).filter(Boolean)
      : [normalize(reference)];

    if (parts.length === 0) return 0;

    const answerTokens = new Set(tokenize(answer));
    let total = 0;
    for (const part of parts) {
      const referenceTokens = tokenize(part);
      if (referenceTokens.length === 0) continue;
      const hit = referenceTokens.filter((t) => answerTokens.has(t)).length;
      const coverage = hit / referenceTokens.length;
      const phraseHit = normalize(answer).includes(part) ? 1 : 0;
      total += Math.max(coverage, phraseHit);
    }
    return Math.max(0, Math.min(1, total / parts.length));
  }

  private resultView(session: TestSession & { answers?: unknown[] }) {
    return {
      sessionId: session.id,
      status: session.status,
      score: session.score,
      gradeBefore: session.gradeBefore,
      gradeAfter: session.gradeAfter,
      recommendation: session.recommendation,
      theta: session.thetaEstimate,
      isSuspicious: session.isSuspicious,
      cheatFlags: session.cheatFlags,
      completedAt: session.completedAt,
    };
  }
}

function normalize(value: string): string {
  return (value || '')
    .toLowerCase()
    .replace(/ё/g, 'е')
    .replace(/[^a-zа-я0-9]+/g, ' ')
    .trim();
}

function tokenize(value: string): string[] {
  return normalize(value).split(' ').filter((t) => t.length > 1);
}
