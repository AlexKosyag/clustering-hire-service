import { CATService, IrtAnswer, IrtQuestion, sessionOrderKey } from './cat.service';

const q = (id: string, difficulty: number, discrimination = 1, guessing = 0.25): IrtQuestion => ({
  id,
  difficulty,
  discrimination,
  guessing,
});

describe('CATService (3PL IRT)', () => {
  const cat = new CATService();

  describe('probability', () => {
    it('на трудности вопроса даёт c + (1-c)/2', () => {
      const p = cat.probability(0.5, 1, 0.5, 0.25);
      expect(p).toBeCloseTo(0.25 + 0.75 / 2, 6);
    });

    it('монотонно растёт с ростом theta', () => {
      const low = cat.probability(-2, 1, 0, 0.25);
      const mid = cat.probability(0, 1, 0, 0.25);
      const high = cat.probability(2, 1, 0, 0.25);
      expect(low).toBeLessThan(mid);
      expect(mid).toBeLessThan(high);
    });

    it('защищает от некорректных параметров', () => {
      expect(cat.probability(NaN, NaN, NaN, NaN)).toBeGreaterThan(0);
      expect(cat.probability(NaN, NaN, NaN, NaN)).toBeLessThan(1);
      expect(cat.probability(0, 1, 0, 5)).toBeLessThanOrEqual(1 - 1e-6);
    });
  });

  describe('fisherInformation', () => {
    it('информационная функция максимальна около сложности вопроса', () => {
      const atDifficulty = cat.fisherInformation(0, q('a', 0));
      const farAway = cat.fisherInformation(3, q('a', 0));
      expect(atDifficulty).toBeGreaterThan(0);
      expect(atDifficulty).toBeGreaterThan(farAway);
    });

    it('всегда конечна и неотрицательна', () => {
      const info = cat.fisherInformation(100, q('a', -5, 3, 0.1));
      expect(Number.isFinite(info)).toBe(true);
      expect(info).toBeGreaterThanOrEqual(0);
    });
  });

  describe('estimateTheta', () => {
    const answers = (correct: boolean, count = 10): IrtAnswer[] =>
      Array.from({ length: count }, (_, i) => ({ question: q(`q${i}`, (i % 5) * 0.2), isCorrect: correct }));

    it('все верные ответы дают высокий theta', () => {
      expect(cat.estimateTheta(answers(true))).toBeGreaterThan(1);
    });

    it('все неверные ответы дают низкий theta', () => {
      expect(cat.estimateTheta(answers(false))).toBeLessThan(-1);
    });

    it('theta ограничен [-4, 4]', () => {
      const theta = cat.estimateTheta(answers(true, 30));
      expect(theta).toBeLessThanOrEqual(4);
      expect(theta).toBeGreaterThanOrEqual(-4);
    });

    it('пустая история даёт 0', () => {
      expect(cat.estimateTheta([])).toBe(0);
    });
  });

  describe('selectNext', () => {
    const pool = [
      q('easy', 0.1),
      q('mid', 0.5),
      q('hard', 0.9),
      q('mid2', 0.55),
    ];

    it('первый вопрос — ближайший к сложности 0.5', () => {
      const next = cat.selectNext(pool, [], 'session-1');
      expect(next?.id).toBe('mid');
    });

    it('исключает уже отвеченные вопросы', () => {
      const answered: IrtAnswer[] = [{ question: q('mid', 0.5), isCorrect: true }];
      const next = cat.selectNext(pool, answered, 'session-1');
      expect(next?.id).not.toBe('mid');
      expect(next).not.toBeNull();
    });

    it('выбирает вопрос с максимальной информацией при theta ~ 0', () => {
      const answered: IrtAnswer[] = [
        { question: q('a', 0.5), isCorrect: true },
        { question: q('b', 0.5), isCorrect: true },
      ];
      const next = cat.selectNext(pool, answered, 'session-1');
      // при положительной оценке предпочтительны более сложные, но не крайние вопросы
      expect(next?.id).not.toBe('easy');
    });

    it('детерминирован для одной и той же сессии', () => {
      const answered: IrtAnswer[] = [{ question: q('mid', 0.5), isCorrect: false }];
      const a = cat.selectNext(pool, answered, 'session-1');
      const b = cat.selectNext(pool, answered, 'session-1');
      expect(a?.id).toBe(b?.id);
    });

    it('возвращает null, когда пул исчерпан', () => {
      const answered: IrtAnswer[] = pool.map((p) => ({ question: p, isCorrect: true }));
      expect(cat.selectNext(pool, answered, 'session-1')).toBeNull();
      expect(cat.selectNext([], [], 'session-1')).toBeNull();
    });
  });

  describe('sessionOrderKey', () => {
    it('детерминирован и различается для разных пар', () => {
      expect(sessionOrderKey('s1', 'q1')).toBe(sessionOrderKey('s1', 'q1'));
      expect(sessionOrderKey('s1', 'q1')).not.toBe(sessionOrderKey('s1', 'q2'));
      expect(sessionOrderKey('s1', 'q1')).not.toBe(sessionOrderKey('s2', 'q1'));
    });

    it('возвращает 32-битное беззнаковое значение', () => {
      const key = sessionOrderKey('abc', 'def');
      expect(key).toBeGreaterThanOrEqual(0);
      expect(Number.isInteger(key)).toBe(true);
    });
  });
});
