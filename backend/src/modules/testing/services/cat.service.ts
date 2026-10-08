import { Injectable } from '@nestjs/common';

export interface IrtQuestion {
  id: string;
  difficulty: number;
  discrimination: number;
  guessing: number;
}

export interface IrtAnswer {
  question: IrtQuestion;
  isCorrect: boolean;
}

/** Детерминированный порядок вопроса внутри сессии (анти-чит: уникальная последовательность у каждого кандидата). */
export function sessionOrderKey(sessionId: string, questionId: string): number {
  const input = `${sessionId}:${questionId}`;
  let hash = 0x811c9dc5;
  for (let i = 0; i < input.length; i++) {
    hash ^= input.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

/**
 * Computer Adaptive Testing (3PL IRT):
 *  - следующий вопрос выбирается по максимальной Fisher Information при текущем theta,
 *  - theta оценивается методом максимального правдоподобия (Newton-Raphson).
 * Все вычисления детерминированы — повторный запрос даёт тот же вопрос.
 */
@Injectable()
export class CATService {
  /** P(правильный ответ) по 3PL: c + (1-c)/(1+e^{-a(theta-b)}) */
  probability(theta: number, a: number, b: number, c: number): number {
    const ability = Number.isFinite(theta) ? theta : 0;
    const discrimination = Number.isFinite(a) && a > 0 ? a : 1;
    const difficulty = Number.isFinite(b) ? b : 0.5;
    const guessing = Number.isFinite(c) && c >= 0 && c < 1 ? c : 0.25;
    const p =
      guessing + (1 - guessing) / (1 + Math.exp(-discrimination * (ability - difficulty)));
    if (!Number.isFinite(p)) return 1 - 1e-6;
    return Math.min(1 - 1e-6, Math.max(1e-6, p));
  }

  /** Fisher Information для 3PL-модели. */
  fisherInformation(theta: number, question: IrtQuestion): number {
    const a = question.discrimination;
    const b = question.difficulty;
    const c = question.guessing;
    const p = this.probability(theta, a, b, c);
    const exp = Math.exp(-a * (theta - b));
    const denom = Math.pow(1 + exp, 2);
    if (denom === 0) return 0;
    const pPrime = (a * (1 - c) * exp) / denom;
    const value = (pPrime * pPrime) / (p * (1 - p));
    return Number.isFinite(value) ? value : 0;
  }

  /**
   * Оценка theta (MLE, Newton-Raphson / Fisher scoring).
   * Используется ожидаемая информация I(theta) >= 0 вместо наблюдаемого гессиана:
   * гессиан log-правдоподобия 3PL меняет знак в области p < c и ломал оценку
   * (например, «все ответы неверны» давало theta = 0 вместо нижней границы).
   */
  estimateTheta(answers: IrtAnswer[]): number {
    if (!answers.length) return 0;
    let theta = 0;
    const tolerance = 0.001;
    const maxIter = 50;

    for (let iter = 0; iter < maxIter; iter++) {
      let first = 0; // dL/d(theta)
      let info = 0; // ожидаемая информация I(theta) >= 0

      for (const answer of answers) {
        const { discrimination: a, difficulty: b, guessing: c } = answer.question;
        const p = this.probability(theta, a, b, c);
        const u = answer.isCorrect ? 1 : 0;
        const cSafe = Number.isFinite(c) && c >= 0 && c < 1 ? c : 0.25;
        const pc = p - cSafe;
        if (pc <= 0 || p <= 0 || p >= 1) continue;

        first += (a * (u - p) * pc) / (p * (1 - cSafe));
        info += (a * a * pc * pc * (1 - p)) / ((1 - cSafe) * p);
      }

      if (!Number.isFinite(first) || !Number.isFinite(info) || info < 1e-9) break;

      const step = first / info;
      if (!Number.isFinite(step)) break;

      // демпфер: один шаг не больше ±0.5, иначе на старте возможен разлёт
      theta += Math.max(-0.5, Math.min(0.5, step));
      theta = Math.max(-4, Math.min(4, theta));

      if (Math.abs(step) < tolerance) break;
    }

    return Number.isFinite(theta) ? theta : 0;
  }

  /**
   * Выбор следующего вопроса из пула (повторные вопросы исключаются).
   * При равной информации побеждает порядок сессии — /current и /answer согласованы.
   */
  selectNext<T extends IrtQuestion>(available: T[], answered: IrtAnswer[], sessionId: string): T | null {
    const answeredIds = new Set(answered.map((a) => a.question.id));
    const pool = available.filter((q) => !answeredIds.has(q.id));
    if (pool.length === 0) return null;

    if (answered.length === 0) {
      // Первый вопрос — ближайший к средней сложности (docs: difficulty ~0.5)
      return [...pool].sort((x, y) => {
        const diff = Math.abs(x.difficulty - 0.5) - Math.abs(y.difficulty - 0.5);
        if (diff !== 0) return diff;
        return sessionOrderKey(sessionId, x.id) - sessionOrderKey(sessionId, y.id);
      })[0];
    }

    const theta = this.estimateTheta(answered);
    let best: T | null = null;
    let bestInfo = -Infinity;

    for (const question of pool) {
      const info = this.fisherInformation(theta, question);
      const isBetter =
        best === null ||
        info > bestInfo + 1e-12 ||
        (Math.abs(info - bestInfo) <= 1e-12 &&
          sessionOrderKey(sessionId, question.id) < sessionOrderKey(sessionId, best.id));
      if (isBetter) {
        bestInfo = info;
        best = question;
      }
    }
    return best;
  }
}
