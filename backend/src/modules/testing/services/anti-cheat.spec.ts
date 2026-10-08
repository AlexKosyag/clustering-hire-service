import { AntiCheatService, TimedAnswer } from './anti-cheat.service';
import { CATService, IrtAnswer } from './cat.service';

const answer = (partial: Partial<TimedAnswer> & { isCorrect?: boolean }): TimedAnswer =>
  ({
    timeSpentSeconds: 30,
    difficulty: 0.5,
    poolTag: 'backend_concept_1',
    ...partial,
  }) as TimedAnswer;

describe('AntiCheatService', () => {
  const service = new AntiCheatService();

  describe('analyze: слишком быстрые ответы', () => {
    it('кладёт флаг при >3 быстрых ответах на сложные вопросы', () => {
      const answers = [
        answer({ timeSpentSeconds: 2, difficulty: 0.8 }),
        answer({ timeSpentSeconds: 1, difficulty: 0.9 }),
        answer({ timeSpentSeconds: 3, difficulty: 0.7 }),
        answer({ timeSpentSeconds: 2, difficulty: 0.6 }),
        answer({ timeSpentSeconds: 40, difficulty: 0.8 }),
      ];
      const result = service.analyze(answers);
      expect(result.flags).toContain('MULTIPLE_TOO_FAST_ANSWERS');
    });

    it('не срабатывает на нормальном темпе', () => {
      const answers = Array.from({ length: 10 }, () => answer({ timeSpentSeconds: 30, difficulty: 0.8 }));
      const result = service.analyze(answers);
      expect(result.flags).not.toContain('MULTIPLE_TOO_FAST_ANSWERS');
      expect(result.isSuspicious).toBe(false);
    });
  });

  describe('analyze: подозрительный паттерн сложности', () => {
    it('детектит «лёгкие мимо, сложные идеально»', () => {
      const answers = [
        answer({ difficulty: 0.1, isCorrect: false }),
        answer({ difficulty: 0.2, isCorrect: false }),
        answer({ difficulty: 0.15, isCorrect: false }),
        answer({ difficulty: 0.8, isCorrect: true }),
        answer({ difficulty: 0.9, isCorrect: true }),
        answer({ difficulty: 0.85, isCorrect: true }),
      ];
      const result = service.analyze(answers);
      expect(result.flags).toContain('SUSPICIOUS_DIFFICULTY_PATTERN');
    });

    it('не срабатывает при обычном распределении верных ответов', () => {
      const answers = [
        answer({ difficulty: 0.1, isCorrect: true }),
        answer({ difficulty: 0.2, isCorrect: true }),
        answer({ difficulty: 0.15, isCorrect: true }),
        answer({ difficulty: 0.8, isCorrect: true }),
        answer({ difficulty: 0.9, isCorrect: false }),
        answer({ difficulty: 0.85, isCorrect: true }),
      ];
      expect(service.analyze(answers).flags).not.toContain('SUSPICIOUS_DIFFICULTY_PATTERN');
    });
  });

  describe('analyze: идеальный пул вопросов', () => {
    it('флаг при >5 идеальных ответах из одного пула', () => {
      const answers = Array.from({ length: 6 }, () =>
        answer({ poolTag: 'backend_concept_1', isCorrect: true }),
      );
      const result = service.analyze(answers);
      expect(result.flags.some((f) => f.startsWith('PERFECT_SCORE_POOL_'))).toBe(true);
      expect(result.flags).toContain('PERFECT_SCORE_POOL_BACKEND_CONCEPT_1');
    });

    it('нет флага при 5 и менее ответах в пуле', () => {
      const answers = Array.from({ length: 5 }, () =>
        answer({ poolTag: 'backend_concept_1', isCorrect: true }),
      );
      const result = service.analyze(answers);
      expect(result.flags.some((f) => f.startsWith('PERFECT_SCORE_POOL_'))).toBe(false);
    });
  });

  describe('analyze: волатильность theta', () => {
    it('флаг при >2 резких скачков', () => {
      const history = [0, 3, -1, 2.5, -2, 3.5, 0];
      const result = service.analyze([], history);
      expect(result.flags).toContain('THETA_VOLATILITY');
    });

    it('нет флага при стабильной оценке', () => {
      const history = [0, 0.2, 0.1, 0.3, 0.2, 0.4];
      expect(service.analyze([], history).flags).not.toContain('THETA_VOLATILITY');
    });
  });

  describe('оценка подозрительности', () => {
    it('isSuspicious только при двух и более флагах', () => {
      const single = service.analyze([
        answer({ timeSpentSeconds: 1, difficulty: 0.9 }),
        answer({ timeSpentSeconds: 1, difficulty: 0.9 }),
        answer({ timeSpentSeconds: 1, difficulty: 0.9 }),
        answer({ timeSpentSeconds: 1, difficulty: 0.9 }),
      ]);
      expect(single.flags.length).toBe(1);
      expect(single.isSuspicious).toBe(false);
      expect(single.confidence).toBeCloseTo(0.35, 2);
    });

    it('confidence ограничена единицей', () => {
      const history = [0, 3, -1, 2.5, -2, 3.5, 0];
      const fast = Array.from({ length: 4 }, () =>
        answer({ timeSpentSeconds: 1, difficulty: 0.9, poolTag: 'p1', isCorrect: true }),
      );
      const pool = Array.from({ length: 6 }, () =>
        answer({ timeSpentSeconds: 40, difficulty: 0.5, poolTag: 'p2', isCorrect: true }),
      );
      const result = service.analyze([...fast, ...pool], history);
      expect(result.flags.length).toBeGreaterThanOrEqual(3);
      expect(result.isSuspicious).toBe(true);
      expect(result.confidence).toBeLessThanOrEqual(1);
      expect(result.confidence).toBe(1);
    });

    it('чистая сессия не подозрительна', () => {
      const answers = Array.from({ length: 20 }, (_, i) =>
        answer({
          timeSpentSeconds: 20 + (i % 5),
          difficulty: 0.3 + (i % 4) * 0.2,
          poolTag: `concept_${i % 5}`,
          isCorrect: i % 3 !== 0,
        }),
      );
      const result = service.analyze(answers, [0, 0.1, -0.1, 0.2, 0.15]);
      expect(result.flags).toEqual([]);
      expect(result.isSuspicious).toBe(false);
      expect(result.confidence).toBe(0);
    });
  });

  describe('shuffleOptions', () => {
    it('перемешивает детерминированно и сохраняет состав', () => {
      const options = ['a', 'b', 'c', 'd'];
      const first = service.shuffleOptions('s1', 'q1', options);
      const second = service.shuffleOptions('s1', 'q1', options);
      expect(first).toEqual(second);
      expect([...first].sort()).toEqual([...options].sort());
      expect(first).not.toBe(options);
    });

    it('разные сессии дают разный порядок хотя бы для части вопросов', () => {
      const options = ['a', 'b', 'c', 'd', 'e'];
      const base = service.shuffleOptions('session-0', 'question-X', options).join();
      const differs = Array.from({ length: 20 }, (_, i) =>
        service.shuffleOptions(`session-${i + 1}`, 'question-X', options).join(),
      ).some((value) => value !== base);
      expect(differs).toBe(true);
    });

    it('не ломает пустой и одиночный списки', () => {
      expect(service.shuffleOptions('s', 'q', [])).toEqual([]);
      expect(service.shuffleOptions('s', 'q', ['only'])).toEqual(['only']);
    });
  });

  describe('thetaHistory', () => {
    it('строит оценку на каждом префиксе ответов', () => {
      const cat = new CATService();
      const answers: IrtAnswer[] = [
        { question: { id: 'q1', difficulty: 0.5, discrimination: 1, guessing: 0.25 }, isCorrect: true },
        { question: { id: 'q2', difficulty: 0.5, discrimination: 1, guessing: 0.25 }, isCorrect: true },
        { question: { id: 'q3', difficulty: 0.5, discrimination: 1, guessing: 0.25 }, isCorrect: false },
      ];
      const history = service.thetaHistory(answers, (prefix) => cat.estimateTheta(prefix));
      expect(history).toHaveLength(answers.length);
      expect(history.every((h) => Number.isFinite(h))).toBe(true);
      expect(history[0]).toBeGreaterThan(history[2]);
    });
  });
});
