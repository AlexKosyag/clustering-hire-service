import { Specialization } from '@prisma/client';
import { BANKS_A } from './banks-a';
import { BANKS_B } from './banks-b';
import { makeRng, shuffle } from './rng';

const BANKS: Record<string, any> = { ...BANKS_A, ...BANKS_B };

export const SPECIALIZATIONS: Specialization[] = [
  Specialization.BACKEND,
  Specialization.FRONTEND,
  Specialization.FULLSTACK,
  Specialization.MOBILE,
  Specialization.DEVOPS,
  Specialization.DATA_SCIENCE,
  Specialization.QA,
  Specialization.SECURITY,
  Specialization.EMBEDDED,
  Specialization.GAME_DEV,
  Specialization.ML_AI,
  Specialization.DESIGN_UX,
  Specialization.MANAGEMENT,
];

export const SPEC_LABELS: Record<string, string> = {
  BACKEND: 'Backend-разработка',
  FRONTEND: 'Frontend-разработка',
  FULLSTACK: 'Fullstack-разработка',
  MOBILE: 'Мобильная разработка',
  DEVOPS: 'DevOps и инфраструктура',
  DATA_SCIENCE: 'Data Science',
  QA: 'Тестирование (QA)',
  SECURITY: 'Информационная безопасность',
  EMBEDDED: 'Встраиваемые системы',
  GAME_DEV: 'GameDev',
  ML_AI: 'Машинное обучение и ИИ',
  DESIGN_UX: 'UI/UX-дизайн',
  MANAGEMENT: 'IT-менеджмент',
};

export const CATEGORY_NAMES: Record<string, string> = {
  BACKEND: 'Backend Basic',
  FRONTEND: 'Frontend Basic',
  FULLSTACK: 'Fullstack Basic',
  MOBILE: 'Mobile Basic',
  DEVOPS: 'DevOps Basic',
  DATA_SCIENCE: 'Data Science Basic',
  QA: 'QA Basic',
  SECURITY: 'Security Basic',
  EMBEDDED: 'Embedded Basic',
  GAME_DEV: 'GameDev Basic',
  ML_AI: 'ML/AI Basic',
  DESIGN_UX: 'UI/UX Basic',
  MANAGEMENT: 'Management Basic',
};

/** Шаблоны вариативности: один и тот же концепт даёт 5 вопросов разной сложности. */
const STEM_TEMPLATES: Array<{ build: (term: string) => string; difficulty: number }> = [
  { build: (t) => `Что такое «${t}»?`, difficulty: 0.2 },
  { build: (t) => `Какое утверждение о «${t}» верно?`, difficulty: 0.45 },
  { build: (t) => `Выберите корректное определение понятия «${t}».`, difficulty: 0.55 },
  { build: (t) => `Как из перечисленных корректно описывается «${t}»?`, difficulty: 0.7 },
  { build: (t) => `Разберите варианты: какой из них верно характеризует «${t}»?`, difficulty: 0.8 },
];

const OPTION_IDS = ['a', 'b', 'c', 'd'];

export interface CategorySeed {
  key: string;
  name: string;
  description: string;
  specialization: Specialization;
}

export interface QuestionSeed {
  key: string; // ключ категории
  questionType: string;
  text: string;
  codeSnippet?: string;
  options?: Array<{ id: string; text: string; isCorrect: boolean }>;
  correctAnswer?: string;
  explanation: string;
  difficulty: number;
  discrimination: number;
  guessing: number;
  isUnique: boolean;
  poolTag: string;
}

function jitter(rng: () => number, base: number, spread = 0.05): number {
  const value = base + (rng() * 2 - 1) * spread;
  return Math.round(Math.min(0.95, Math.max(0.05, value)) * 100) / 100;
}

/** Генерирует категории и вопросы для всех 13 специализаций (~66 вопросов на специализацию). */
export function buildQuestionSeeds(): { categories: CategorySeed[]; questions: QuestionSeed[] } {
  const categories: CategorySeed[] = [];
  const questions: QuestionSeed[] = [];

  for (const spec of SPECIALIZATIONS) {
    const bank = BANKS[spec];
    if (!bank) throw new Error(`Нет банка вопросов для ${spec}`);

    const key = spec;
    categories.push({
      key,
      name: CATEGORY_NAMES[spec],
      description: `Адаптивный тест по специализации «${SPEC_LABELS[spec]}»: ${bank.concepts.length * 5 + 6} вопросов разной сложности`,
      specialization: spec,
    });

    const rng = makeRng(spec.length * 7919 + spec.charCodeAt(0) * 31 + 17);

    // 1. MCQ по концептам (12 концептов × 5 шаблонов = 60)
    bank.concepts.forEach((concept: any, conceptIndex: number) => {
      const poolTag = `${spec.toLowerCase()}_concept_${conceptIndex + 1}`;
      STEM_TEMPLATES.forEach((template, templateIndex) => {
        const correctText = concept.c as string;
        const wrongTexts = shuffle(rng, [...concept.w]) as string[];
        const texts = shuffle(rng, [correctText, ...wrongTexts.slice(0, 3)]);
        const correctIndex = texts.indexOf(correctText);

        questions.push({
          key,
          questionType: 'multiple_choice',
          text: template.build(concept.t),
          options: texts.map((text, index) => ({
            id: OPTION_IDS[index],
            text,
            isCorrect: index === correctIndex,
          })),
          correctAnswer: OPTION_IDS[correctIndex],
          explanation: concept.e,
          difficulty: jitter(rng, template.difficulty),
          discrimination: Math.round((0.9 + rng() * 0.8) * 100) / 100,
          guessing: 0.25,
          isUnique: templateIndex === 0 && conceptIndex % 3 === 0,
          poolTag,
        });
      });
    });

    // 2. Практические вопросы: 3 coding + 2 open_text + 1 scenario
    const textQuestions: Array<{ source: any; type: string }> = [
      ...(bank.coding || []).map((q: any) => ({ source: q, type: 'coding' })),
      ...(bank.open || []).map((q: any, i: number) => ({ source: q, type: i < 2 ? 'open_text' : 'scenario' })),
    ];

    textQuestions.forEach((item, index) => {
      const q = item.source;
      questions.push({
        key,
        questionType: item.type,
        text: q.text,
        codeSnippet: q.code,
        correctAnswer: q.correct,
        explanation: q.e,
        difficulty: item.type === 'coding' ? jitter(rng, 0.62) : jitter(rng, 0.5, 0.08),
        discrimination: Math.round((1.0 + rng() * 0.7) * 100) / 100,
        guessing: item.type === 'coding' ? 0.1 : 0.15,
        isUnique: true,
        poolTag: `${spec.toLowerCase()}_practice_${index + 1}`,
      });
    });
  }

  return { categories, questions };
}

export function countsByCategory(questions: QuestionSeed[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const question of questions) {
    counts[question.key] = (counts[question.key] || 0) + 1;
  }
  return counts;
}
