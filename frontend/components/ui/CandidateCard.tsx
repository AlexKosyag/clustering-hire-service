'use client';

import { theme, getGradeColor, getGradeLabel, getSpecLabel } from '@/lib/theme';
import GradeBadge from './GradeBadge';
import ScoreRing from './ScoreRing';
import TechTag from './TechTag';

interface CandidateCardProps {
  candidate: {
    id: string;
    displayName?: string;
    specialization?: string | null;
    verifiedGrade?: string | null;
    declaredGrade?: string | null;
    stack?: string[];
    experienceYears?: number | null;
    city?: string | null;
    testStatus?: string;
    testScore?: number | null;
    achievements?: Array<{ name: string; points: number }>;
    matchScore?: number;
    semanticSimilarity?: number;
    rerankerScore?: number | null;
    matchExplanation?: {
      overall?: string;
      details?: Array<{
        factor: string;
        match: boolean | string;
        explanation: string;
        overlap?: string[];
      }>;
    };
  };
  onInvite?: (candidateId: string) => void;
  onDetails?: (candidateId: string) => void;
}

/**
 * Candidate card for employer search results.
 * Shows match score, key info, and quick actions.
 */
export default function CandidateCard({
  candidate,
  onInvite,
  onDetails,
}: CandidateCardProps) {
  const matchScore = candidate.matchScore ?? 0;
  const topMatches = getTopMatches(candidate);

  return (
    <div
      style={{
        background: theme.colors.surface,
        border: `1px solid ${theme.colors.border}`,
        borderRadius: theme.radius.lg,
        padding: theme.spacing.xl,
        transition: theme.transitions.normal,
        cursor: 'pointer',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = theme.shadows.lg;
        e.currentTarget.style.borderColor = theme.colors.primary;
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = theme.shadows.sm;
        e.currentTarget.style.borderColor = theme.colors.border;
      }}
      onClick={() => onDetails?.(candidate.id)}
    >
      <div style={{ display: 'flex', gap: 16 }}>
        {/* Avatar + Score */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
          <div
            style={{
              width: 56,
              height: 56,
              borderRadius: theme.radius.full,
              background: `linear-gradient(135deg, ${theme.colors.primary}, ${theme.colors.primaryDark})`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'white',
              fontSize: 20,
              fontWeight: 700,
            }}
          >
              {candidate.displayName?.charAt(0) || '?'}
          </div>
          <ScoreRing score={matchScore} size={48} strokeWidth={4} label="%" />
        </div>

        {/* Main content */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {/* Name + badges */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
            <span style={{ fontWeight: 700, fontSize: 16, color: theme.colors.text }}>
              {candidate.displayName || 'Кандидат'}
            </span>
            {candidate.specialization && (
              <span
                style={{
                  padding: '2px 8px',
                  background: theme.colors.primaryBg,
                  color: theme.colors.primary,
                  borderRadius: 6,
                  fontSize: 12,
                  fontWeight: 600,
                }}
              >
                {getSpecLabel(candidate.specialization)}
              </span>
            )}
            {candidate.verifiedGrade && (
              <GradeBadge grade={candidate.verifiedGrade} verified size="sm" />
            )}
            {candidate.declaredGrade && !candidate.verifiedGrade && (
              <GradeBadge grade={candidate.declaredGrade} size="sm" />
            )}
          </div>

          {/* Meta info */}
          <div
            style={{
              display: 'flex',
              gap: 16,
              fontSize: 13,
              color: theme.colors.textSecondary,
              marginBottom: 10,
            }}
          >
            {candidate.experienceYears != null && (
              <span>📋 {candidate.experienceYears} лет опыта</span>
            )}
            {candidate.city && <span>📍 {candidate.city}</span>}
            <span>
              🧪 {candidate.testStatus === 'PASSED' ? 'Тест пройден' : 'Тест не пройден'}
              {candidate.testScore != null && ` (${candidate.testScore} баллов)`}
            </span>
          </div>

          {/* Stack */}
          {candidate.stack && candidate.stack.length > 0 && (
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 10 }}>
              {candidate.stack.slice(0, 6).map((tech) => (
                <TechTag
                  key={tech}
                  tech={tech}
                  size="sm"
                  emphasized={topMatches.includes(tech)}
                />
              ))}
              {candidate.stack.length > 6 && (
                <span style={{ fontSize: 12, color: theme.colors.textMuted }}>
                  +{candidate.stack.length - 6}
                </span>
              )}
            </div>
          )}

          {/* Achievements */}
          {candidate.achievements && candidate.achievements.length > 0 && (
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 10 }}>
              {candidate.achievements.slice(0, 3).map((ach, idx) => (
                <span
                  key={idx}
                  style={{
                    padding: '2px 8px',
                    background: theme.colors.warningLight,
                    color: theme.colors.warning,
                    borderRadius: 6,
                    fontSize: 11,
                    fontWeight: 500,
                  }}
                >
                  🏆 {ach.name}
                </span>
              ))}
            </div>
          )}

          {/* Actions */}
          <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onInvite?.(candidate.id);
              }}
              style={{
                padding: '6px 16px',
                background: theme.colors.primary,
                color: 'white',
                border: 'none',
                borderRadius: theme.radius.sm,
                fontSize: 13,
                fontWeight: 500,
                cursor: 'pointer',
                transition: theme.transitions.fast,
              }}
            >
              Пригласить
            </button>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onDetails?.(candidate.id);
              }}
              style={{
                padding: '6px 16px',
                background: 'transparent',
                color: theme.colors.textSecondary,
                border: `1px solid ${theme.colors.border}`,
                borderRadius: theme.radius.sm,
                fontSize: 13,
                fontWeight: 500,
                cursor: 'pointer',
                transition: theme.transitions.fast,
              }}
            >
              Подробнее
            </button>
          </div>
        </div>

        {/* Match score (right) */}
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'flex-end',
            gap: 8,
            minWidth: 120,
          }}
        >
          {candidate.matchExplanation?.overall && (
            <span
              style={{
                fontSize: 13,
                fontWeight: 600,
                color:
                  matchScore >= 70
                    ? theme.colors.success
                    : matchScore >= 40
                    ? theme.colors.warning
                    : theme.colors.error,
              }}
            >
              {candidate.matchExplanation.overall}
            </span>
          )}
          {candidate.semanticSimilarity != null && (
            <span style={{ fontSize: 11, color: theme.colors.textMuted }}>
              Семантика: {Math.round(candidate.semanticSimilarity * 100)}%
            </span>
          )}
          {candidate.rerankerScore != null && (
            <span style={{ fontSize: 11, color: theme.colors.textMuted }}>
              Re-rank: {candidate.rerankerScore.toFixed(2)}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

function getTopMatches(candidate: CandidateCardProps['candidate']): string[] {
  if (!candidate.matchExplanation?.details) return [];
  const stackFactor = candidate.matchExplanation.details.find(
    (d) => d.factor === 'Стек',
  );
  return stackFactor?.overlap || [];
}