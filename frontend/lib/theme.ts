/**
 * FSP Platform Design System — Theme Tokens
 *
 * Modern, clean design inspired by Linear, Vercel, and Notion.
 * Primary color: Blue (ФСП branding)
 */

export const theme = {
  // ─── Colors ───
  colors: {
    // Primary
    primary: '#2563eb',
    primaryLight: '#3b82f6',
    primaryDark: '#1d4ed8',
    primaryBg: '#eff6ff',

    // Semantic
    success: '#10b981',
    successLight: '#d1fae5',
    warning: '#f59e0b',
    warningLight: '#fef3c7',
    error: '#ef4444',
    errorLight: '#fee2e2',
    info: '#3b82f6',
    infoLight: '#dbeafe',

    // Neutrals
    bg: '#f8fafc',
    surface: '#ffffff',
    surfaceHover: '#f1f5f9',
    border: '#e2e8f0',
    borderLight: '#f1f5f9',

    // Text
    text: '#0f172a',
    textSecondary: '#475569',
    textMuted: '#94a3b8',
    textInverse: '#ffffff',

    // Grades
    gradeJunior: '#22c55e',
    gradeMiddle: '#3b82f6',
    gradeMiddlePlus: '#8b5cf6',
    gradeSenior: '#f59e0b',
    gradeLead: '#ef4444',
    gradePrincipal: '#ec4899',

    // Match score
    matchHigh: '#10b981',    // 70-100
    matchMedium: '#f59e0b',  // 40-69
    matchLow: '#ef4444',     // 0-39
  },

  // ─── Shadows ───
  shadows: {
    sm: '0 1px 2px rgba(0,0,0,0.05)',
    md: '0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1)',
    lg: '0 10px 15px -3px rgba(0,0,0,0.1), 0 4px 6px -4px rgba(0,0,0,0.1)',
    xl: '0 20px 25px -5px rgba(0,0,0,0.1), 0 8px 10px -6px rgba(0,0,0,0.1)',
    none: 'none',
  },

  // ─── Border Radius ───
  radius: {
    sm: '6px',
    md: '10px',
    lg: '16px',
    xl: '24px',
    full: '9999px',
  },

  // ─── Transitions ───
  transitions: {
    fast: '150ms cubic-bezier(0.4, 0, 0.2, 1)',
    normal: '300ms cubic-bezier(0.4, 0, 0.2, 1)',
    slow: '500ms cubic-bezier(0.4, 0, 0.2, 1)',
  },

  // ─── Typography ───
  fonts: {
    sans: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
    mono: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
  },

  // ─── Spacing ───
  spacing: {
    xs: '4px',
    sm: '8px',
    md: '12px',
    lg: '16px',
    xl: '24px',
    xxl: '32px',
  },
} as const;

// ─── Helper Functions ───

export function getGradeColor(grade: string | null | undefined): string {
  if (!grade) return theme.colors.textMuted;
  const map: Record<string, string> = {
    JUNIOR: theme.colors.gradeJunior,
    MIDDLE: theme.colors.gradeMiddle,
    MIDDLE_PLUS: theme.colors.gradeMiddlePlus,
    SENIOR: theme.colors.gradeSenior,
    LEAD: theme.colors.gradeLead,
    PRINCIPAL: theme.colors.gradePrincipal,
  };
  return map[grade] || theme.colors.textMuted;
}

export function getGradeLabel(grade: string | null | undefined): string {
  if (!grade) return '—';
  const map: Record<string, string> = {
    JUNIOR: 'Junior',
    MIDDLE: 'Middle',
    MIDDLE_PLUS: 'Middle+',
    SENIOR: 'Senior',
    LEAD: 'Lead',
    PRINCIPAL: 'Principal',
  };
  return map[grade] || grade;
}

export function getMatchColor(score: number | null | undefined): string {
  if (score == null) return theme.colors.textMuted;
  if (score >= 70) return theme.colors.matchHigh;
  if (score >= 40) return theme.colors.matchMedium;
  return theme.colors.matchLow;
}

export function getSpecLabel(spec: string | null | undefined): string {
  if (!spec) return '—';
  const map: Record<string, string> = {
    BACKEND: 'Backend',
    FRONTEND: 'Frontend',
    FULLSTACK: 'Fullstack',
    MOBILE: 'Mobile',
    DEVOPS: 'DevOps',
    DATA_SCIENCE: 'Data Science',
    QA: 'QA',
    SECURITY: 'Security',
    EMBEDDED: 'Embedded',
    GAME_DEV: 'Game Dev',
    ML_AI: 'ML / AI',
    DESIGN_UX: 'Design / UX',
    MANAGEMENT: 'Management',
  };
  return map[spec] || spec;
}

export function formatSalary(from: number, to: number): string {
  const fmt = (n: number) => {
    if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
    if (n >= 1000) return `${Math.round(n / 1000)}K`;
    return String(n);
  };
  return `${fmt(from)} – ${fmt(to)} ₽`;
}