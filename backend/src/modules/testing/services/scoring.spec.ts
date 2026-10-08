import { Grade } from '@prisma/client';
import { AnswerStat, GRADE_THRESHOLDS, ScoringService } from './scoring.service';

describe('ScoringService (грейды и результат теста)', () => {
  const scoring = new ScoringService();

  describe('calculateGrade по порогам theta', () => {
    const cases: Array<[number, Grade]> = [
      [-4, Grade.JUNIOR],
      [-1.5, Grade.JUNIOR],
      [-1, Grade.MIDDLE],
      [-0.5, Grade.MIDDLE],
      [0, Grade.MIDDLE_PLUS],
      [0.49, Grade.MIDDLE_PLUS],
      [0.5, Grade.SENIOR],
      [0.99, Grade.SENIOR],
      [1, Grade.LEAD],
      [1.49, Grade.LEAD],
      [1.5, Grade.PRINCIPAL],
      [3, Grade.PRINCIPAL],
    ];

    it.each(cases)('theta=%p → %p', (theta, grade) => {
      expect(scoring.calculateGrade(theta)).toBe(grade);
    });

    it('нечисловое theta трактуется как 0', () => {
      expect(scoring.calculateGrade(Number.NaN)).toBe(Grade.MIDDLE_PLUS);
    });

    it('пороги соответствуют docs/modules/testing-engine.md', () => {
      const byGrade = Object.fromEntries(GRADE_THRESHOLDS.map((t) => [t.grade, t.min]));
      expect(byGrade[Grade.MIDDLE]).toBe(-1.0);
      expect(byGrade[Grade.MIDDLE_PLUS]).toBe(0.0);
      expect(byGrade[Grade.SENIOR]).toBe(0.5);
      expect(byGrade[Grade.LEAD]).toBe(1.0);
      expect(byGrade[Grade.PRINCIPAL]).toBe(1.5);
      expect(byGrade[Grade.JUNIOR]).toBe(-Infinity);
    });
  });

  describe('getRecommendation', () => {
    it('рассчитанный грейд выше объявленного и score >= 85 → UPGRADE', () => {
      expect(scoring.getRecommendation(Grade.MIDDLE, Grade.MIDDLE_PLUS, 85)).toBe('UPGRADE');
      expect(scoring.getRecommendation(Grade.JUNIOR, Grade.SENIOR, 95)).toBe('UPGRADE');
    });

    it('рассчитанный грейд выше объявленного, но score < 85 → PASS', () => {
      expect(scoring.getRecommendation(Grade.MIDDLE, Grade.SENIOR, 70)).toBe('PASS');
    });

    it('рейтинг совпал или выше → PASS', () => {
      expect(scoring.getRecommendation(Grade.MIDDLE, Grade.MIDDLE, 60)).toBe('PASS');
      expect(scoring.getRecommendation(Grade.MIDDLE, Grade.LEAD, 50)).toBe('PASS');
    });

    it('рассчитанный грейд ниже объявленного → FAIL', () => {
      expect(scoring.getRecommendation(Grade.SENIOR, Grade.MIDDLE, 90)).toBe('FAIL');
    });

    it('без объявленного грейда решает score (порог 50)', () => {
      expect(scoring.getRecommendation(null, Grade.JUNIOR, 50)).toBe('PASS');
      expect(scoring.getRecommendation(null, Grade.JUNIOR, 49.9)).toBe('FAIL');
    });
  });

  describe('scoreFromAnswers (взвешенный балл 0-100)', () => {
    it('пустая история → 0', () => {
      expect(scoring.scoreFromAnswers([])).toBe(0);
    });

    it('все верные → 100', () => {
      const answers: AnswerStat[] = [
        { isCorrect: true, difficulty: 0.2 },
        { isCorrect: true, difficulty: 0.8 },
      ];
      expect(scoring.scoreFromAnswers(answers)).toBe(100);
    });

    it('половина верных при равной сложности → около 50', () => {
      const answers: AnswerStat[] = [
        { isCorrect: true, difficulty: 0.5 },
        { isCorrect: false, difficulty: 0.5 },
      ];
      expect(scoring.scoreFromAnswers(answers)).toBeCloseTo(50, 5);
    });

    it('сложные вопросы весят больше простых', () => {
      const hardCorrect: AnswerStat[] = [
        { isCorrect: true, difficulty: 0.9 },
        { isCorrect: false, difficulty: 0.1 },
      ];
      const hardWrong: AnswerStat[] = [
        { isCorrect: false, difficulty: 0.9 },
        { isCorrect: true, difficulty: 0.1 },
      ];
      expect(scoring.scoreFromAnswers(hardCorrect)).toBeGreaterThan(scoring.scoreFromAnswers(hardWrong));
    });

    it('частичный балл текстового ответа используется вместо isCorrect', () => {
      const answers: AnswerStat[] = [{ isCorrect: false, difficulty: 0.5, score: 0.6 }];
      expect(scoring.scoreFromAnswers(answers)).toBeCloseTo(60, 5);
    });

    it('значение score ограничивается диапазоном [0, 1]', () => {
      expect(scoring.scoreFromAnswers([{ isCorrect: false, difficulty: 0.5, score: 42 }])).toBe(100);
      expect(scoring.scoreFromAnswers([{ isCorrect: true, difficulty: 0.5, score: -3 }])).toBe(0);
    });
  });

  describe('detailsByDifficulty', () => {
    it('распределяет ответы по корзинам сложности', () => {
      const answers: AnswerStat[] = [
        { isCorrect: true, difficulty: 0.2 },
        { isCorrect: false, difficulty: 0.35 },
        { isCorrect: true, difficulty: 0.5 },
        { isCorrect: true, difficulty: 0.65 },
        { isCorrect: false, difficulty: 0.7 },
        { isCorrect: true, difficulty: 0.95 },
      ];
      const details = scoring.detailsByDifficulty(answers);
      expect(details.easy).toEqual({ correct: 1, total: 2 });
      expect(details.medium).toEqual({ correct: 2, total: 2 });
      expect(details.hard).toEqual({ correct: 1, total: 2 });
    });
  });

  describe('gradeIndex', () => {
    it('задаёт строгий порядок грейдов', () => {
      expect(scoring.gradeIndex(Grade.JUNIOR)).toBe(0);
      expect(scoring.gradeIndex(Grade.MIDDLE)).toBe(1);
      expect(scoring.gradeIndex(Grade.PRINCIPAL)).toBe(5);
      expect(scoring.gradeIndex(Grade.SENIOR)).toBeGreaterThan(scoring.gradeIndex(Grade.MIDDLE));
    });
  });
});
