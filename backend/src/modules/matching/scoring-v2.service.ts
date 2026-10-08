import { Injectable, Logger } from '@nestjs/common';
import { Grade, Specialization } from '@prisma/client';
import { MlServiceClient, QueryUnderstanding } from './ml-client.service';

export const GRADE_ORDER: Grade[] = [
  Grade.JUNIOR,
  Grade.MIDDLE,
  Grade.MIDDLE_PLUS,
  Grade.SENIOR,
  Grade.LEAD,
  Grade.PRINCIPAL,
];

export interface MatchComponents {
  specializationMatch: number;
  stackOverlap: number;
  semanticSimilarity: number;
  gradeMatch: number;
  achievementBoost: number;
  experienceRelevance: number;
  testScore: number;
}

export interface MatchScore {
  total: number;
  components: MatchComponents;
  contributions: Record<string, number>;  // V2: contribution of each component
  weightsSource: 'default' | 'learned' | 'hybrid';
}

export interface ScoreableCandidate {
  specialization: Specialization | null;
  verifiedGrade: Grade | null;
  declaredGrade: Grade | null;
  stack: string[];
  achievementPoints: number;
  testScore: number | null;
  experienceYears: number | null;
}

export interface Need {
  specialization?: Specialization | null;
  grade?: Grade | null;
  stack: string[];
  requiredSkills?: string[];
  preferredSkills?: string[];
}

export const DEFAULT_WEIGHTS = {
  specialization: 0.18,
  stack: 0.15,
  semantic: 0.20,
  grade: 0.12,
  achievement: 0.08,
  experience: 0.12,
  testScore: 0.15,
} as const;

// Skill importance: some skills are "must-have", others "nice-to-have"
const SKILL_IMPORTANCE: Record<string, number> = {
  // Languages — high importance
  'node.js': 1.0, nodejs: 1.0, javascript: 1.0, typescript: 1.0,
  python: 1.0, java: 1.0, go: 1.0, golang: 1.0,
  'c++': 0.9, 'c#': 0.9, rust: 0.9, ruby: 0.8, php: 0.8,
  swift: 0.9, kotlin: 0.9,
  // Frameworks — medium-high
  react: 0.9, vue: 0.85, angular: 0.85, django: 0.9, flask: 0.8,
  fastapi: 0.85, spring: 0.9, nextjs: 0.85,
  // Databases — high
  postgresql: 1.0, postgres: 1.0, mysql: 0.9, mongodb: 0.85, redis: 0.9,
  elasticsearch: 0.8,
  // Infrastructure — medium
  docker: 0.85, kubernetes: 0.9, k8s: 0.9, aws: 0.85, gcp: 0.8, azure: 0.8,
  // Messaging — medium
  kafka: 0.85, rabbitmq: 0.8, grpc: 0.8, graphql: 0.8, rest: 0.75,
  // Concepts — medium
  microservices: 0.85, cicd: 0.75, 'ci/cd': 0.75,
};

@Injectable()
export class MatchingScoringServiceV2 {
  private readonly logger = new Logger('ScoringV2');

  constructor(private readonly mlClient: MlServiceClient) {}

  /**
   * V2: Calculate match with expanded features and dynamic weights.
   */
  async calculateMatch(
    candidate: ScoreableCandidate,
    need: Need,
    semanticSimilarity: number,
    candidateText?: string,
    queryText?: string,
  ): Promise<MatchScore> {
    // Calculate all components
    const components: MatchComponents = {
      specializationMatch: this.specializationScore(candidate.specialization, need.specialization),
      stackOverlap: this.stackScore(candidate.stack, need.stack, need.requiredSkills),
      semanticSimilarity: this.clamp100(semanticSimilarity * 100),
      gradeMatch: this.gradeScore(candidate.verifiedGrade, candidate.declaredGrade, need.grade),
      achievementBoost: this.achievementScore(candidate.achievementPoints),
      experienceRelevance: this.experienceScore(candidate.experienceYears, need.grade),
      testScore: this.testScoreComponent(candidate.testScore),
    };

    // V2: Optional re-ranking boost
    let rerankBoost = 0;
    if (candidateText && queryText && this.mlClient.available) {
      rerankBoost = await this.getRerankBoost(queryText, candidateText);
    }

    // Use default weights (V2: could be learned per-employer)
    const weights = DEFAULT_WEIGHTS;
    const weightsSource: 'default' | 'learned' | 'hybrid' = 'default';

    // Calculate weighted score
    const contributions: Record<string, number> = {};
    let total = 0;

    for (const [key, weight] of Object.entries(weights)) {
      const componentKey = key === 'specialization' ? 'specializationMatch'
        : key === 'stack' ? 'stackOverlap'
        : key === 'semantic' ? 'semanticSimilarity'
        : key === 'grade' ? 'gradeMatch'
        : key === 'achievement' ? 'achievementBoost'
        : key === 'experience' ? 'experienceRelevance'
        : key === 'testScore' ? 'testScore'
        : key;

      const componentValue = components[componentKey as keyof MatchComponents] || 0;
      const contribution = componentValue * weight;
      contributions[key] = Math.round(contribution * 10) / 10;
      total += contribution;
    }

    // Add re-ranking boost (up to +10 points)
    if (rerankBoost > 0) {
      const boostContribution = rerankBoost * 10;
      contributions['rerank'] = Math.round(boostContribution * 10) / 10;
      total += boostContribution;
    }

    return {
      total: Math.round(total * 10) / 10,
      components,
      contributions,
      weightsSource,
    };
  }

  /**
   * V2: Get re-ranking boost for a single candidate.
   * Returns 0-1 (how much to boost based on cross-encoder).
   */
  private async getRerankBoost(query: string, candidateText: string): Promise<number> {
    try {
      const result = await this.mlClient.rerank(query, [candidateText], 1);
      if (result.available && result.results.length > 0) {
        // Normalize cross-encoder score (typically -10 to 10) to 0-1
        const rawScore = result.results[0].score;
        return Math.max(0, Math.min(1, (rawScore + 10) / 20));
      }
    } catch (e) {
      this.logger.debug(`Rerank boost failed: ${(e as Error).message}`);
    }
    return 0;
  }

  /**
   * Specialization match scoring.
   */
  private specializationScore(
    candidateSpec: Specialization | null,
    needSpec: Specialization | null | undefined,
  ): number {
    if (!needSpec) return 50;  // Neutral if not specified
    if (!candidateSpec) return 30;

    if (candidateSpec === needSpec) return 100;

    // Related specializations
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

    if ((RELATED[needSpec] || []).includes(candidateSpec)) return 60;
    return 0;
  }

  /**
   * V2: Stack overlap with importance weighting.
   */
  stackScore(candidateStack: string[], needStack: string[], requiredSkills?: string[]): number {
    if (!needStack?.length && !requiredSkills?.length) return 50;
    if (!candidateStack?.length) return 0;

    const candidateNormalized = new Set(candidateStack.map(normalizeTech));

    // Calculate weighted overlap
    let totalWeight = 0;
    let matchedWeight = 0;

    // Required skills (higher weight)
    for (const skill of requiredSkills || []) {
      const importance = SKILL_IMPORTANCE[skill.toLowerCase()] ?? 0.7;
      totalWeight += importance * 1.5;  // Required skills weighted 1.5x

      if (candidateNormalized.has(normalizeTech(skill))) {
        matchedWeight += importance * 1.5;
      }
    }

    // Need stack (base weight)
    for (const skill of needStack) {
      const importance = SKILL_IMPORTANCE[skill.toLowerCase()] ?? 0.7;
      totalWeight += importance;

      if (candidateNormalized.has(normalizeTech(skill))) {
        matchedWeight += importance;
      }
    }

    if (totalWeight === 0) return 50;
    return this.clamp100(Math.round((matchedWeight / totalWeight) * 100));
  }

  /**
   * V2: Grade match with sub-grade handling.
   */
  gradeScore(
    verifiedGrade: Grade | null,
    declaredGrade: Grade | null,
    needGrade: Grade | null | undefined,
  ): number {
    if (!needGrade) return 50;
    const candidateGrade = verifiedGrade || declaredGrade;
    if (!candidateGrade) return 30;

    const candidateIdx = GRADE_ORDER.indexOf(candidateGrade);
    const needIdx = GRADE_ORDER.indexOf(needGrade);

    if (candidateIdx === needIdx) return 100;
    if (candidateIdx === needIdx + 1) return 80;  // One above — good
    if (candidateIdx === needIdx - 1) return 60;  // One below — acceptable
    if (candidateIdx > needIdx + 1) return 40;    // Overqualified
    return 20;  // Underqualified
  }

  /**
   * V2: Achievement score with diminishing returns.
   */
  achievementScore(totalPoints: number): number {
    if (!totalPoints || totalPoints <= 0) return 0;
    // Logarithmic scale: diminishing returns for many achievements
    return Math.min(100, Math.round(Math.log2(1 + totalPoints) * 15));
  }

  /**
   * V2: Experience relevance based on grade requirement.
   */
  experienceScore(experienceYears: number | null, needGrade: Grade | null | undefined): number {
    if (experienceYears == null) return 40;  // Neutral
    if (!needGrade) return 50;

    // Expected experience by grade
    const expected: Record<Grade, [number, number]> = {
      JUNIOR: [0, 2],
      MIDDLE: [2, 5],
      MIDDLE_PLUS: [3, 7],
      SENIOR: [5, 10],
      LEAD: [8, 15],
      PRINCIPAL: [10, 20],
    };

    const [min, max] = expected[needGrade];
    const mid = (min + max) / 2;

    if (experienceYears >= min && experienceYears <= max) return 100;
    if (experienceYears < min) {
      // Below minimum — proportional
      if (experienceYears >= min * 0.5) return 60;
      return 20;
    }
    // Above maximum — still good, slight penalty for overqualification
    if (experienceYears <= max * 1.5) return 80;
    return 50;
  }

  /**
   * V2: Test score component.
   */
  private testScoreComponent(testScore: number | null): number {
    if (testScore == null) return 40;  // Neutral — not tested
    return this.clamp100(Math.round(testScore));
  }

  private clamp100(value: number): number {
    if (!Number.isFinite(value)) return 0;
    return Math.max(0, Math.min(100, Math.round(value)));
  }
}

/**
 * V2: Normalize tech names for comparison.
 */
export function normalizeTech(value: string): string {
  const normalized = (value || '').toLowerCase().trim();

  // Handle common aliases
  const aliases: Record<string, string> = {
    'node.js': 'nodejs',
    'node js': 'nodejs',
    'node': 'nodejs',
    'postgres': 'postgresql',
    'react.js': 'react',
    'react js': 'react',
    'vue.js': 'vue',
    'vue js': 'vue',
    'angular.js': 'angular',
    'k8s': 'kubernetes',
    'js': 'javascript',
    'ts': 'typescript',
    'py': 'python',
    'golang': 'go',
    'c sharp': 'csharp',
    'c#': 'csharp',
    '.net': 'dotnet',
    'dot net': 'dotnet',
  };

  if (aliases[normalized]) return aliases[normalized];

  // Remove dots, spaces, dashes for comparison
  return normalized.replace(/[.\s-]+/g, '');
}