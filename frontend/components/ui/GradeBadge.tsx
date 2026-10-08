'use client';

import { theme, getGradeColor, getGradeLabel } from '@/lib/theme';

interface GradeBadgeProps {
  grade: string | null | undefined;
  verified?: boolean;    // If true, shows checkmark
  size?: 'sm' | 'md' | 'lg';
  showLabel?: boolean;
}

/**
 * Color-coded grade badge.
 * Verified grades show a checkmark icon.
 */
export default function GradeBadge({
  grade,
  verified = false,
  size = 'md',
  showLabel = true,
}: GradeBadgeProps) {
  const color = getGradeColor(grade);
  const label = getGradeLabel(grade);

  const sizeStyles = {
    sm: { padding: '2px 6px', fontSize: 11, borderRadius: 4 },
    md: { padding: '3px 10px', fontSize: 12, borderRadius: 6 },
    lg: { padding: '5px 14px', fontSize: 14, borderRadius: 8 },
  }[size];

  if (!grade) {
    return (
      <span
        style={{
          ...sizeStyles,
          background: theme.colors.borderLight,
          color: theme.colors.textMuted,
          fontWeight: 500,
        }}
      >
        —
      </span>
    );
  }

  return (
    <span
      style={{
        ...sizeStyles,
        background: `${color}15`,
        color,
        fontWeight: 600,
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
      }}
    >
      {verified && (
        <svg width={12} height={12} viewBox="0 0 12 12" fill="none">
          <path
            d="M10 3L4.5 8.5L2 6"
            stroke={color}
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      )}
      {showLabel && label}
    </span>
  );
}