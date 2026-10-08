import { Grade, Specialization } from '@prisma/client';
import { ExplanationService } from './explanation.service';
import {
  MatchingScoringService,
  MatchScore,
  Need,
  normalizeTech,
  ScoreableCandidate,
  WEIGHTS,
} from './scoring.service';

const candidate = (overrides: Partial<ScoreableCandidate> = {}): ScoreableCandidate => ({
  specialization: Specialization.BACKEND,
  verifiedGrade: Grade.MIDDLE,
  declaredGrade: Grade.MIDDLE,
  stack: ['Node.js', 'PostgreSQL', 'Redis', 'Docker'],
  achievementPoints: 0,
  ...overrides,
});

const need = (overrides: Partial<Need> = {}): Need => ({
  specialization: Specialization.BACKEND,
  grade: Grade.MIDDLE,
  stack: ['Node.js', 'PostgreSQL'],
  ...overrides,
});

const score = (total: number, components: Partial<MatchScore['components']> = {}): MatchScore => ({
  total,
  components: {
    categoryMatch: 100,
    stackOverlap: 100,
    semanticSimilarity: 80,
    gradeMatch: 100,
    achievementBoost: 0,
    ...components,
  },
});

describe('MatchingScoringService', () => {
  const scoring = new MatchingScoringService();

  it('веса категорий дают ровно 1.0', () => {
    const sum = WEIGHTS.category + WEIGHTS.stack + WEIGHTS.semantic + WEIGHTS.grade + WEIGHTS.achievement;
    expect(sum).toBeCloseTo(1, 10);
  });

  it('идеальное совпадение даёт 100 баллов', () => {
    const result = scoring.calculateMatch(
      candidate({ achievementPoints: 10 }),
      need({ stack: ['Node.js', 'PostgreSQL'] }),
      1,
    );
    expect(result.total).toBeCloseTo(100, 1);
    expect(result.components.categoryMatch).toBe(100);
    expect(result.components.stackOverlap).toBe(100);
    expect(result.components.achievementBoost).toBe(100);
  });

  it('без достижений максимум 90 баллов (10% веса пустые)', () => {
    const result = scoring.calculateMatch(candidate(), need({ stack: ['Node.js', 'PostgreSQL'] }), 1);
    expect(result.total).toBeCloseTo(90, 1);
    expect(result.components.achievementBoost).toBe(0);
  });

  it('категория: совпадение 100, смежная 60, без специализации 50, несовпадение 0', () => {
    expect(scoring.categoryScore(candidate({ specialization: Specialization.BACKEND }), need({ specialization: Specialization.BACKEND }))).toBe(100);
    expect(scoring.categoryScore(candidate({ specialization: Specialization.DEVOPS }), need({ specialization: Specialization.BACKEND }))).toBe(60);
    expect(scoring.categoryScore(candidate({ specialization: Specialization.GAME_DEV }), need({ specialization: Specialization.BACKEND }))).toBe(0);
    expect(scoring.categoryScore(candidate(), { stack: [] })).toBe(50);
    expect(scoring.categoryScore(candidate({ specialization: null }), need())).toBe(30);
  });

  it('стек: доля пересечения с учётом регистра и пробелов', () => {
    expect(scoring.stackScore(['node js', 'POSTGRESQL'], ['Node.js', 'PostgreSQL'])).toBe(100);
    expect(scoring.stackScore(['Node.js'], ['Node.js', 'PostgreSQL', 'Redis'])).toBe(33);
    expect(scoring.stackScore([], ['Node.js'])).toBe(0);
    expect(scoring.stackScore(['Node.js'], [])).toBe(50);
  });

  it('грейд: точное совпадение 100, соседние 80/60, дальше 40/20', () => {
    expect(scoring.gradeScore(candidate({ verifiedGrade: Grade.MIDDLE }), need({ grade: Grade.MIDDLE }))).toBe(100);
    expect(scoring.gradeScore(candidate({ verifiedGrade: Grade.MIDDLE_PLUS }), need({ grade: Grade.MIDDLE }))).toBe(80);
    expect(scoring.gradeScore(candidate({ verifiedGrade: Grade.JUNIOR }), need({ grade: Grade.MIDDLE }))).toBe(60);
    expect(scoring.gradeScore(candidate({ verifiedGrade: Grade.SENIOR }), need({ grade: Grade.MIDDLE }))).toBe(40);
    expect(scoring.gradeScore(candidate({ verifiedGrade: Grade.MIDDLE }), need({ grade: Grade.PRINCIPAL }))).toBe(20);
    expect(scoring.gradeScore(candidate(), { stack: [] })).toBe(50);
    expect(scoring.gradeScore(candidate({ verifiedGrade: null, declaredGrade: null }), need())).toBe(30);
  });

  it('verifiedGrade важнее declaredGrade', () => {
    const result = scoring.calculateMatch(
      candidate({ verifiedGrade: Grade.SENIOR, declaredGrade: Grade.JUNIOR }),
      need({ grade: Grade.SENIOR }),
      1,
    );
    expect(result.components.gradeMatch).toBe(100);
  });

  it('достижения дают максимум 10 баллов компонента (очки × 10, cap 100)', () => {
    expect(scoring.achievementScore(0)).toBe(0);
    expect(scoring.achievementScore(5)).toBe(50);
    expect(scoring.achievementScore(1000)).toBe(100);
  });

  it('итог ограничен 0..100 и не содержит NaN при граничных входах', () => {
    const result = scoring.calculateMatch(
      candidate({ specialization: null, verifiedGrade: null, declaredGrade: null, stack: [] }),
      { stack: ['Node.js'], grade: Grade.PRINCIPAL },
      Number.NaN,
    );
    expect(Number.isFinite(result.total)).toBe(true);
    expect(result.total).toBeGreaterThanOrEqual(0);
    expect(result.total).toBeLessThanOrEqual(100);
  });

  it('normalizeTech убирает регистр и пробелы', () => {
    expect(normalizeTech(' Node JS ')).toBe('nodejs');
    expect(normalizeTech('NODEJS')).toBe('nodejs');
    expect(normalizeTech('')).toBe('');
  });
});

describe('ExplanationService', () => {
  const service = new ExplanationService();

  it('содержит детали по всем весомым факторам', () => {
    const explanation = service.generate(
      { ...candidate({ achievementPoints: 50 }), achievementNames: ['Хакатон «Цифровая экономика»'], lastTestDate: new Date(Date.now() - 10 * 86_400_000) },
      need(),
      score(85),
    );
    const weights = explanation.details.map((d) => d.weight);
    expect(weights).toContain(WEIGHTS.category);
    expect(weights).toContain(WEIGHTS.stack);
    expect(weights).toContain(WEIGHTS.grade);
    expect(weights).toContain(WEIGHTS.semantic);
    expect(weights).toContain(WEIGHTS.achievement);
    expect(explanation.overall.length).toBeGreaterThan(0);
  });

  it('без достижений и теста нет соответствующих блоков', () => {
    const explanation = service.generate(candidate({ achievementPoints: 0 }), need(), score(50));
    expect(explanation.details.some((d) => d.weight === WEIGHTS.achievement)).toBe(false);
    expect(explanation.details.some((d) => d.factor.toLowerCase().includes('тестирован'))).toBe(false);
  });

  it('перекрытие стека отражает реальные совпадения', () => {
    const explanation = service.generate(candidate(), need({ stack: ['Node.js', 'GraphQL'] }), score(60));
    const stackDetail = explanation.details.find((d) => d.weight === WEIGHTS.stack);
    expect(stackDetail?.overlap).toEqual(['Node.js']);
    expect(stackDetail?.match).toBe(true);
  });

  it('semantic-фактор зависит от порога 50%', () => {
    const high = service.generate(candidate(), need(), score(70, { semanticSimilarity: 80 }));
    const low = service.generate(candidate(), need(), score(70, { semanticSimilarity: 20 }));
    const factor = (e: { details: Array<{ weight: number; match: unknown }> }) =>
      e.details.find((d) => d.weight === WEIGHTS.semantic)?.match;
    expect(factor(high)).toBe(true);
    expect(factor(low)).toBe(false);
  });

  it('давность теста влияет на блок «тестирование»', () => {
    const fresh = service.generate(candidate(), need(), score(60));
    const withFresh = service.generate({ ...candidate(), lastTestDate: new Date(Date.now() - 5 * 86_400_000) }, need(), score(60));
    const withOld = service.generate({ ...candidate(), lastTestDate: new Date(Date.now() - 400 * 86_400_000) }, need(), score(60));
    expect(fresh.details.some((d) => d.weight === 0)).toBe(false);
    const block = (e: { details: Array<{ weight: number; match: unknown }> }) =>
      e.details.find((d) => d.weight === 0 && d.match !== undefined);
    expect(block(withFresh)?.match).toBe(true);
    expect(block(withOld)?.match).toBe(false);
  });

  it('summary разделяет уровни совпадения', () => {
    expect(service.summary(90)).not.toBe(service.summary(70));
    expect(service.summary(70)).not.toBe(service.summary(50));
    expect(service.summary(50)).not.toBe(service.summary(20));
    expect(service.summary(100).length).toBeGreaterThan(0);
  });

  it('общее совпадение разумно пересчитывается из компонентов', () => {
    const explanation = service.generate(candidate(), need(), score(45));
    expect(explanation.details.every((d) => typeof d.explanation === 'string' && d.explanation.length > 0)).toBe(true);
  });
});
