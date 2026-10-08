import { Test, TestingModule } from '@nestjs/testing';
import { BehavioralAntiCheatService, BehavioralData } from './behavioral-anti-cheat.service';

describe('BehavioralAntiCheatService', () => {
  let service: BehavioralAntiCheatService;

  beforeEach(async () => {
    const module: TestingModule = await Test.createTestingModule({
      providers: [BehavioralAntiCheatService],
    }).compile();

    service = module.get<BehavioralAntiCheatService>(BehavioralAntiCheatService);
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('analyzeQuestion', () => {
    it('should flag too-fast answers on hard questions', () => {
      const behavioral: BehavioralData = {
        timeSpentSeconds: 3,
        copyPasteEvents: 0,
        tabSwitches: 0,
        keystrokeCount: 50,
        idleTimeSeconds: 0,
      };

      const result = service.analyzeQuestion(behavioral, 0.8, 100);

      expect(result.flags).toContain('TOO_FAST_FOR_HARD_QUESTION');
      expect(result.suspicionScore).toBeGreaterThan(20);
    });

    it('should flag multiple copy-paste events', () => {
      const behavioral: BehavioralData = {
        timeSpentSeconds: 60,
        copyPasteEvents: 5,
        tabSwitches: 1,
        keystrokeCount: 10,
        idleTimeSeconds: 5,
      };

      const result = service.analyzeQuestion(behavioral, 0.5, 200);

      expect(result.flags).toContain('MULTIPLE_COPY_PASTE');
    });

    it('should flag large paste detection', () => {
      const behavioral: BehavioralData = {
        timeSpentSeconds: 30,
        copyPasteEvents: 1,
        tabSwitches: 0,
        keystrokeCount: 5,
        idleTimeSeconds: 0,
      };

      const result = service.analyzeQuestion(behavioral, 0.5, 200);

      expect(result.flags).toContain('LARGE_PASTE_DETECTED');
    });

    it('should flag excessive tab switching', () => {
      const behavioral: BehavioralData = {
        timeSpentSeconds: 60,
        copyPasteEvents: 0,
        tabSwitches: 10,
        keystrokeCount: 100,
        idleTimeSeconds: 5,
      };

      const result = service.analyzeQuestion(behavioral, 0.5, 100);

      expect(result.flags).toContain('EXCESSIVE_TAB_SWITCHING');
    });

    it('should have low suspicion for honest behavior', () => {
      const behavioral: BehavioralData = {
        timeSpentSeconds: 45,
        copyPasteEvents: 0,
        tabSwitches: 1,
        keystrokeCount: 80,
        idleTimeSeconds: 5,
      };

      const result = service.analyzeQuestion(behavioral, 0.5, 100);

      expect(result.suspicionScore).toBeLessThan(30);
      expect(result.flags.length).toBeLessThanOrEqual(1);
    });
  });

  describe('analyzeSession', () => {
    it('should aggregate flags from multiple questions', () => {
      const analyses = [
        { suspicionScore: 30, flags: ['FLAG_A'], details: {}, confidence: 0.8 },
        { suspicionScore: 40, flags: ['FLAG_B'], details: {}, confidence: 0.8 },
        { suspicionScore: 20, flags: [], details: {}, confidence: 0.8 },
      ];

      const result = service.analyzeSession(analyses, []);

      expect(result.flags).toContain('FLAG_A');
      expect(result.flags).toContain('FLAG_B');
      expect(result.suspicionScore).toBeGreaterThan(0);
    });

    it('should flag theta volatility', () => {
      const thetaHistory = [0, 3, -2, 2.5, -1, 4];
      const analyses = [
        { suspicionScore: 10, flags: [], details: {}, confidence: 0.5 },
      ];

      const result = service.analyzeSession(analyses, thetaHistory);

      expect(result.flags).toContain('THETA_VOLATILITY');
    });

    it('should flag multiple suspicious questions', () => {
      const analyses = [
        { suspicionScore: 60, flags: [], details: {}, confidence: 0.8 },
        { suspicionScore: 70, flags: [], details: {}, confidence: 0.8 },
        { suspicionScore: 55, flags: [], details: {}, confidence: 0.8 },
      ];

      const result = service.analyzeSession(analyses, []);

      expect(result.flags).toContain('MULTIPLE_SUSPICIOUS_QUESTIONS');
    });
  });

  describe('analyzeKeystrokePattern', () => {
    it('should return null for insufficient data', () => {
      const result = service.analyzeKeystrokePattern([100, 100, 100]);
      expect(result).toBeNull();
    });

    it('should analyze consistent typing pattern', () => {
      const intervals = Array(20).fill(100);  // Very consistent
      const result = service.analyzeKeystrokePattern(intervals);

      expect(result).not.toBeNull();
      expect(result!.rhythmConsistency).toBeGreaterThan(0.9);
      expect(result!.burstiness).toBeLessThan(0.1);
    });

    it('should analyze bursty typing pattern', () => {
      const intervals = [50, 50, 50, 200, 200, 200, 50, 50, 50, 200, 200, 200, 50, 50, 50, 200, 200, 200, 50, 50];
      const result = service.analyzeKeystrokePattern(intervals);

      expect(result).not.toBeNull();
      expect(result!.burstiness).toBeGreaterThan(0.5);
    });
  });
});