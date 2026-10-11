/**
 * FSP Platform Design System — Theme Tokens
 *
 * Фирменный стиль Федерации спортивного программирования (брендбук ФСП).
 * Тёмная база #212330, синий логотипа #402FFD, сквозной красный #EC1D35.
 * Цвета грейдов и дисциплин — по цветовому коду дисциплин брендбука:
 *   фиолетовый — продуктовое, синий — алгоритмическое, бирюзовый — БАС,
 *   жёлтый — робототехника, зелёный — информационная безопасность,
 *   красный — сквозной цвет.
 */

export const theme = {
  // ─── Colors ───
  colors: {
    // Primary — синий логотипа ФСП
    primary: '#402ffd',
    primaryLight: '#6a5cff',
    primaryDark: '#2e1fd6',
    primaryBg: '#282650',

    // Brand accents
    accent: '#ec1d35', // сквозной красный
    accentLight: '#3a2434',
    violet: '#8f1bf4',
    turquoise: '#1ed3ca',
    yellow: '#ecb617',
    green: '#09a953',
    deepBlue: '#0a006b',

    // Semantic
    success: '#09a953',
    successLight: '#123326',
    warning: '#ecb617',
    warningLight: '#332b18',
    error: '#ec1d35',
    errorLight: '#3a2434',
    info: '#402ffd',
    infoLight: '#282650',

    // Neutrals — тёмная база брендбука
    bg: '#212330',
    surface: '#2a2d3e',
    surfaceHover: '#333749',
    border: '#3a3e52',
    borderLight: '#2f3344',

    // Text
    text: '#ffffff',
    textSecondary: '#c7cad6',
    textMuted: '#8a8fa3',
    textInverse: '#212330',

    // Grades — цветовой код дисциплин ФСП
    gradeJunior: '#09a953',     // зелёный
    gradeMiddle: '#1ed3ca',     // бирюзовый
    gradeMiddlePlus: '#402ffd', // синий
    gradeSenior: '#ecb617',     // жёлтый
    gradeLead: '#8f1bf4',       // фиолетовый
    gradePrincipal: '#ec1d35',  // красный

    // Match score
    matchHigh: '#09a953',    // 70-100
    matchMedium: '#ecb617',  // 40-69
    matchLow: '#ec1d35',     // 0-39
  },

  // ─── Shadows ───
  shadows: {
    sm: '0 1px 2px rgba(0,0,0,0.30)',
    md: '0 4px 6px -1px rgba(0,0,0,0.35), 0 2px 4px -2px rgba(0,0,0,0.30)',
    lg: '0 10px 15px -3px rgba(0,0,0,0.40), 0 4px 6px -4px rgba(0,0,0,0.35)',
    xl: '0 20px 25px -5px rgba(0,0,0,0.45), 0 8px 10px -6px rgba(0,0,0,0.40)',
    none: 'none',
  },

  // ─── Border Radius ───
  radius: {
    sm: '4px',
    md: '6px',
    lg: '10px',
    xl: '16px',
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
    // Основной шрифт — JetBrains Mono
    sans: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
    mono: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
    // Акцентный шрифт — TDAText (заголовки, капс-лейблы)
    display: "'TDAText', 'JetBrains Mono', ui-monospace, monospace",
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

/**
 * Цвет специализации по цветовому коду дисциплин брендбука ФСП.
 * Для специализаций вне спортивных дисциплин — нейтральный акцент.
 */
export function getSpecColor(spec: string | null | undefined): string {
  if (!spec) return theme.colors.textMuted;
  const map: Record<string, string> = {
    SECURITY: theme.colors.green,      // ИБ — зелёный
    EMBEDDED: theme.colors.yellow,     // робототехника/встраиваемые — жёлтый
    GAME_DEV: theme.colors.violet,     // продуктовая разработка — фиолетовый
    DESIGN_UX: theme.colors.violet,
    MOBILE: theme.colors.turquoise,    // БАС — бирюзовый
    DEVOPS: theme.colors.turquoise,
    DATA_SCIENCE: theme.colors.violet,
    ML_AI: theme.colors.violet,
    BACKEND: theme.colors.primary,     // алгоритмическое — синий
    FRONTEND: theme.colors.primary,
    FULLSTACK: theme.colors.primary,
    QA: theme.colors.turquoise,
    MANAGEMENT: theme.colors.accent,
  };
  return map[spec] || theme.colors.primary;
}

export function formatSalary(from: number, to: number): string {
  const fmt = (n: number) => {
    if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
    if (n >= 1000) return `${Math.round(n / 1000)}K`;
    return String(n);
  };
  return `${fmt(from)} – ${fmt(to)} ₽`;
}
