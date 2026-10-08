import { Test, TestingModule } from '@nestjs/testing';
import { HttpService } from '@nestjs/axios';
import { MatchingScoringServiceV2, normalizeTech } from './scoring-v2.service';
import { MlServiceClient } from './ml-client.service';
import { Grade, Specialization } from '@prisma/client';

// Mock @nestjs/axios to avoid ESM issues
jest.mock('@nestjs/axios', () => ({
  HttpService: class MockHttpService {
    get = jest.fn();
    post = jest.fn();
  },
}));

describe('MatchingScoringServiceV2', () => {
  let service: MatchingScoringServiceV2;

  beforeEach(async () => {
    const module: TestingModule = await Test.createTestingModule({
      providers: [
        MatchingScoringServiceV2,
        MlServiceClient,
        { provide: HttpService, useValue: { get: jest.fn(), post: jest.fn() } },
      ],
    }).compile();

    service = module.get<MatchingScoringServiceV2>(MatchingScoringServiceV2);
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('calculateMatch', () => {
    const perfectCandidate = {
      specialization: Specialization.BACKEND,
      verifiedGrade: Grade.MIDDLE,
      declaredGrade: Grade.MIDDLE,
      stack: ['Node.js', 'PostgreSQL', 'Redis', 'Docker'],
      achievementPoints: 15,
      testScore: 85,
      experienceYears: 4,
    };

    const perfectNeed = {
      specialization: Specialization.BACKEND,
      grade: Grade.MIDDLE,
      stack: ['Node.js', 'PostgreSQL'],
      requiredSkills: ['Node.js', 'PostgreSQL'],
    };

    it('should return high score for perfect match', async () => {
      const result = await service.calculateMatch(
        perfectCandidate,
        perfectNeed,
        0.85,
      );

      expect(result.total).toBeGreaterThanOrEqual(70);
      expect(result.components.specializationMatch).toBe(100);
      expect(result.components.gradeMatch).toBe(100);
      expect(result.weightsSource).toBe('default');
    });

    it('should return low score for mismatched candidate', async () => {
      const mismatchedCandidate = {
        specialization: Specialization.FRONTEND,
        verifiedGrade: Grade.JUNIOR,
        declaredGrade: Grade.JUNIOR,
        stack: ['React', 'CSS'],
        achievementPoints: 0,
        testScore: 40,
        experienceYears: 1,
      };

      const result = await service.calculateMatch(
        mismatchedCandidate,
        perfectNeed,
        0.2,
      );

      expect(result.total).toBeLessThanOrEqual(50);
      expect(result.components.specializationMatch).toBe(0);
    });

    it('should calculate contributions correctly', async () => {
      const result = await service.calculateMatch(
        perfectCandidate,
        perfectNeed,
        0.85,
      );

      expect(result.contributions).toBeDefined();
      expect(result.contributions['specialization']).toBeGreaterThan(0);
      expect(result.contributions['semantic']).toBeGreaterThan(0);
    });
  });

  describe('stackScore', () => {
    it('should return 100 for exact match', () => {
      const score = service.stackScore(
        ['Node.js', 'PostgreSQL'],
        ['Node.js', 'PostgreSQL'],
      );
      expect(score).toBe(100);
    });

    it('should return 0 for no overlap', () => {
      const score = service.stackScore(
        ['React', 'CSS'],
        ['Node.js', 'PostgreSQL'],
      );
      expect(score).toBe(0);
    });

    it('should handle aliases correctly', () => {
      const score = service.stackScore(
        ['nodejs', 'postgres'],
        ['Node.js', 'PostgreSQL'],
      );
      expect(score).toBe(100);
    });
  });
});

describe('normalizeTech', () => {
  it('should normalize common aliases', () => {
    expect(normalizeTech('Node.js')).toBe('nodejs');
    expect(normalizeTech('node js')).toBe('nodejs');
    expect(normalizeTech('PostgreSQL')).toBe('postgresql');
    expect(normalizeTech('postgres')).toBe('postgresql');
    expect(normalizeTech('React.js')).toBe('react');
    expect(normalizeTech('K8s')).toBe('kubernetes');
  });

  it('should handle case and whitespace', () => {
    expect(normalizeTech('  NODE.JS  ')).toBe('nodejs');
    expect(normalizeTech('Type Script')).toBe('typescript');
  });

  it('should preserve special characters like C++', () => {
    expect(normalizeTech('C++')).toBe('c++');
  });
});