import {
  BadRequestException,
  ConflictException,
  ForbiddenException,
  Injectable,
  Logger,
  NotFoundException,
} from '@nestjs/common';
import { CandidateProfile, Grade, SessionStatus, TestStatus } from '@prisma/client';
import { PrismaService } from '../../prisma/prisma.module';
import { MatchingServiceV2 } from '../matching/matching-v2.service';
import { IndexingService } from '../matching/indexing.service';
import { CATService } from './services/cat.service';
import { AntiCheatService } from './services/anti-cheat.service';
import { BehavioralAntiCheatService, BehavioralData } from './services/behavioral-anti-cheat.service';
import { CodeSandboxService } from './services/code-sandbox.service';
import { LLMEvaluationService } from './services/llm-evaluation.service';
import { TaskGeneratorService, TestTask } from './services/task-generator.service';
import { QuestionPoolService, PoolQuestion } from './services/question-pool.service';
import { AnswerStat, ScoringService } from './services/scoring.service';
import { StartTestDto, SubmitAnswerDto } from './dto/testing.dto';

// ─────────────────────────── V2 Types ───────────────────────────

export interface TaskAnswer {
  // Multiple choice
  selectedOptionId?: string;

  // Code execution
  code?: string;

  // LLM open
  answerText?: string;

  // Code review
  fixedCode?: string;
  bugsFound?: string[];
  explanation?: string;

  // System design
  designDescription?: string;
  diagram?: string;

  // Behavioral data (anti-cheat)
  behavioral?: BehavioralData;
}

export interface TaskResult {
  taskId: string;
  type: string;
  score: number;
  maxScore: number;
  isCorrect: boolean | null;

  codeExecution?: {
    passedTests: number;
    totalTests: number;
    score: number;
  };

  llmEvaluation?: {
    score: number;
    breakdown: Record<string, number>;
    strengths: string[];
    weaknesses: string[];
    feedback: string;
  };

  antiCheat?: {
    suspicionScore: number;
    flags: string[];
  };
}

export interface TestResultV2 {
  sessionId: string;
  status: string;
  score: number;
  gradeBefore: Grade | null;
  gradeAfter: Grade;
  recommendation: string;
  theta: number;

  // V2: Task breakdown by type
  taskBreakdown: Record<string, { total: number; passed: number; avgScore: number }>;

  // V2: Anti-cheat
  antiCheat: {
    isSuspicious: boolean;
    flags: string[];
    confidence: number;
    behavioralAnalysis: Record<string, unknown>;
  };

  cooldownUntil: Date;
}

// ─────────────────────────── Service ───────────────────────────

/**
 * Testing Engine V2.
 *
 * Key improvements over V1:
 * 1. Multi-format tasks (code execution, LLM open, code review, system design)
 * 2. Behavioral anti-cheat (keystroke, copy-paste, tab-switch)
 * 3. Code execution sandbox
 * 4. LLM-powered evaluation for open-text answers
 * 5. Task type distribution by grade
 */
@Injectable()
export class TestingServiceV2 {
  private readonly logger = new Logger('TestingV2');
  private readonly totalTasks: number;
  private readonly cooldownDays: number;

  constructor(
    private readonly prisma: PrismaService,
    private readonly pool: QuestionPoolService,
    private readonly cat: CATService,
    private readonly scoring: ScoringService,
    private readonly antiCheat: AntiCheatService,
    private readonly behavioralAntiCheat: BehavioralAntiCheatService,
    private readonly codeSandbox: CodeSandboxService,
    private readonly llmEvaluation: LLMEvaluationService,
    private readonly taskGenerator: TaskGeneratorService,
    private readonly matching: MatchingServiceV2,
    private readonly indexing: IndexingService,
  ) {
    this.totalTasks = parseInt(process.env.TESTING_TASKS_PER_TEST || '15', 10);
    this.cooldownDays = parseInt(process.env.TESTING_COOLDOWN_DAYS || '90', 10);
  }

  /**
   * V2: Start a test with multi-format tasks.
   */
  async start(userId: string, dto: StartTestDto) {
    const profile = await this.profileOf(userId);

    // Check cooldown
    if (profile.testCooldownUntil && profile.testCooldownUntil.getTime() > Date.now()) {
      throw new BadRequestException(
        `Пересдача возможна не раньше ${profile.testCooldownUntil.toISOString()}`,
      );
    }

    // Check for active session
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

    // Get category
    const category = await this.pool.categoryForSpecialization(dto.specialization);
    if (!category) throw new BadRequestException('Для этой специализации нет теста');

    // Get pool
    const poolQuestions = await this.pool.poolFor(category.id);
    if (poolQuestions.length < 10) {
      throw new BadRequestException('Банк вопросов неполный — тест временно недоступен');
    }

    const declared = dto.declaredGrade || profile.declaredGrade;

    // Create session
    const session = await this.prisma.testSession.create({
      data: {
        candidateId: profile.id,
        categoryId: category.id,
        gradeBefore: declared,
        timeLimitMinutes: 90,  // V2: Longer for multi-format
      },
    });

    await this.prisma.candidateProfile.update({
      where: { id: profile.id },
      data: { testStatus: TestStatus.IN_PROGRESS },
    });
    await this.matching.invalidateCache().catch(() => undefined);

    // V2: Generate first task
    const firstTask = this.generateTask(poolQuestions, profile, session.id, 0);

    return {
      sessionId: session.id,
      totalTasks: this.totalTasks,
      timeLimitMinutes: session.timeLimitMinutes,
      specialization: category.specialization,
      gradeBefore: declared,
      taskDistribution: this.taskGenerator.getDistribution(
        (declared || 'MIDDLE') as Grade,
      ),
      firstTask,
    };
  }

  /**
   * V2: Get current task.
   */
  async currentTask(userId: string, sessionId: string) {
    const { session, profile } = await this.loadOwned(userId, sessionId);

    if (session.status !== SessionStatus.IN_PROGRESS) {
      return { sessionId: session.id, status: session.status, task: null, isFinished: true };
    }
    if (this.isExpired(session)) {
      await this.prisma.testSession.update({
        where: { id: session.id },
        data: { status: SessionStatus.TIMED_OUT, completedAt: new Date() },
      });
      return { sessionId: session.id, status: SessionStatus.TIMED_OUT, task: null, isFinished: true };
    }

    const answered = session.answers;
    const poolQuestions = await this.pool.poolFor(session.categoryId);
    const taskIndex = answered.length;
    const task = this.generateTask(poolQuestions, profile, session.id, taskIndex);

    return {
      sessionId: session.id,
      status: session.status,
      taskNumber: taskIndex + 1,
      totalTasks: this.totalTasks,
      currentScore: this.scoring.scoreFromAnswers(this.toStats(answered)),
      timeRemainingSeconds: this.timeRemaining(session),
      isFinished: answered.length >= this.totalTasks,
      task,
    };
  }

  /**
   * V2: Submit answer with behavioral data.
   */
  async submitAnswer(userId: string, sessionId: string, dto: SubmitAnswerDto & { behavioral?: BehavioralData }) {
    const { session, profile } = await this.loadOwned(userId, sessionId);

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

    // Grade the answer based on question type
    const graded = await this.gradeAnswerV2(question, dto);

    // V2: Behavioral anti-cheat analysis
    let behavioralFlags: string[] = [];
    if (dto.behavioral) {
      const behavioralAnalysis = this.behavioralAntiCheat.analyzeQuestion(
        dto.behavioral,
        question.difficulty,
        (dto.answerText || '').length,
      );
      behavioralFlags = behavioralAnalysis.flags;
    }

    // Save answer
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

    // Update session
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

    const remaining = Math.max(0, this.totalTasks - answered.length);

    return {
      taskId: question.id,
      correct: graded.isCorrect,
      score: Math.round(graded.score * 100) / 100,
      explanation: question.explanation || null,
      currentScore: this.scoring.scoreFromAnswers(this.toStats(answered)),
      theta: Math.round(theta * 1000) / 1000,
      tasksRemaining: remaining,
      isFinished: remaining === 0,
      behavioralFlags,
      nextTask: remaining > 0 && nextQuestion
        ? this.generateTask(poolQuestions, profile, session.id, answered.length)
        : null,
    };
  }

  /**
   * V2: Complete test with enhanced anti-cheat and task breakdown.
   */
  async complete(userId: string, sessionId: string): Promise<TestResultV2> {
    const { session, profile } = await this.loadOwned(userId, sessionId);

    if (session.status === SessionStatus.COMPLETED) {
      return this.resultViewV2(session);
    }
    if (session.status !== SessionStatus.IN_PROGRESS) {
      throw new BadRequestException('Сессию нельзя завершить');
    }

    const expired = this.isExpired(session);
    if (session.answers.length < this.totalTasks && !expired) {
      throw new BadRequestException(
        `Осталось неотвеченных задач: ${this.totalTasks - session.answers.length}`,
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

    // V1 anti-cheat
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

    // V2: Behavioral anti-cheat (aggregate)
    const behavioralAnalyses = answered.map((a) =>
      this.behavioralAntiCheat.analyzeQuestion(
        {
          timeSpentSeconds: a.timeSpentSeconds || 0,
          copyPasteEvents: 0,  // Would come from client
          tabSwitches: 0,
          keystrokeCount: 0,
          idleTimeSeconds: 0,
        },
        a.question.difficulty,
        (a.answerText || '').length,
      ),
    );
    const behavioralSession = this.behavioralAntiCheat.analyzeSession(
      behavioralAnalyses,
      history,
    );

    // Merge flags
    const allFlags = [...new Set([...analysis.flags, ...behavioralSession.flags])];
    const isSuspicious = analysis.isSuspicious || behavioralSession.suspicionScore > 50;

    const cooldownUntil = new Date(Date.now() + this.cooldownDays * 86_400_000);
    const status: SessionStatus = expired ? SessionStatus.TIMED_OUT : SessionStatus.COMPLETED;
    const testStatus: TestStatus =
      recommendation === 'FAIL' ? TestStatus.FAILED
      : recommendation === 'UPGRADE' ? TestStatus.UPGRADED
      : TestStatus.PASSED;

    // Update session
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
        cheatFlags: allFlags,
        isSuspicious,
      },
    });

    // Update profile
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
    await this.indexing.indexCandidate(profile.id).catch((e) => this.logger.warn(`Indexing: ${e.message}`));

    // V2: Calculate task breakdown by type
    const taskBreakdown = this.calculateTaskBreakdown(answered);

    return {
      sessionId: session.id,
      status,
      score,
      gradeBefore: declaredGrade,
      gradeAfter,
      recommendation,
      theta: Math.round(theta * 1000) / 1000,
      taskBreakdown,
      antiCheat: {
        isSuspicious,
        flags: allFlags,
        confidence: Math.max(analysis.confidence, behavioralSession.confidence),
        behavioralAnalysis: behavioralSession.details,
      },
      cooldownUntil,
    };
  }

  // ─────────────────────────── V2 Helpers ───────────────────────────

  /**
   * Generate a task from pool question.
   */
  private generateTask(
    poolQuestions: PoolQuestion[],
    profile: CandidateProfile,
    sessionId: string,
    index: number,
  ): TestTask | null {
    // Use CAT to select next question
    const irtAnswers: Array<{ question: PoolQuestion; isCorrect: boolean }> = [];
    const selected = this.cat.selectNext(poolQuestions, irtAnswers, sessionId);

    if (!selected) return null;

    // Convert to V2 task
    const grade = (profile.verifiedGrade || profile.declaredGrade || 'MIDDLE') as Grade;
    return this.taskGenerator.createTaskFromQuestion(
      {
        id: selected.id,
        questionType: selected.questionType || 'multiple_choice',
        text: selected.text,
        codeSnippet: selected.codeSnippet ?? undefined,
        options: selected.options,
        correctAnswer: selected.correctAnswer ?? undefined,
        difficulty: selected.difficulty,
        poolTag: selected.poolTag ?? undefined,
      },
      grade,
      index,
    );
  }

  /**
   * Grade answer based on question type.
   */
  private async gradeAnswerV2(
    question: PoolQuestion,
    dto: SubmitAnswerDto & TaskAnswer,
  ): Promise<{ answerText: string | null; isCorrect: boolean | null; score: number }> {
    const questionType = question.questionType || 'multiple_choice';

    switch (questionType) {
      case 'code_execution':
        return this.gradeCodeExecution(question, dto);
      case 'llm_open':
        return this.gradeLLMOpen(question, dto);
      case 'code_review':
        return this.gradeCodeReview(question, dto);
      case 'system_design':
        return this.gradeSystemDesign(question, dto);
      case 'multiple_choice':
      default:
        // Fall back to V1 grading
        return this.gradeMultipleChoice(question, dto);
    }
  }

  private async gradeCodeExecution(
    question: PoolQuestion,
    dto: TaskAnswer,
  ): Promise<{ answerText: string | null; isCorrect: boolean | null; score: number }> {
    if (!dto.code) {
      return { answerText: null, isCorrect: false, score: 0 };
    }

    try {
      // Parse sample tests from question options
      const options = question.options as { sampleTests?: Array<{ input: string; expectedOutput: string }> } | null;
      const sampleTests = options?.sampleTests || [];

      const report = await this.codeSandbox.executeWithTests(
        dto.code,
        'javascript',  // Default, could be detected from code
        sampleTests,
        5,  // hidden tests
        { timeLimitMs: 10000 },
      );

      return {
        answerText: dto.code,
        isCorrect: report.score >= 60,  // Pass if 60%+ tests pass
        score: report.score / 100,  // Normalize to 0-1
      };
    } catch (e) {
      this.logger.warn(`Code execution failed: ${(e as Error).message}`);
      return { answerText: dto.code, isCorrect: false, score: 0 };
    }
  }

  private async gradeLLMOpen(
    question: PoolQuestion,
    dto: TaskAnswer,
  ): Promise<{ answerText: string | null; isCorrect: boolean | null; score: number }> {
    if (!dto.answerText) {
      return { answerText: null, isCorrect: false, score: 0 };
    }

    try {
      const options = question.options as {
        evaluationCriteria?: Array<{ name: string; weight: number; description?: string }>;
        keyConcepts?: string[];
      } | null;

      const result = await this.llmEvaluation.evaluate({
        question: question.text,
        candidateAnswer: dto.answerText,
        criteria: options?.evaluationCriteria || [
          { name: 'accuracy', weight: 0.4 },
          { name: 'completeness', weight: 0.3 },
          { name: 'structure', weight: 0.3 },
        ],
        keyConcepts: options?.keyConcepts || [],
      });

      return {
        answerText: dto.answerText,
        isCorrect: result.score >= 50,
        score: result.score / 100,
      };
    } catch (e) {
      this.logger.warn(`LLM evaluation failed: ${(e as Error).message}`);
      return { answerText: dto.answerText, isCorrect: false, score: 0 };
    }
  }

  private async gradeCodeReview(
    question: PoolQuestion,
    dto: TaskAnswer,
  ): Promise<{ answerText: string | null; isCorrect: boolean | null; score: number }> {
    if (!dto.fixedCode && !dto.bugsFound?.length) {
      return { answerText: null, isCorrect: false, score: 0 };
    }

    const options = question.options as {
      expectedBugCount?: number;
      bugDescriptions?: string[];
    } | null;

    const expectedBugs = options?.expectedBugCount || 2;
    const foundBugs = dto.bugsFound?.length || 0;

    // Score based on bugs found
    const bugScore = Math.min(1, foundBugs / expectedBugs) * 0.6;

    // Bonus for fix quality (simple check)
    const fixScore = dto.fixedCode && dto.fixedCode.length > 0 ? 0.4 : 0;

    const totalScore = (bugScore + fixScore) * 100;

    return {
      answerText: JSON.stringify({ bugsFound: dto.bugsFound, fixedCode: dto.fixedCode }),
      isCorrect: totalScore >= 50,
      score: totalScore / 100,
    };
  }

  private async gradeSystemDesign(
    question: PoolQuestion,
    dto: TaskAnswer,
  ): Promise<{ answerText: string | null; isCorrect: boolean | null; score: number }> {
    if (!dto.designDescription) {
      return { answerText: null, isCorrect: false, score: 0 };
    }

    // Use LLM evaluation
    const result = await this.llmEvaluation.evaluate({
      question: question.text,
      candidateAnswer: dto.designDescription,
      criteria: [
        { name: 'architecture', weight: 0.3 },
        { name: 'scalability', weight: 0.25 },
        { name: 'tradeoffs', weight: 0.25 },
        { name: 'clarity', weight: 0.2 },
      ],
      keyConcepts: [],
    });

    return {
      answerText: dto.designDescription,
      isCorrect: result.score >= 50,
      score: result.score / 100,
    };
  }

  private gradeMultipleChoice(
    question: PoolQuestion,
    dto: SubmitAnswerDto & TaskAnswer,
  ): { answerText: string | null; isCorrect: boolean | null; score: number } {
    const options = question.options as Array<{ id: string; isCorrect: boolean }> | null;
    if (!options || !dto.selectedOptionId) {
      return { answerText: dto.selectedOptionId || null, isCorrect: false, score: 0 };
    }

    const selected = options.find((o) => o.id === dto.selectedOptionId);
    const isCorrect = selected?.isCorrect === true;

    return {
      answerText: dto.selectedOptionId,
      isCorrect,
      score: isCorrect ? 1 : 0,
    };
  }

  private calculateTaskBreakdown(
    answered: Array<{ question: { questionType: string | null; difficulty: number }; isCorrect: boolean | null; score: number | null }>,
  ): Record<string, { total: number; passed: number; avgScore: number }> {
    const breakdown: Record<string, { total: number; passed: number; scores: number[] }> = {};

    for (const answer of answered) {
      const type = answer.question.questionType || 'multiple_choice';
      if (!breakdown[type]) {
        breakdown[type] = { total: 0, passed: 0, scores: [] };
      }
      breakdown[type].total++;
      if (answer.isCorrect) breakdown[type].passed++;
      if (answer.score != null) breakdown[type].scores.push(answer.score);
    }

    // Convert to final format
    const result: Record<string, { total: number; passed: number; avgScore: number }> = {};
    for (const [type, data] of Object.entries(breakdown)) {
      result[type] = {
        total: data.total,
        passed: data.passed,
        avgScore: data.scores.length > 0
          ? Math.round((data.scores.reduce((a, b) => a + b, 0) / data.scores.length) * 100)
          : 0,
      };
    }
    return result;
  }

  private resultViewV2(session: {
    id: string; status: string; score: number | null; gradeBefore: Grade | null;
    gradeAfter: Grade | null; recommendation: string | null; thetaEstimate: number | null;
  }): TestResultV2 {
    return {
      sessionId: session.id,
      status: session.status,
      score: session.score ?? 0,
      gradeBefore: session.gradeBefore,
      gradeAfter: session.gradeAfter ?? Grade.JUNIOR,
      recommendation: session.recommendation || 'FAIL',
      theta: session.thetaEstimate ?? 0,
      taskBreakdown: {},
      antiCheat: { isSuspicious: false, flags: [], confidence: 0, behavioralAnalysis: {} },
      cooldownUntil: new Date(),
    };
  }

  // ─────────────────────────── Shared Helpers ───────────────────────────

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
    })) as any;
    if (!session) throw new NotFoundException('Сессия не найдена');
    if (session.candidateId !== profile.id) throw new ForbiddenException('Это чужая сессия');
    return { session, profile };
  }

  private isExpired(session: { startedAt: Date; timeLimitMinutes: number }): boolean {
    const deadline = new Date(session.startedAt.getTime() + session.timeLimitMinutes * 60_000);
    return Date.now() > deadline.getTime();
  }

  private timeRemaining(session: { startedAt: Date; timeLimitMinutes: number }): number {
    const deadline = new Date(session.startedAt.getTime() + session.timeLimitMinutes * 60_000);
    return Math.max(0, Math.floor((deadline.getTime() - Date.now()) / 1000));
  }

  private toIrt(answers: Array<{ isCorrect: boolean | null; question: { id: string; difficulty: number; discrimination: number; guessing: number } }>) {
    return answers
      .filter((a) => a.isCorrect !== null)
      .map((a) => ({
        question: a.question,
        isCorrect: a.isCorrect === true,
      }));
  }

  private toStats(answers: Array<{ isCorrect: boolean | null; score: number | null; question: { difficulty: number } }>): AnswerStat[] {
    return answers.map((a) => ({
      isCorrect: a.isCorrect === true,
      difficulty: a.question.difficulty,
      score: a.score,
    }));
  }
}