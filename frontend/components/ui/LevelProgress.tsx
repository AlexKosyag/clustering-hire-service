'use client';

import { theme } from '@/lib/theme';

interface LevelProgressProps {
  level: number;
  levelName: string;
  xp: number;
  xpToNextLevel: number;
  xpProgress: number; // 0-100
  compact?: boolean;
}

/**
 * Level progress indicator with XP bar.
 */
export default function LevelProgress({
  level,
  levelName,
  xp,
  xpToNextLevel,
  xpProgress,
  compact = false,
}: LevelProgressProps) {
  if (compact) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div
          style={{
            width: 40,
            height: 40,
            borderRadius: theme.radius.full,
            background: `linear-gradient(135deg, ${theme.colors.primary}, ${theme.colors.primaryDark})`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'white',
            fontSize: 16,
            fontWeight: 700,
          }}
        >
          {level}
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 13, fontWeight: 600, color: theme.colors.text }}>
            {levelName}
          </div>
          <div style={{ fontSize: 11, color: theme.colors.textMuted }}>
            {xp} / {xpToNextLevel} XP
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 12 }}>
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
            fontSize: 24,
            fontWeight: 700,
            boxShadow: `0 4px 12px ${theme.colors.primary}40`,
          }}
        >
          {level}
        </div>
        <div>
          <div style={{ fontSize: 18, fontWeight: 700, color: theme.colors.text }}>
            {levelName}
          </div>
          <div style={{ fontSize: 13, color: theme.colors.textSecondary }}>
            Уровень {level}
          </div>
        </div>
      </div>

      <div
        style={{
          height: 8,
          background: theme.colors.borderLight,
          borderRadius: 4,
          overflow: 'hidden',
          marginBottom: 8,
        }}
      >
        <div
          style={{
            width: `${xpProgress}%`,
            height: '100%',
            background: `linear-gradient(90deg, ${theme.colors.primary}, ${theme.colors.primaryLight})`,
            borderRadius: 4,
            transition: 'width 0.5s ease',
          }}
        />
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
        <span style={{ color: theme.colors.textSecondary }}>
          {xp} XP
        </span>
        <span style={{ color: theme.colors.textMuted }}>
          {xpToNextLevel - xp} XP до следующего уровня
        </span>
      </div>
    </div>
  );
}