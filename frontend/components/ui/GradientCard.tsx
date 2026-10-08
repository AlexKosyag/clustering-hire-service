'use client';

import { ReactNode } from 'react';
import { theme } from '@/lib/theme';

interface GradientCardProps {
  children: ReactNode;
  gradient?: string;
  className?: string;
  style?: React.CSSProperties;
  hoverable?: boolean;
}

/**
 * Card with optional gradient header.
 */
export default function GradientCard({
  children,
  gradient,
  className,
  style,
  hoverable = false,
}: GradientCardProps) {
  return (
    <div
      className={className}
      style={{
        background: theme.colors.surface,
        borderRadius: theme.radius.lg,
        border: `1px solid ${theme.colors.border}`,
        overflow: 'hidden',
        transition: theme.transitions.normal,
        boxShadow: theme.shadows.sm,
        ...(hoverable && {
          cursor: 'pointer',
          '&:hover': {
            boxShadow: theme.shadows.lg,
            transform: 'translateY(-2px)',
          },
        }),
        ...style,
      }}
    >
      {gradient && (
        <div
          style={{
            height: 4,
            background: gradient,
          }}
        />
      )}
      {children}
    </div>
  );
}

/**
 * Card with header section.
 */
export function CardHeader({
  title,
  subtitle,
  action,
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'flex-start',
        padding: `${theme.spacing.lg} ${theme.spacing.xl}`,
        borderBottom: `1px solid ${theme.colors.borderLight}`,
      }}
    >
      <div>
        <div style={{ fontWeight: 600, fontSize: 16, color: theme.colors.text }}>
          {title}
        </div>
        {subtitle && (
          <div style={{ fontSize: 13, color: theme.colors.textMuted, marginTop: 4 }}>
            {subtitle}
          </div>
        )}
      </div>
      {action}
    </div>
  );
}

/**
 * Card body section.
 */
export function CardBody({
  children,
  style,
}: {
  children: ReactNode;
  style?: React.CSSProperties;
}) {
  return (
    <div style={{ padding: theme.spacing.xl, ...style }}>{children}</div>
  );
}