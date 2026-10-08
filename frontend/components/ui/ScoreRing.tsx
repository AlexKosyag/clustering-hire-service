'use client';

import { theme, getMatchColor } from '@/lib/theme';

interface ScoreRingProps {
  score: number;           // 0-100
  size?: number;           // px
  strokeWidth?: number;    // px
  showLabel?: boolean;
  label?: string;
  animated?: boolean;
}

/**
 * Circular progress indicator for match scores.
 * Color-coded: green (70+), yellow (40-69), red (<40).
 */
export default function ScoreRing({
  score,
  size = 64,
  strokeWidth = 5,
  showLabel = true,
  label,
  animated = true,
}: ScoreRingProps) {
  const radius = (size - strokeWidth) / 2;
  const circumference = radius * 2 * Math.PI;
  const offset = circumference - (score / 100) * circumference;
  const color = getMatchColor(score);

  return (
    <div
      style={{
        position: 'relative',
        width: size,
        height: size,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <svg
        width={size}
        height={size}
        style={{
          transform: 'rotate(-90deg)',
          transition: animated ? theme.transitions.normal : 'none',
        }}
      >
        {/* Background circle */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={theme.colors.border}
          strokeWidth={strokeWidth}
        />
        {/* Progress circle */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          style={{
            transition: animated ? theme.transitions.normal : 'none',
          }}
        />
      </svg>
      {showLabel && (
        <div
          style={{
            position: 'absolute',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            lineHeight: 1,
          }}
        >
          <span
            style={{
              fontSize: size / 4,
              fontWeight: 700,
              color,
            }}
          >
            {Math.round(score)}
          </span>
          {label && (
            <span
              style={{
                fontSize: size / 8,
                color: theme.colors.textMuted,
                marginTop: 2,
              }}
            >
              {label}
            </span>
          )}
        </div>
      )}
    </div>
  );
}