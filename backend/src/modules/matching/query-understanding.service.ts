import { Injectable, Logger } from '@nestjs/common';
import { Specialization } from '@prisma/client';
import { MlServiceClient, ParsedQuery, QueryUnderstanding } from './ml-client.service';

export interface EmployerNeedParsed {
  description: string;
  specialization: Specialization | null;
  grade: string | null;
  requiredSkills: string[];
  preferredSkills: string[];
  context: string | null;
  embedding: number[];
  embeddingProvider: string;
}

/**
 * V2: Query Understanding Service.
 *
 * Transforms employer's free-text description into structured query
 * using ML (NER + intent classification) or local fallback.
 */
@Injectable()
export class QueryUnderstandingService {
  private readonly logger = new Logger('QueryUnderstanding');

  constructor(private readonly mlClient: MlServiceClient) {}

  /**
   * Parse employer's need description into structured query.
   */
  async parseNeed(
    description: string,
    explicitSpecialization?: Specialization | null,
    explicitGrade?: string | null,
    explicitStack?: string[],
  ): Promise<EmployerNeedParsed> {
    const t0 = Date.now();

    // Get ML parsing
    const understanding = await this.mlClient.parseQuery(description);
    const parsed = understanding.parsed;

    // Merge explicit filters with ML-parsed (explicit takes priority)
    const specialization = explicitSpecialization || this.toSpecialization(parsed.specialization);
    const grade = explicitGrade || parsed.seniority;

    // Merge skills
    const explicitSkills = explicitStack || [];
    const mlSkills = parsed.requiredSkills;
    const requiredSkills = [...new Set([...mlSkills, ...explicitSkills])];

    this.logger.log(
      `Parsed need in ${Date.now() - t0}ms — spec=${specialization}, grade=${grade}, ` +
      `skills=[${requiredSkills.join(', ')}], provider=${understanding.embeddingProvider}`,
    );

    return {
      description,
      specialization,
      grade,
      requiredSkills,
      preferredSkills: parsed.preferredSkills,
      context: parsed.context,
      embedding: understanding.embedding,
      embeddingProvider: understanding.embeddingProvider,
    };
  }

  /**
   * Build text representation of candidate for re-ranking.
   */
  buildCandidateText(candidate: {
    specialization?: string | null;
    verifiedGrade?: string | null;
    declaredGrade?: string | null;
    stack?: string[];
    bio?: string | null;
    roles?: string[];
    softSkills?: string[];
    experienceYears?: number | null;
  }): string {
    const parts: string[] = [];

    if (candidate.specialization) {
      parts.push(`Specialization: ${candidate.specialization}`);
    }

    const grade = candidate.verifiedGrade || candidate.declaredGrade;
    if (grade) {
      parts.push(`Grade: ${grade}`);
    }

    if (candidate.stack?.length) {
      parts.push(`Tech stack: ${candidate.stack.join(', ')}`);
    }

    if (candidate.experienceYears != null) {
      parts.push(`Experience: ${candidate.experienceYears} years`);
    }

    if (candidate.roles?.length) {
      parts.push(`Roles: ${candidate.roles.join(', ')}`);
    }

    if (candidate.softSkills?.length) {
      parts.push(`Soft skills: ${candidate.softSkills.join(', ')}`);
    }

    if (candidate.bio) {
      parts.push(`Bio: ${candidate.bio}`);
    }

    return parts.join('. ');
  }

  private toSpecialization(spec: string | null): Specialization | null {
    if (!spec) return null;
    const validSpecs = Object.values(Specialization);
    return validSpecs.includes(spec as Specialization) ? (spec as Specialization) : null;
  }
}