import { CandidateProfile, Prisma } from '@prisma/client';
import { MatchExplanation } from './explanation.service';
import { MatchScore } from './scoring.service';

export type CandidateWithRelations = CandidateProfile & {
  achievements: Array<{ achievement: { name: string; category: string; points: number; year: number | null; place: string | null } }>;
  user?: { email: string } | null;
};

export interface CandidatePublicDto {
  id: string;
  displayName: string;
  specialization: string | null;
  declaredGrade: string | null;
  verifiedGrade: string | null;
  stack: string[];
  roles: string[];
  experienceYears: number | null;
  city: string | null;
  softSkills: string[];
  bio: string | null;
  testStatus: string;
  testScore: number | null;
  lastTestDate: Date | null;
  achievements: Array<{ name: string; category: string; points: number; year: number | null; place: string | null }>;
  achievementPoints: number;
  lastActive: Date;
  indexed: boolean;
  matchScore?: number;
  matchExplanation?: MatchExplanation;
  matchComponents?: MatchScore['components'];
}

export const candidateInclude = {
  achievements: { include: { achievement: true } },
  user: { select: { email: true } },
} satisfies Prisma.CandidateProfileInclude;

/** «Иван И.» — как в документации; при отключённом showFullName — псевдоним. */
export function buildDisplayName(candidate: CandidateProfile): string {
  const anonymized = candidate.showFullName === false;
  if (anonymized || (!candidate.firstName && !candidate.lastName)) {
    return `Кандидат #${candidate.id.slice(0, 4)}`;
  }
  const first = candidate.firstName || '';
  const last = candidate.lastName ? `${candidate.lastName[0]}.` : '';
  return `${first} ${last}`.trim();
}

export function achievementPoints(candidate: CandidateWithRelations): number {
  return candidate.achievements.reduce((sum, a) => sum + (a.achievement.points || 0), 0);
}

export function toCandidatePublic(candidate: CandidateWithRelations, indexed = true): CandidatePublicDto {
  return {
    id: candidate.id,
    displayName: buildDisplayName(candidate),
    specialization: candidate.specialization,
    declaredGrade: candidate.declaredGrade,
    verifiedGrade: candidate.verifiedGrade,
    stack: candidate.stack || [],
    roles: candidate.roles || [],
    experienceYears: candidate.experienceYears,
    city: candidate.city,
    softSkills: candidate.softSkills || [],
    bio: candidate.bio,
    testStatus: candidate.testStatus,
    testScore: candidate.lastTestScore,
    lastTestDate: candidate.lastTestDate,
    achievements: candidate.achievements.map((a) => ({
      name: a.achievement.name,
      category: a.achievement.category,
      points: a.achievement.points,
      year: a.achievement.year,
      place: a.achievement.place,
    })),
    achievementPoints: achievementPoints(candidate),
    lastActive: candidate.updatedAt,
    indexed,
  };
}
