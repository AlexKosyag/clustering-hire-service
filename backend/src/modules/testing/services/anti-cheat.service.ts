import { Injectable } from '@nestjs/common';
import { IrtAnswer, sessionOrderKey } from './cat.service';

export interface CheatAnalysis {
  isSuspicious: boolean;
  flags: string[];
  confidence: number;
}

export interface TimedAnswer {
  timeSpentSeconds: number | null;
  difficulty: number;
  poolTag: string | null;
}

/**
 * Anti-cheat: анализ времени, паттернов сложности, «идеальных» пулов,
 * волатильности theta, а также рандомизация порядка вариантов ответа.
 */
@Injectable()
export class AntiCheatService {
  analyze(answers: TimedAnswer[], thetaHistory: number[] = []): CheatAnalysis {
    const flags: string[] = [];

    // 1. Слишком быстрые ответы на сложных вопросах
    const tooFast = answers.filter(
      (a) => a.timeSpentSeconds != null && a.timeSpentSeconds < 5 && a.difficulty > 0.5,
    );
    if (tooFast.length > 3) flags.push('MULTIPLE_TOO_FAST_ANSWERS');

    // 2. Паттерн «сложные — идеально, лёгкие — провал»
    const easy = answers.filter((a) => a.difficulty < 0.3);
    const hard = answers.filter((a) => a.difficulty > 0.7);
    if (easy.length >= 3 && hard.length >= 3) {
      const easyCorrect = easy.filter((a) => (a as any).isCorrect === true).length;
      const hardCorrect = hard.filter((a) => (a as any).isCorrect === true).length;
      if (hardCorrect > easyCorrect && easyCorrect < 2) flags.push('SUSPICIOUS_DIFFICULTY_PATTERN');
    }

    // 3. Идеальный результат по всему пулу вопросов
    const byPool = new Map<string, TimedAnswer[]>();
    for (const answer of answers) {
      if (!answer.poolTag) continue;
      const list = byPool.get(answer.poolTag) || [];
      list.push(answer);
      byPool.set(answer.poolTag, list);
    }
    for (const [pool, list] of byPool) {
      if (list.length > 5 && list.every((a) => (a as any).isCorrect === true)) {
        flags.push(`PERFECT_SCORE_POOL_${pool.toUpperCase()}`);
      }
    }

    // 4. Резкие скачки theta между ответами
    if (thetaHistory.length >= 4) {
      let jumps = 0;
      for (let i = 1; i < thetaHistory.length; i++) {
        if (Math.abs(thetaHistory[i] - thetaHistory[i - 1]) > 2) jumps += 1;
      }
      if (jumps > 2) flags.push('THETA_VOLATILITY');
    }

    return {
      isSuspicious: flags.length >= 2,
      flags,
      confidence: Math.min(1, Math.round(flags.length * 0.35 * 100) / 100),
    };
  }

  /** Порядок вариантов ответа, уникальный для каждой сессии (клиент не видит isCorrect). */
  shuffleOptions<T>(sessionId: string, questionId: string, options: T[]): T[] {
    if (!options || options.length <= 1) return options || [];
    const shift = sessionOrderKey(sessionId, questionId) % options.length;
    return [...options.slice(shift), ...options.slice(0, shift)];
  }

  /** История theta после каждого ответа — вход для детекции волатильности. */
  thetaHistory(answers: IrtAnswer[], estimate: (prefix: IrtAnswer[]) => number): number[] {
    const history: number[] = [];
    for (let i = 1; i <= answers.length; i++) {
      history.push(estimate(answers.slice(0, i)));
    }
    return history;
  }
}
