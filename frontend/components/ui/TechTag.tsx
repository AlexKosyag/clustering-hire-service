'use client';

import { theme } from '@/lib/theme';

interface TechTagProps {
  tech: string;
  size?: 'sm' | 'md';
  emphasized?: boolean;  // Highlight as "match"
}

/**
 * Technology tag with optional emphasis for matches.
 */
export default function TechTag({ tech, size = 'md', emphasized = false }: TechTagProps) {
  const sizeStyles = {
    sm: { padding: '2px 6px', fontSize: 11 },
    md: { padding: '3px 10px', fontSize: 12 },
  }[size];

  return (
    <span
      style={{
        ...sizeStyles,
        borderRadius: 6,
        fontWeight: 500,
        display: 'inline-block',
        background: emphasized ? theme.colors.primaryBg : theme.colors.surfaceHover,
        color: emphasized ? theme.colors.primary : theme.colors.textSecondary,
        border: emphasized
          ? `1px solid ${theme.colors.primary}30`
          : `1px solid ${theme.colors.border}`,
      }}
    >
      {tech}
    </span>
  );
}