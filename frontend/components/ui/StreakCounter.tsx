'use client';

import { theme } from '@/lib/theme';

interface StreakCounterProps {
  streak: number;
  longestStreak?: number;
  multiplier?: number;
  compact?: boolean;
}

/**
 * Fire streak counter with multiplier display.
 */
export default function StreakCounter({
  streak,
  longestStreak,
  multiplier = 1.0,
  compact = false,
}: StreakCounterProps) {
  const getFireIntensity = () => {
    if (streak >= 30) return '🔥🔥🔥';
    if (streak >= 7) return '🔥🔥';
    if (streak >= 3) return '🔥';
    return '💫';
  };

  if (compact) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{ fontSize: 24 }}>{getFireIntensity()}</span>
        <div>
          <div style={{ fontSize: 18, fontWeight: 700, color: theme.colors.text }}>
            {streak}
          </div>
          <div style={{ fontSize: 11, color: theme.colors.textMuted }}>дней</div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ textAlign: 'center' }}>
      <div style={{ fontSize: 48, marginBottom: 8 }}>{getFireIntensity()}</div>
      <div
        style={{
          fontSize: 32,
          fontWeight: 700,
          color: streak >= 7 ? theme.colors.warning : theme.colors.text,
        }}
      >
        {streak}
      </div>
      <div style={{ fontSize: 13, color: theme.colors.textSecondary, marginBottom: 8 }}>
        дней подряд
      </div>

      {multiplier > 1 && (
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            padding: '4px 12px',
            background: theme.colors.warningLight,
            borderRadius: theme.radius.full,
            fontSize: 13,
            fontWeight: 600,
            color: theme.colors.warning,
          }}
        >
          ⚡ x{multiplier} XP
        </div>
      )}

      {longestStreak && longestStreak > streak && (
        <div style={{ fontSize: 11, color: theme.colors.textMuted, marginTop: 8 }}>
          Рекорд: {longestStreak} дней
        </div>
      )}
    </div>
  );
}