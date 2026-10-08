import { Injectable, Logger } from '@nestjs/common';
import { Specialization, Grade } from '@prisma/client';

export type TaskType = 'multiple_choice' | 'code_execution' | 'llm_open' | 'code_review' | 'system_design';

export interface TestTask {
  id: string;
  type: TaskType;
  difficulty: number;  // 0-1
  points: number;
  title: string;
  description: string;
  timeLimitSeconds: number;

  // Type-specific
  multipleChoice?: {
    question: string;
    options: Array<{ id: string; text: string; isCorrect: boolean }>;
    explanation?: string;
  };

  codeExecution?: {
    question: string;
    starterCode: string;
    language: 'javascript' | 'python' | 'go';
    sampleTests: Array<{ input: string; expectedOutput: string }>;
    hiddenTestsCount: number;
  };

  llmOpen?: {
    question: string;
    evaluationCriteria: Array<{ name: string; weight: number; description?: string }>;
    keyConcepts: string[];
    referenceAnswer?: string;
  };

  codeReview?: {
    question: string;
    buggyCode: string;
    language: string;
    expectedBugCount: number;
    bugDescriptions: string[];
  };

  systemDesign?: {
    question: string;
    constraints: string[];
    evaluationCriteria: Array<{ name: string; weight: number; description?: string }>;
  };
}

export interface TaskDistribution {
  multipleChoice: number;
  codeExecution: number;
  llmOpen: number;
  codeReview: number;
  systemDesign: number;
}

/**
 * V2: Task Generator.
 *
 * Generates multi-format tasks for testing.
 * Distribution varies by grade.
 */
@Injectable()
export class TaskGeneratorService {
  private readonly logger = new Logger('TaskGenerator');

  // Task distribution by grade (percentages)
  private readonly distributions: Record<string, TaskDistribution> = {
    JUNIOR: {
      multipleChoice: 40,
      codeExecution: 30,
      llmOpen: 20,
      codeReview: 10,
      systemDesign: 0,
    },
    MIDDLE: {
      multipleChoice: 20,
      codeExecution: 35,
      llmOpen: 25,
      codeReview: 15,
      systemDesign: 5,
    },
    MIDDLE_PLUS: {
      multipleChoice: 15,
      codeExecution: 35,
      llmOpen: 25,
      codeReview: 15,
      systemDesign: 10,
    },
    SENIOR: {
      multipleChoice: 10,
      codeExecution: 35,
      llmOpen: 25,
      codeReview: 15,
      systemDesign: 15,
    },
    LEAD: {
      multipleChoice: 5,
      codeExecution: 30,
      llmOpen: 30,
      codeReview: 15,
      systemDesign: 20,
    },
    PRINCIPAL: {
      multipleChoice: 5,
      codeExecution: 25,
      llmOpen: 30,
      codeReview: 20,
      systemDesign: 20,
    },
  };

  /**
   * Get task distribution for a grade.
   */
  getDistribution(grade: Grade): TaskDistribution {
    return this.distributions[grade] || this.distributions.MIDDLE;
  }

  /**
   * Calculate points per task type based on difficulty.
   */
  getPoints(type: TaskType, difficulty: number): number {
    const basePoints: Record<TaskType, number> = {
      multiple_choice: 10,
      code_execution: 25,
      llm_open: 20,
      code_review: 30,
      system_design: 40,
    };

    // Higher difficulty = more points
    const multiplier = 0.5 + difficulty * 1.0;  // 0.5x to 1.5x
    return Math.round(basePoints[type] * multiplier);
  }

  /**
   * Create task template from existing question data.
   * Converts multiple_choice to appropriate type based on distribution.
   */
  createTaskFromQuestion(
    question: {
      id: string;
      questionType: string;
      text: string;
      codeSnippet?: string;
      options?: unknown;
      correctAnswer?: string;
      difficulty: number;
      poolTag?: string;
    },
    grade: Grade,
    index: number,
  ): TestTask | null {
    const distribution = this.getDistribution(grade);
    const totalTasks = 20;  // Default test size

    // Determine task type based on distribution and index
    const taskType = this.selectTaskType(distribution, index, totalTasks);

    switch (taskType) {
      case 'multiple_choice':
        return this.createMultipleChoiceTask(question, index);

      case 'code_execution':
        return this.createCodeExecutionTask(question, index);

      case 'llm_open':
        return this.createLLMOpenTask(question, index);

      case 'code_review':
        return this.createCodeReviewTask(question, index);

      case 'system_design':
        return this.createSystemDesignTask(question, index);

      default:
        return this.createMultipleChoiceTask(question, index);
    }
  }

  private selectTaskType(
    distribution: TaskDistribution,
    index: number,
    totalTasks: number,
  ): TaskType {
    // Calculate how many tasks of each type
    const counts = {
      multiple_choice: Math.round((distribution.multipleChoice / 100) * totalTasks),
      code_execution: Math.round((distribution.codeExecution / 100) * totalTasks),
      llm_open: Math.round((distribution.llmOpen / 100) * totalTasks),
      code_review: Math.round((distribution.codeReview / 100) * totalTasks),
      system_design: Math.round((distribution.systemDesign / 100) * totalTasks),
    };

    // Distribute tasks round-robin based on counts
    let remaining = index;
    const types: TaskType[] = [];

    for (let i = 0; i < counts.multiple_choice; i++) types.push('multiple_choice');
    for (let i = 0; i < counts.code_execution; i++) types.push('code_execution');
    for (let i = 0; i < counts.llm_open; i++) types.push('llm_open');
    for (let i = 0; i < counts.code_review; i++) types.push('code_review');
    for (let i = 0; i < counts.system_design; i++) types.push('system_design');

    // Shuffle deterministically based on index
    const shuffled = this.shuffleDeterministic(types, index);

    return shuffled[index % shuffled.length] || 'multiple_choice';
  }

  private shuffleDeterministic<T>(array: T[], seed: number): T[] {
    const result = [...array];
    let state = seed;
    for (let i = result.length - 1; i > 0; i--) {
      state = (state * 1103515245 + 12345) & 0x7fffffff;
      const j = state % (i + 1);
      [result[i], result[j]] = [result[j], result[i]];
    }
    return result;
  }

  private createMultipleChoiceTask(
    question: { id: string; text: string; options?: unknown; correctAnswer?: string; difficulty: number },
    index: number,
  ): TestTask | null {
    if (!question.options) return null;

    const options = question.options as Array<{ id: string; text: string; isCorrect: boolean }>;

    return {
      id: question.id,
      type: 'multiple_choice',
      difficulty: question.difficulty,
      points: this.getPoints('multiple_choice', question.difficulty),
      title: `Вопрос ${index + 1}`,
      description: question.text,
      timeLimitSeconds: 120,
      multipleChoice: {
        question: question.text,
        options,
      },
    };
  }

  private createCodeExecutionTask(
    question: { id: string; text: string; codeSnippet?: string; difficulty: number },
    index: number,
  ): TestTask {
    return {
      id: question.id,
      type: 'code_execution',
      difficulty: question.difficulty,
      points: this.getPoints('code_execution', question.difficulty),
      title: `Задача с кодом ${index + 1}`,
      description: question.text,
      timeLimitSeconds: 300,  // 5 minutes
      codeExecution: {
        question: question.text,
        starterCode: question.codeSnippet || '// Ваш код здесь\nfunction solution(input) {\n  // TODO\n}\n',
        language: this.detectLanguage(question.codeSnippet),
        sampleTests: [
          { input: 'test', expectedOutput: 'test' },  // Will be customized
        ],
        hiddenTestsCount: 5,
      },
    };
  }

  private createLLMOpenTask(
    question: { id: string; text: string; difficulty: number },
    index: number,
  ): TestTask {
    return {
      id: question.id,
      type: 'llm_open',
      difficulty: question.difficulty,
      points: this.getPoints('llm_open', question.difficulty),
      title: `Развернутый ответ ${index + 1}`,
      description: question.text,
      timeLimitSeconds: 300,
      llmOpen: {
        question: question.text,
        evaluationCriteria: [
          { name: 'accuracy', weight: 0.4, description: 'Точность технических утверждений' },
          { name: 'completeness', weight: 0.3, description: 'Полнота ответа' },
          { name: 'structure', weight: 0.2, description: 'Структурированность' },
          { name: 'examples', weight: 0.1, description: 'Примеры кода/практики' },
        ],
        keyConcepts: this.extractKeyConcepts(question.text),
      },
    };
  }

  private createCodeReviewTask(
    question: { id: string; text: string; codeSnippet?: string; difficulty: number },
    index: number,
  ): TestTask {
    return {
      id: question.id,
      type: 'code_review',
      difficulty: question.difficulty,
      points: this.getPoints('code_review', question.difficulty),
      title: `Code Review ${index + 1}`,
      description: question.text,
      timeLimitSeconds: 240,
      codeReview: {
        question: question.text,
        buggyCode: question.codeSnippet || '// Код с багами\nfunction buggy() {\n  // TODO\n}',
        language: this.detectLanguage(question.codeSnippet),
        expectedBugCount: 2,
        bugDescriptions: [],  // Will be filled from question data
      },
    };
  }

  private createSystemDesignTask(
    question: { id: string; text: string; difficulty: number },
    index: number,
  ): TestTask {
    return {
      id: question.id,
      type: 'system_design',
      difficulty: question.difficulty,
      points: this.getPoints('system_design', question.difficulty),
      title: `Проектирование системы ${index + 1}`,
      description: question.text,
      timeLimitSeconds: 600,  // 10 minutes
      systemDesign: {
        question: question.text,
        constraints: [],
        evaluationCriteria: [
          { name: 'architecture', weight: 0.3, description: 'Выбор архитектуры' },
          { name: 'scalability', weight: 0.25, description: 'Масштабируемость' },
          { name: 'tradeoffs', weight: 0.25, description: 'Осознанные компромиссы' },
          { name: 'clarity', weight: 0.2, description: 'Ясность объяснения' },
        ],
      },
    };
  }

  private detectLanguage(codeSnippet?: string): 'javascript' | 'python' | 'go' {
    if (!codeSnippet) return 'javascript';

    const lower = codeSnippet.toLowerCase();
    if (lower.includes('def ') || lower.includes('import ') || lower.includes('print(')) {
      return 'python';
    }
    if (lower.includes('func ') || lower.includes('package ') || lower.includes('fmt.')) {
      return 'go';
    }
    return 'javascript';
  }

  private extractKeyConcepts(text: string): string[] {
    // Simple keyword extraction
    const concepts: string[] = [];
    const conceptKeywords = [
      'garbage collection', 'memory', 'concurrency', 'thread', 'async',
      'database', 'index', 'query', 'transaction', 'acid',
      'api', 'rest', 'graphql', 'microservices', 'monolith',
      'cache', 'redis', 'memcached',
      'queue', 'kafka', 'rabbitmq',
      'docker', 'kubernetes', 'container',
      'ci/cd', 'devops', 'deployment',
    ];

    const lower = text.toLowerCase();
    for (const keyword of conceptKeywords) {
      if (lower.includes(keyword)) {
        concepts.push(keyword);
      }
    }

    return concepts;
  }
}