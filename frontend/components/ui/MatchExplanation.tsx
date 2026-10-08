'use client';

import { useState } from 'react';
import { theme, getMatchColor } from '@/lib/theme';
import ScoreRing from './ScoreRing';

interface ExplanationFactor {
  factor: string;
  match: boolean | string | number;
  weight: number;
  explanation: string;
  overlap?: string[];
  detail?: string;
}

interface MatchExplanationProps {
  score: number;
  overall?: string;
  details: ExplanationFactor[];
  breakdown?: Record<string, { score: number; weight: number; contribution: number }>;
  semanticSimilarity?: number;
  rerankerScore?: number | null;
  compact?: boolean;
}

/**
 * Detailed match explanation with score breakdown.
 * Shows SHAP-like feature attribution.
 */
export default function MatchExplanation({
  score,
  overall,
  details,
  breakdown,
  semanticSimilarity,
  rerankerScore,
  compact = false,
}: MatchExplanationProps) {
  const [expanded, setExpanded] = useState(false);
  const color = getMatchColor(score);

  if (compact) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <ScoreRing score={score} size={48} strokeWidth={4} />
        <div>
          <div style={{ fontWeight: 600, color }}>{overall || getSummary(score)}</div>
          <div style={{ fontSize: 12, color: theme.colors.textMuted }}>
            {details.length} факторов · {Math.round(score)}%
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      {/* Header with score */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16 }}>
        <ScoreRing score={score} size={72} strokeWidth={6} />
        <div>
          <div style={{ fontSize: 18, fontWeight: 600, color }}>
            {overall || getSummary(score)}
          </div>
          <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
            Совпадение: <b style={{ color }}>{Math.round(score)}%</b>
          </div>
          {semanticSimilarity != null && (
            <div style={{ fontSize: 12, color: theme.colors.textMuted }}>
              Семантика: {Math.round(semanticSimilarity * 100)}%
              {rerankerScore != null && ` · Re-rank: ${rerankerScore.toFixed(2)}`}
            </div>
          )}
        </div>
      </div>

      {/* Breakdown bars (V2) */}
      {breakdown && (
        <div style={{ marginBottom: 16 }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: theme.colors.textSecondary, marginBottom: 8 }}>
            Разбивка score
          </div>
          {Object.entries(breakdown).map(([key, data]) => (
            <div
              key={key}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                marginBottom: 6,
              }}
            >
              <div style={{ width: 100, fontSize: 12, color: theme.colors.textSecondary }}>
                {getComponentLabel(key)}
              </div>
              <div
                style={{
                  flex: 1,
                  height: 6,
                  background: theme.colors.borderLight,
                  borderRadius: 3,
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    width: `${data.score}%`,
                    height: '100%',
                    background: getMatchColor(data.score),
                    borderRadius: 3,
                    transition: theme.transitions.normal,
                  }}
                />
              </div>
              <div style={{ width: 40, fontSize: 12, color: theme.colors.textMuted, textAlign: 'right' }}>
                {data.score}%
              </div>
              <div style={{ width: 40, fontSize: 11, color: theme.colors.textMuted, textAlign: 'right' }}>
                +{data.contribution}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Detailed factors */}
      <div>
        <button
          onClick={() => setExpanded(!expanded)}
          style={{
            background: 'none',
            border: 'none',
            color: theme.colors.primary,
            fontSize: 13,
            cursor: 'pointer',
            padding: 0,
            fontWeight: 500,
          }}
        >
          {expanded ? 'Скрыть детали' : 'Показать детали'}
        </button>

        {expanded && (
          <div style={{ marginTop: 12 }}>
            {details.map((factor, idx) => (
              <div
                key={idx}
                style={{
                  padding: '10px 12px',
                  background: theme.colors.bg,
                  borderRadius: theme.radius.sm,
                  marginBottom: 8,
                  borderLeft: `3px solid ${
                    factor.match === true || factor.match === 'да'
                      ? theme.colors.success
                      : factor.match === false
                      ? theme.colors.error
                      : theme.colors.textMuted
                  }`,
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                  <span style={{ fontWeight: 600, fontSize: 13, color: theme.colors.text }}>
                    {factor.factor}
                  </span>
                  <span style={{ fontSize: 12, color: theme.colors.textMuted }}>
                    вес: {Math.round(factor.weight * 100)}%
                  </span>
                </div>
                <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
                  {factor.explanation}
                </div>
                {factor.overlap && factor.overlap.length > 0 && (
                  <div style={{ display: 'flex', gap: 4, marginTop: 6, flexWrap: 'wrap' }}>
                    {factor.overlap.map((tech) => (
                      <span
                        key={tech}
                        style={{
                          padding: '2px 6px',
                          background: theme.colors.primaryBg,
                          color: theme.colors.primary,
                          borderRadius: 4,
                          fontSize: 11,
                          fontWeight: 500,
                        }}
                      >
                        {tech}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function getSummary(score: number): string {
  if (score >= 80) return 'Сильное совпадение';
  if (score >= 60) return 'Хорошее совпадение';
  if (score >= 40) return 'Частичное совпадение';
  return 'Слабое совпадение';
}

function getComponentLabel(key: string): string {
  const map: Record<string, string> = {
    specialization: 'Специализация',
    stack: 'Стек',
    semantic: 'Семантика',
    grade: 'Грейд',
    achievement: 'Достижения',
    experience: 'Опыт',
    testScore: 'Тест',
    rerank: 'Re-rank',
  };
  return map[key] || key;
}