import { Injectable, Logger } from '@nestjs/common';

export interface BehavioralData {
  timeSpentSeconds: number;
  copyPasteEvents: number;
  tabSwitches: number;
  keystrokeCount: number;
  idleTimeSeconds: number;
  mouseMovementDistance?: number;
  focusLossEvents?: number;
  pasteSources?: string[];  // What was pasted
}

export interface BehavioralAnalysis {
  suspicionScore: number;  // 0-100, higher = more suspicious
  flags: string[];
  details: Record<string, { value: number; threshold: number; suspicious: boolean }>;
  confidence: number;  // 0-1
}

export interface KeystrokePattern {
  averageIntervalMs: number;
  varianceMs: number;
  burstiness: number;  // 0 = uniform, 1 = bursty
  rhythmConsistency: number;  // 0-1
}

/**
 * V2: Behavioral Anti-Cheat Service.
 *
 * Analyzes candidate behavior during testing:
 * - Keystroke dynamics
 * - Copy-paste detection
 * - Tab switching
 * - Time analysis
 * - Focus loss
 */
@Injectable()
export class BehavioralAntiCheatService {
  private readonly logger = new Logger('BehavioralAntiCheat');

  // Thresholds (tunable)
  private readonly thresholds = {
    minTimePerDifficulty: {
      easy: 10,     // seconds
      medium: 20,
      hard: 40,
    },
    maxCopyPastePerTest: 3,
    maxTabSwitchesPerTest: 5,
    minKeystrokesPerChar: 0.3,  // At least 0.3 keystrokes per character
    maxIdleRatio: 0.3,          // Max 30% idle time
    suspiciousFastAnswer: 5,    // seconds
  };

  /**
   * Analyze behavioral data for a single question.
   */
  analyzeQuestion(
    behavioral: BehavioralData,
    questionDifficulty: number,
    answerLength: number,
  ): BehavioralAnalysis {
    const flags: string[] = [];
    const details: BehavioralAnalysis['details'] = {};
    let suspicionScore = 0;

    // 1. Time analysis
    const expectedMinTime = this.getExpectedMinTime(questionDifficulty);
    const timeRatio = behavioral.timeSpentSeconds / expectedMinTime;

    if (behavioral.timeSpentSeconds < this.thresholds.suspiciousFastAnswer && questionDifficulty > 0.5) {
      flags.push('TOO_FAST_FOR_HARD_QUESTION');
      suspicionScore += 30;
    }

    if (timeRatio < 0.3 && behavioral.timeSpentSeconds < 10) {
      flags.push('SUSPICIOUSLY_QUICK');
      suspicionScore += 20;
    }

    details['timeSpent'] = {
      value: behavioral.timeSpentSeconds,
      threshold: expectedMinTime,
      suspicious: behavioral.timeSpentSeconds < expectedMinTime * 0.5,
    };

    // 2. Copy-paste analysis
    if (behavioral.copyPasteEvents > 0) {
      const pasteRatio = answerLength > 0 ? behavioral.copyPasteEvents / answerLength : 0;

      if (behavioral.copyPasteEvents >= 3) {
        flags.push('MULTIPLE_COPY_PASTE');
        suspicionScore += 25;
      }

      if (answerLength > 100 && behavioral.copyPasteEvents === 1 && answerLength / behavioral.copyPasteEvents > 50) {
        // Single paste of large text
        flags.push('LARGE_PASTE_DETECTED');
        suspicionScore += 35;
      }

      // Check paste sources (if available)
      if (behavioral.pasteSources?.length) {
        const suspiciousSources = behavioral.pasteSources.filter((s) =>
          /stackoverflow|github|chatgpt|claude|bard|google/i.test(s),
        );
        if (suspiciousSources.length > 0) {
          flags.push('EXTERNAL_SOURCE_PASTE');
          suspicionScore += 40;
        }
      }
    }

    details['copyPaste'] = {
      value: behavioral.copyPasteEvents,
      threshold: this.thresholds.maxCopyPastePerTest,
      suspicious: behavioral.copyPasteEvents > this.thresholds.maxCopyPastePerTest,
    };

    // 3. Tab switching / focus loss
    if (behavioral.tabSwitches > this.thresholds.maxTabSwitchesPerTest) {
      flags.push('EXCESSIVE_TAB_SWITCHING');
      suspicionScore += 20;
    }

    if (behavioral.focusLossEvents && behavioral.focusLossEvents > 3) {
      flags.push('FREQUENT_FOCUS_LOSS');
      suspicionScore += 15;
    }

    details['tabSwitches'] = {
      value: behavioral.tabSwitches,
      threshold: this.thresholds.maxTabSwitchesPerTest,
      suspicious: behavioral.tabSwitches > this.thresholds.maxTabSwitchesPerTest,
    };

    // 4. Idle time analysis
    const idleRatio = behavioral.timeSpentSeconds > 0
      ? behavioral.idleTimeSeconds / behavioral.timeSpentSeconds
      : 0;

    if (idleRatio > this.thresholds.maxIdleRatio) {
      flags.push('EXCESSIVE_IDLE_TIME');
      suspicionScore += 15;
    }

    details['idleRatio'] = {
      value: Math.round(idleRatio * 100) / 100,
      threshold: this.thresholds.maxIdleRatio,
      suspicious: idleRatio > this.thresholds.maxIdleRatio,
    };

    // 5. Keystroke pattern analysis (if available)
    if (behavioral.keystrokeCount > 0 && answerLength > 0) {
      const keystrokeRatio = behavioral.keystrokeCount / answerLength;

      // Too few keystrokes for the answer length (pasted)
      if (keystrokeRatio < this.thresholds.minKeystrokesPerChar && answerLength > 50) {
        flags.push('LOW_KEYSTROKE_DENSITY');
        suspicionScore += 20;
      }
    }

    // Clamp suspicion score
    suspicionScore = Math.min(100, suspicionScore);

    return {
      suspicionScore,
      flags,
      details,
      confidence: this.calculateConfidence(flags, behavioral),
    };
  }

  /**
   * Analyze keystroke dynamics for identity verification.
   */
  analyzeKeystrokePattern(intervals: number[]): KeystrokePattern | null {
    if (intervals.length < 10) return null;

    const mean = intervals.reduce((a, b) => a + b, 0) / intervals.length;
    const variance = intervals.reduce((sum, x) => sum + Math.pow(x - mean, 2), 0) / intervals.length;
    const stdDev = Math.sqrt(variance);

    // Burstiness: ratio of stdDev to mean
    const burstiness = mean > 0 ? Math.min(1, stdDev / mean) : 0;

    // Rhythm consistency: how consistent is the typing rhythm
    // Low variance = high consistency
    const rhythmConsistency = mean > 0 ? Math.max(0, 1 - stdDev / mean) : 0;

    return {
      averageIntervalMs: Math.round(mean * 100) / 100,
      varianceMs: Math.round(variance * 100) / 100,
      burstiness: Math.round(burstiness * 100) / 100,
      rhythmConsistency: Math.round(rhythmConsistency * 100) / 100,
    };
  }

  /**
   * Analyze entire test session.
   */
  analyzeSession(
    questionAnalyses: BehavioralAnalysis[],
    thetaHistory: number[],
  ): BehavioralAnalysis {
    const allFlags = new Set<string>();
    let totalSuspicion = 0;

    for (const analysis of questionAnalyses) {
      totalSuspicion += analysis.suspicionScore;
      for (const flag of analysis.flags) {
        allFlags.add(flag);
      }
    }

    const avgSuspicion =
      questionAnalyses.length > 0 ? totalSuspicion / questionAnalyses.length : 0;

    // Additional session-level flags
    const flags = Array.from(allFlags);

    // Theta volatility check
    if (thetaHistory.length >= 4) {
      let jumps = 0;
      for (let i = 1; i < thetaHistory.length; i++) {
        if (Math.abs(thetaHistory[i] - thetaHistory[i - 1]) > 2) {
          jumps++;
        }
      }
      if (jumps > 2) {
        flags.push('THETA_VOLATILITY');
      }
    }

    // Multiple suspicious questions
    const suspiciousQuestions = questionAnalyses.filter((a) => a.suspicionScore > 50).length;
    if (suspiciousQuestions >= 3) {
      flags.push('MULTIPLE_SUSPICIOUS_QUESTIONS');
    }

    const finalSuspicion = Math.min(100, avgSuspicion * 1.5 + suspiciousQuestions * 10);

    return {
      suspicionScore: Math.round(finalSuspicion),
      flags,
      details: {},
      confidence: Math.min(1, flags.length * 0.2),
    };
  }

  private getExpectedMinTime(difficulty: number): number {
    if (difficulty < 0.3) return this.thresholds.minTimePerDifficulty.easy;
    if (difficulty < 0.6) return this.thresholds.minTimePerDifficulty.medium;
    return this.thresholds.minTimePerDifficulty.hard;
  }

  private calculateConfidence(flags: string[], behavioral: BehavioralData): number {
    let confidence = 0;

    // More data points = higher confidence
    if (behavioral.keystrokeCount > 0) confidence += 0.2;
    if (behavioral.copyPasteEvents !== undefined) confidence += 0.2;
    if (behavioral.tabSwitches !== undefined) confidence += 0.2;
    if (behavioral.timeSpentSeconds > 0) confidence += 0.2;
    if (behavioral.idleTimeSeconds !== undefined) confidence += 0.2;

    return Math.min(1, confidence);
  }
}