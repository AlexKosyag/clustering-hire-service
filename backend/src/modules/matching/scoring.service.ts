import { Injectable } from '@nestjs/common';
import { Grade, Specialization } from '@prisma/client';

export const GRADE_ORDER: Grade[] = [
  Grade.JUNIOR,
  Grade.MIDDLE,
  Grade.MIDDLE_PLUS,
  Grade.SENIOR,
  Grade.LEAD,
  Grade.PRINCIPAL,
];

/** Близкие специализации — частичное совпадение категории. */
const RELATED: Record<string, Specialization[]> = {
  BACKEND: [Specialization.FULLSTACK, Specialization.DEVOPS],
  FRONTEND: [Specialization.FULLSTACK, Specialization.DESIGN_UX],
  FULLSTACK: [Specialization.BACKEND, Specialization.FRONTEND],
  MOBILE: [Specialization.FULLSTACK],
  DEVOPS: [Specialization.BACKEND, Specialization.SECURITY],
  DATA_SCIENCE: [Specialization.ML_AI],
  ML_AI: [Specialization.DATA_SCIENCE],
  QA: [Specialization.BACKEND, Specialization.FRONTEND],
  SECURITY: [Specialization.DEVOPS],
  EMBEDDED: [Specialization.BACKEND],
  GAME_DEV: [Specialization.BACKEND, Specialization.DESIGN_UX],
  DESIGN_UX: [Specialization.FRONTEND],
  MANAGEMENT: [Specialization.FULLSTACK],
};

export const WEIGHTS = {
  category: 0.3,
  stack: 0.25,
  semantic: 0.2,
  grade: 0.15,
  achievement: 0.1,
} as const;

export interface MatchComponents {
  categoryMatch: number;
  stackOverlap: number;
  semanticSimilarity: number;
  gradeMatch: number;
  achievementBoost: number;
}

export interface MatchScore {
  total: number;
  components: MatchComponents;
}

export interface ScoreableCandidate {
  specialization: Specialization | null;
  verifiedGrade: Grade | null;
  declaredGrade: Grade | null;
  stack: string[];
  achievementPoints: number;
}

export interface Need {
  specialization?: Specialization | null;
  grade?: Grade | null;
  stack: string[];
}

@Injectable()
export class MatchingScoringService {
  calculateMatch(candidate: ScoreableCandidate, need: Need, semanticSimilarity: number): MatchScore {
    const components: MatchComponents = {
      categoryMatch: this.categoryScore(candidate, need),
      stackOverlap: this.stackScore(candidate.stack, need.stack),
      semanticSimilarity: this.clamp100(semanticSimilarity * 100),
      gradeMatch: this.gradeScore(candidate, need),
      achievementBoost: this.achievementScore(candidate.achievementPoints),
    };

    const total =
      components.categoryMatch * WEIGHTS.category +
      components.stackOverlap * WEIGHTS.stack +
      components.semanticSimilarity * WEIGHTS.semantic +
      components.gradeMatch * WEIGHTS.grade +
      components.achievementBoost * WEIGHTS.achievement;

    return { total: Math.round(total * 10) / 10, components };
  }

  categoryScore(candidate: ScoreableCandidate, need: Need): number {
    if (!need.specialization) return 50;
    if (!candidate.specialization) return 30;
    if (candidate.specialization === need.specialization) return 100;
    if ((RELATED[need.specialization] || []).includes(candidate.specialization)) return 60;
    return 0;
  }

  stackScore(candidateStack: string[], needStack: string[]): number {
    if (!needStack?.length) return 50;
    if (!candidateStack?.length) return 0;
    const need = needStack.map(normalizeTech);
    const have = new Set(candidateStack.map(normalizeTech));
    const overlap = need.filter((s) => have.has(s));
    return this.clamp100(Math.round((overlap.length / need.length) * 100));
  }

  gradeScore(candidate: ScoreableCandidate, need: Need): number {
    if (!need.grade) return 50;
    const candidateGrade = candidate.verifiedGrade || candidate.declaredGrade;
    if (!candidateGrade) return 30;

    const candidateIdx = GRADE_ORDER.indexOf(candidateGrade);
    const needIdx = GRADE_ORDER.indexOf(need.grade);

    if (candidateIdx === needIdx) return 100;
    if (candidateIdx === needIdx + 1) return 80;
    if (candidateIdx === needIdx - 1) return 60;
    if (candidateIdx > needIdx + 1) return 40;
    return 20;
  }

  achievementScore(totalPoints: number): number {
    if (!totalPoints || totalPoints <= 0) return 0;
    return Math.min(100, totalPoints * 10);
  }

  private clamp100(value: number): number {
    if (!Number.isFinite(value)) return 0;
    return Math.max(0, Math.min(100, Math.round(value)));
  }
}

/**
 * Нормализация названий технологий для сравнения стеков:
 * регистр не важен, «Node.js» = «Node JS» = «node-js» = «nodejs».
 * Знаки +/# сохраняются («C++» ≠ «C»).
 */
export function normalizeTech(value: string): string {
  return (value || '').toLowerCase().trim().replace(/[.\s-]+/g, '');
}
