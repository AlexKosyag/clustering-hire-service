import { Injectable } from '@nestjs/common';
import { Grade, Specialization } from '@prisma/client';

export interface AnswerStat {
  isCorrect: boolean;
  difficulty: number;
  score?: number | null;
}

export interface DifficultyDetails {
  easy: { correct: number; total: number };
  medium: { correct: number; total: number };
  hard: { correct: number; total: number };
}

/** Пороговые значения theta для каждого грейда (из docs/modules/testing-engine.md). */
export const GRADE_THRESHOLDS: Array<{ grade: Grade; min: number }> = [
  { grade: Grade.JUNIOR, min: -Infinity },
  { grade: Grade.MIDDLE, min: -1.0 },
  { grade: Grade.MIDDLE_PLUS, min: 0.0 },
  { grade: Grade.SENIOR, min: 0.5 },
  { grade: Grade.LEAD, min: 1.0 },
  { grade: Grade.PRINCIPAL, min: 1.5 },
];

/** Небольшая коррекция сложности по специализации (в доках указана как normalizeTheta). */
const SPEC_OFFSET: Partial<Record<Specialization, number>> = {};

@Injectable()
export class ScoringService {
  calculateGrade(theta: number, specialization?: Specialization | null): Grade {
    const value = Number.isFinite(theta) ? theta : 0;
    const normalized = value + (specialization ? SPEC_OFFSET[specialization] ?? 0 : 0);

    let result: Grade = Grade.JUNIOR;
    for (const threshold of GRADE_THRESHOLDS) {
      if (normalized >= threshold.min) result = threshold.grade;
    }
    return result;
  }

  getRecommendation(declared: Grade | null, calculated: Grade, score: number): 'PASS' | 'FAIL' | 'UPGRADE' {
    if (!declared) return score >= 50 ? 'PASS' : 'FAIL';
    const declaredIndex = this.gradeIndex(declared);
    const calculatedIndex = this.gradeIndex(calculated);

    if (calculatedIndex >= declaredIndex + 1 && score >= 85) return 'UPGRADE';
    if (calculatedIndex >= declaredIndex) return 'PASS';
    return 'FAIL';
  }

  /** Взвешенная оценка 0-100: сложные вопросы весят больше. */
  scoreFromAnswers(answers: AnswerStat[]): number {
    if (!answers.length) return 0;
    let earned = 0;
    let weight = 0;
    for (const answer of answers) {
      const w = 0.5 + answer.difficulty;
      weight += w;
      const value = answer.score != null ? answer.score : answer.isCorrect ? 1 : 0;
      earned += w * Math.max(0, Math.min(1, value));
    }
    if (weight === 0) return 0;
    return Math.round((earned / weight) * 1000) / 10;
  }

  detailsByDifficulty(answers: AnswerStat[]): DifficultyDetails {
    const details: DifficultyDetails = {
      easy: { correct: 0, total: 0 },
      medium: { correct: 0, total: 0 },
      hard: { correct: 0, total: 0 },
    };
    for (const answer of answers) {
      const bucket = answer.difficulty < 0.4 ? 'easy' : answer.difficulty <= 0.65 ? 'medium' : 'hard';
      details[bucket].total += 1;
      if (answer.isCorrect) details[bucket].correct += 1;
    }
    return details;
  }

  gradeIndex(grade: Grade): number {
    const order = [Grade.JUNIOR, Grade.MIDDLE, Grade.MIDDLE_PLUS, Grade.SENIOR, Grade.LEAD, Grade.PRINCIPAL];
    return order.indexOf(grade);
  }
}
