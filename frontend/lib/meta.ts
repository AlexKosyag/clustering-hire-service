/** Справочники платформы (соответствуют enum-ам Prisma). */

export const SPECIALIZATIONS: Array<{ value: string; label: string }> = [
  { value: 'BACKEND', label: 'Backend' },
  { value: 'FRONTEND', label: 'Frontend' },
  { value: 'FULLSTACK', label: 'Fullstack' },
  { value: 'MOBILE', label: 'Mobile' },
  { value: 'DEVOPS', label: 'DevOps' },
  { value: 'DATA_SCIENCE', label: 'Data Science' },
  { value: 'QA', label: 'QA' },
  { value: 'SECURITY', label: 'Security' },
  { value: 'EMBEDDED', label: 'Embedded' },
  { value: 'GAME_DEV', label: 'Game Dev' },
  { value: 'ML_AI', label: 'ML / AI' },
  { value: 'DESIGN_UX', label: 'Design / UX' },
  { value: 'MANAGEMENT', label: 'Management' },
];

export const GRADES: Array<{ value: string; label: string }> = [
  { value: 'JUNIOR', label: 'Junior' },
  { value: 'MIDDLE', label: 'Middle' },
  { value: 'MIDDLE_PLUS', label: 'Middle+' },
  { value: 'SENIOR', label: 'Senior' },
  { value: 'LEAD', label: 'Lead' },
  { value: 'PRINCIPAL', label: 'Principal' },
];

export const specLabel = (value?: string | null): string =>
  SPECIALIZATIONS.find((s) => s.value === value)?.label ?? '—';

export const gradeLabel = (value?: string | null): string =>
  GRADES.find((g) => g.value === value)?.label ?? '—';

export const gradeColor = (value?: string | null): string =>
  ({
    JUNIOR: 'green',
    MIDDLE: 'blue',
    MIDDLE_PLUS: 'geekblue',
    SENIOR: 'purple',
    LEAD: 'volcano',
    PRINCIPAL: 'magenta',
  })[value as string] || 'default';

export const money = (value?: number | null): string =>
  value ? `${new Intl.NumberFormat('ru-RU').format(value)} ₽` : '—';

export const salaryRange = (from?: number | null, to?: number | null): string => {
  if (!from && !to) return 'з/д по договорённости';
  if (from && to) return `${money(from)} — ${money(to)}`;
  return money(from || to);
};

export const dateRu = (value?: string | null): string =>
  value ? new Date(value).toLocaleDateString('ru-RU') : '—';

export const plural = (n: number, forms: [string, string, string]): string => {
  const abs = Math.abs(n) % 100;
  const n1 = abs % 10;
  if (abs > 10 && abs < 20) return forms[2];
  if (n1 > 1 && n1 < 5) return forms[1];
  if (n1 === 1) return forms[0];
  return forms[2];
};
