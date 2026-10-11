'use client';

import { theme } from '@/lib/theme';

interface BadgeDisplayProps {
  badge: {
    code: string;
    name: string;
    description: string;
    icon: string;
    category: string;
    rarity: string;
    earnedAt?: Date | string;
  };
  size?: 'sm' | 'md' | 'lg';
  showDescription?: boolean;
}

const RARITY_CONFIG: Record<string, { bg: string; border: string; label: string }> = {
  common: {
    bg: theme.colors.borderLight,
    border: theme.colors.border,
    label: 'Обычный',
  },
  rare: {
    bg: theme.colors.infoLight,
    border: theme.colors.info,
    label: 'Редкий',
  },
  epic: {
    bg: '#2f2440',
    border: theme.colors.violet,
    label: 'Эпический',
  },
  legendary: {
    bg: theme.colors.warningLight,
    border: theme.colors.warning,
    label: 'Легендарный',
  },
};

/**
 * Badge display with rarity styling.
 */
export default function BadgeDisplay({
  badge,
  size = 'md',
  showDescription = false,
}: BadgeDisplayProps) {
  const rarity = RARITY_CONFIG[badge.rarity] || RARITY_CONFIG.common;
  const earned = !!badge.earnedAt;

  const sizeStyles = {
    sm: { padding: '8px 12px', iconSize: 24, nameSize: 12 },
    md: { padding: '12px 16px', iconSize: 32, nameSize: 14 },
    lg: { padding: '16px 20px', iconSize: 40, nameSize: 16 },
  }[size];

  return (
    <div
      style={{
        padding: sizeStyles.padding,
        background: earned ? rarity.bg : theme.colors.borderLight,
        border: `2px solid ${earned ? rarity.border : theme.colors.border}`,
        borderRadius: theme.radius.md,
        opacity: earned ? 1 : 0.5,
        filter: earned ? 'none' : 'grayscale(100%)',
        transition: theme.transitions.normal,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <span style={{ fontSize: sizeStyles.iconSize }}>{badge.icon}</span>
        <div style={{ flex: 1 }}>
          <div
            style={{
              fontSize: sizeStyles.nameSize,
              fontWeight: 600,
              color: theme.colors.text,
              marginBottom: 2,
            }}
          >
            {badge.name}
          </div>
          {showDescription && (
            <div style={{ fontSize: 12, color: theme.colors.textSecondary }}>
              {badge.description}
            </div>
          )}
          {earned && (
            <div
              style={{
                fontSize: 10,
                color: rarity.border,
                fontWeight: 600,
                textTransform: 'uppercase',
                marginTop: 4,
              }}
            >
              {rarity.label}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}