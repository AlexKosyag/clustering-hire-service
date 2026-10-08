/**
 * Автоматизированная валидация платформы ФСП (docs/VALIDATION.md).
 *
 *   npx ts-node --transpile-only scripts/metrics.ts
 *
 * Секции:
 *   1. Smoke        — health, frontend, Swagger, прокси /api через frontend
 *   2. Security     — 401/403/404/4xx, SQL-injection, утечка данных
 *   3. Matching     — precision@5/10, recall@10, MRR, nDCG@10
 *   4. Testing      — дискриминативность (сильный/слабый), test-retest, cooldown
 *   5. Performance  — avg/p50/p95 по ключевым endpoint'ам
 *
 * Требует запущенный backend (API_URL) и frontend (FRONTEND_URL).
 * Результат записывается в docs/VALIDATION_REPORT.md.
 */
import { PrismaClient } from '@prisma/client';
import * as fs from 'fs';
import * as path from 'path';

const API = (process.env.API_URL || 'http://localhost:3000/api').replace(/\/+$/, '');
const FRONTEND = (process.env.FRONTEND_URL || 'http://localhost:8080').replace(/\/+$/, '');
const REPORT = path.resolve(__dirname, '../../docs/VALIDATION_REPORT.md');
const RUN_AT = new Date();

const prisma = new PrismaClient();

interface Check {
  name: string;
  ok: boolean;
  detail: string;
}
interface Section {
  title: string;
  checks: Check[];
  extra?: string;
}

const sections: Section[] = [];
const add = (title: string, checks: Check[], extra?: string) => sections.push({ title, checks, extra });

const norm = (value: string) =>
  (value || '').toLowerCase().replace(/[.\s-]+/g, '');

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
const rnd = (min: number, max: number) => Math.round(min + Math.random() * (max - min));

interface HttpResult<T = any> {
  status: number;
  body: T;
  ms: number;
}

async function call<T = any>(
  url: string,
  options: { method?: string; token?: string; body?: unknown; headers?: Record<string, string> } = {},
): Promise<HttpResult<T>> {
  const started = Date.now();
  try {
    const response = await fetch(url, {
      method: options.method || 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...(options.token ? { Authorization: `Bearer ${options.token}` } : {}),
        ...(options.headers || {}),
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
    const text = await response.text();
    let body: any = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch {
      body = text;
    }
    return { status: response.status, body, ms: Date.now() - started };
  } catch (error: any) {
    return { status: 0, body: { message: error?.message || String(error) } as unknown as T, ms: Date.now() - started };
  }
}

async function register(role: 'CANDIDATE' | 'EMPLOYER') {
  const email = `metrics-${role.toLowerCase()}-${stamp()}@example.com`;
  const password = 'Metrics123!';
  const res = await call(`${API}/auth/register`, { method: 'POST', body: { email, password, role } });
  if (res.status !== 201 && res.status !== 200) throw new Error(`register failed: ${res.status}`);
  return { email, password, token: res.body.accessToken as string, userId: res.body.user.id as string };
}

function stamp() {
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

const GRADE_ORDER = ['JUNIOR', 'MIDDLE', 'MIDDLE_PLUS', 'SENIOR', 'LEAD', 'PRINCIPAL'];
const gradeIdx = (g?: string | null) => (g ? GRADE_ORDER.indexOf(g) : -1);

// ============================================================================
// 1. SMOKE
// ============================================================================
async function smokeSection(): Promise<void> {
  const checks: Check[] = [];

  const health = await call(`${API}/health`);
  checks.push({
    name: 'GET /api/health',
    ok: health.status === 200 && health.body?.status === 'ok',
    detail: `status=${health.status}, db=${health.body?.services?.database}, dim=${health.body?.services?.embedding?.dim}`,
  });

  const docs = await call(`${API}/docs-json`);
  checks.push({
    name: 'Swagger JSON публикуется',
    ok: docs.status === 200 && !!docs.body?.paths,
    detail: `status=${docs.status}, путей: ${Object.keys(docs.body?.paths || {}).length}`,
  });

  const home = await call(`${FRONTEND}/`);
  checks.push({
    name: 'Frontend: главная страница',
    ok: home.status === 200 && typeof home.body === 'string' && home.body.includes('ФСП'),
    detail: `status=${home.status}, ${home.ms} мс, ${typeof home.body === 'string' ? home.body.length : 0} байт`,
  });

  const login = await call(`${FRONTEND}/login`);
  checks.push({
    name: 'Frontend: /login',
    ok: login.status === 200,
    detail: `status=${login.status}, ${login.ms} мс`,
  });

  const proxy = await call(`${FRONTEND}/api/health`);
  checks.push({
    name: 'Прокси /api через frontend (rewrites)',
    ok: proxy.status === 200 && proxy.body?.status === 'ok',
    detail: `status=${proxy.status}, ${proxy.ms} мс`,
  });

  add('1. Smoke', checks);
}

// ============================================================================
// 2. SECURITY
// ============================================================================
async function securitySection(): Promise<void> {
  const checks: Check[] = [];
  const candidate = await register('CANDIDATE');
  const employer = await register('EMPLOYER');

  const noToken = await call(`${API}/candidates/profile`);
  checks.push({
    name: 'Доступ без токена → 401',
    ok: noToken.status === 401,
    detail: `status=${noToken.status}`,
  });

  const badToken = await call(`${API}/candidates/profile`, { token: 'invalid.token.value' });
  checks.push({
    name: 'Невалидный JWT → 401',
    ok: badToken.status === 401,
    detail: `status=${badToken.status}`,
  });

  const roleMismatch = await call(`${API}/candidates/profile`, { token: employer.token });
  checks.push({
    name: 'RBAC: работодатель не читает профиль кандидата → 403',
    ok: roleMismatch.status === 403,
    detail: `status=${roleMismatch.status}`,
  });

  const roleMismatch2 = await call(`${API}/employers/dashboard`, { token: candidate.token });
  checks.push({
    name: 'RBAC: кандидат не читает дашборд работодателя → 403',
    ok: roleMismatch2.status === 403,
    detail: `status=${roleMismatch2.status}`,
  });

  const sqli = await call(`${API}/auth/login`, {
    method: 'POST',
    body: { email: 'admin@example.com" OR 1=1--', password: 'test' },
  });
  checks.push({
    name: 'SQL-injection в login → 4xx (не 500)',
    ok: sqli.status >= 400 && sqli.status < 500,
    detail: `status=${sqli.status}`,
  });

  const weak = await call(`${API}/auth/register`, {
    method: 'POST',
    body: { email: `weak-${stamp()}@example.com`, password: '1', role: 'CANDIDATE' },
  });
  checks.push({
    name: 'Слабый пароль при регистрации → 400',
    ok: weak.status === 400,
    detail: `status=${weak.status}`,
  });

  const missingVacancy = await call(`${API}/vacancies/00000000-0000-0000-0000-000000000000`);
  checks.push({
    name: 'Несуществующая вакансия → 404',
    ok: missingVacancy.status === 404,
    detail: `status=${missingVacancy.status}`,
  });

  const foreign = await call(`${API}/invitations`, {
    method: 'POST',
    token: candidate.token,
    body: { candidateId: '00000000-0000-0000-0000-000000000000', message: 'x'.repeat(20), salaryFrom: 1, salaryTo: 2 },
  });
  checks.push({
    name: 'Кандидат не может создать приглашение → 403',
    ok: foreign.status === 403,
    detail: `status=${foreign.status}`,
  });

  const searchNoAuth = await call(`${API}/matching/search`, {
    method: 'POST',
    body: { description: 'тест тест тест тест тест' },
  });
  checks.push({
    name: 'Поиск кандидатов без токена → 401',
    ok: searchNoAuth.status === 401,
    detail: `status=${searchNoAuth.status}`,
  });

  add('2. Security', checks);
}

// ============================================================================
// 3. MATCHING QUALITY
// ============================================================================
interface Need {
  title: string;
  specialization: string;
  grade: string;
  stack: string[];
  description: string;
}

const NEEDS: Need[] = [
  {
    title: 'Backend: Node.js + PostgreSQL',
    specialization: 'BACKEND',
    grade: 'MIDDLE',
    stack: ['Node.js', 'PostgreSQL'],
    description: 'Ищем backend-разработчика на Node.js: REST API, PostgreSQL, транзакции, высокая нагрузка',
  },
  {
    title: 'Backend: распределённые системы',
    specialization: 'BACKEND',
    grade: 'SENIOR',
    stack: ['Redis', 'Docker', 'Kafka'],
    description: 'Senior backend: Redis, очереди Kafka, Docker, отказоустойчивость',
  },
  {
    title: 'Frontend: React + TypeScript',
    specialization: 'FRONTEND',
    grade: 'MIDDLE',
    stack: ['React', 'TypeScript'],
    description: 'Frontend-разработчик на React и TypeScript, дизайн-системы, производительность UI',
  },
  {
    title: 'DevOps: Kubernetes',
    specialization: 'DEVOPS',
    grade: 'MIDDLE',
    stack: ['Docker', 'Kubernetes'],
    description: 'Инженер DevOps: Kubernetes, Docker, CI/CD, инфраструктура как код',
  },
  {
    title: 'Mobile: Flutter',
    specialization: 'MOBILE',
    grade: 'MIDDLE',
    stack: ['Flutter', 'Android'],
    description: 'Mobile-разработчик: Flutter и Android, публикация в сторах',
  },
  {
    title: 'Data Science: ML-модели',
    specialization: 'DATA_SCIENCE',
    grade: 'MIDDLE',
    stack: ['Python', 'PyTorch'],
    description: 'Data Scientist: Python, обучение моделей, продакшн-инференс',
  },
];

function isRelevant(candidate: any, need: Need): boolean {
  if (need.specialization && candidate.specialization !== need.specialization) return false;
  const wanted = need.stack.map(norm);
  const have = new Set((candidate.stack || []).map(norm));
  const overlap = wanted.filter((s) => have.has(s)).length / wanted.length;
  return overlap >= 0.5;
}

function precisionAt(ranked: boolean[], k: number): number {
  const top = ranked.slice(0, k);
  if (!top.length) return 0;
  return top.filter(Boolean).length / top.length;
}

function recallAt(ranked: boolean[], totalRelevant: number, k: number): number {
  if (!totalRelevant) return 0;
  return ranked.slice(0, k).filter(Boolean).length / totalRelevant;
}

function mrr(ranked: boolean[], k: number): number {
  for (let i = 0; i < Math.min(k, ranked.length); i++) if (ranked[i]) return 1 / (i + 1);
  return 0;
}

function ndcgAt(ranked: boolean[], k: number): number {
  const top = ranked.slice(0, k);
  const dcg = top.reduce((sum, rel, i) => sum + (rel ? 1 / Math.log2(i + 2) : 0), 0);
  const ideal = top.filter(Boolean).length;
  let idcg = 0;
  for (let i = 0; i < ideal; i++) idcg += 1 / Math.log2(i + 2);
  return idcg ? dcg / idcg : 0;
}

async function matchingSection(): Promise<{ p5: number; p10: number; mrrAvg: number; ndcg: number; searchMs: number }> {
  const employer = await register('EMPLOYER');
  const checks: Check[] = [];
  const rows: string[] = [];

  let p5Sum = 0;
  let p10Sum = 0;
  let mrrSum = 0;
  let ndcgSum = 0;
  let searchMsSum = 0;

  for (const need of NEEDS) {
    const full = await call(`${API}/matching/search`, {
      method: 'POST',
      token: employer.token,
      body: { ...need, limit: 100 },
    });
    const fast = await call(`${API}/matching/search`, {
      method: 'POST',
      token: employer.token,
      body: { ...need, limit: 10 },
    });

    const list: any[] = full.body?.candidates || [];
    const ranked = list.map((c) => isRelevant(c, need));
    const relevant = ranked.filter(Boolean).length;

    const p5 = precisionAt(ranked, 5);
    const p10 = precisionAt(ranked, 10);
    const r10 = recallAt(ranked, relevant, 10);
    const m = mrr(ranked, 10);
    const n = ndcgAt(ranked, 10);

    p5Sum += p5;
    p10Sum += p10;
    mrrSum += m;
    ndcgSum += n;
    searchMsSum += fast.ms;

    const hardFiltered = list.every((c) => c.specialization === need.specialization);
    rows.push(
      `| ${need.title} | ${list.length} | ${(p5 * 100).toFixed(0)}% | ${(p10 * 100).toFixed(0)}% | ${(r10 * 100).toFixed(0)}% | ${m.toFixed(2)} | ${n.toFixed(3)} | ${fast.ms} мс | ${hardFiltered ? 'да' : 'НЕТ'} |`,
    );

    checks.push({
      name: `precision@5 · ${need.title}`,
      ok: p5 >= 0.7,
      detail: `${(p5 * 100).toFixed(0)}% (цель ≥ 70%), релевантных в базе: ${relevant}`,
    });
    checks.push({
      name: `hard-фильтр (только ${need.specialization}) · ${need.title}`,
      ok: hardFiltered,
      detail: `кандидатов в выдаче: ${list.length}`,
    });
  }

  const p5 = p5Sum / NEEDS.length;
  const p10 = p10Sum / NEEDS.length;
  const mrrAvg = mrrSum / NEEDS.length;
  const ndcg = ndcgSum / NEEDS.length;
  const searchMs = searchMsSum / NEEDS.length;

  const table = [
    '| Need | Канд. | P@5 | P@10 | R@10 | MRR | nDCG@10 | Поиск | Hard-фильтр |',
    '|------|------|-----|------|------|-----|---------|-------|-------------|',
    ...rows,
    `| **Среднее** | — | **${(p5 * 100).toFixed(0)}%** | **${(p10 * 100).toFixed(0)}%** | — | **${mrrAvg.toFixed(2)}** | **${ndcg.toFixed(3)}** | **${Math.round(searchMs)} мс** | — |`,
  ].join('\n');

  add('3. Matching quality', checks, table);
  return { p5, p10, mrrAvg, ndcg, searchMs };
}

// ============================================================================
// 4. TESTING QUALITY
// ============================================================================
interface TestOutcome {
  score: number;
  theta: number;
  gradeAfter: string;
  recommendation: string;
  suspicious: boolean;
  flags: string[];
}

async function runTest(mode: 'correct' | 'wrong'): Promise<TestOutcome> {
  const account = await register('CANDIDATE');
  const start = await call(`${API}/testing/start`, {
    method: 'POST',
    token: account.token,
    body: { specialization: 'BACKEND', declaredGrade: 'MIDDLE' },
  });
  if (start.status !== 201) throw new Error(`testing/start: ${start.status}`);
  const sessionId = start.body.sessionId as string;

  const session = await prisma.testSession.findUnique({ where: { id: sessionId } });
  if (!session) throw new Error('session not found');
  const pool = await prisma.testQuestion.findMany({ where: { categoryId: session.categoryId } });
  const byId = new Map(pool.map((q) => [q.id, q]));

  for (let i = 0; i < 25; i++) {
    const current = await call(`${API}/testing/session/${sessionId}/current`, { token: account.token });
    const question = current.body?.question;
    if (!question) break;

    const source = byId.get(question.id) as any;
    if (!source) throw new Error(`question ${question.id} not in pool`);

    const options: Array<{ id: string; isCorrect?: boolean }> = (source.options as any) || [];
    const body: Record<string, unknown> = {
      questionId: question.id,
      timeSpentSeconds: rnd(6, 55),
    };

    if (question.questionType === 'multiple_choice') {
      const correct = options.find((o) => o.isCorrect === true);
      const wrong = options.find((o) => o.isCorrect !== true);
      const chosen = mode === 'correct' ? correct || options[0] : wrong || options[options.length - 1];
      if (!chosen) throw new Error('no options');
      body.answer = chosen.id;
    } else {
      body.answerText =
        mode === 'correct'
          ? source.correctAnswer || 'ключевые слова эталона покрыты полностью: ' + (source.correctAnswer ?? '')
          : 'не знаю';
    }

    await call(`${API}/testing/session/${sessionId}/answer`, {
      method: 'POST',
      token: account.token,
      body,
    });
    await wait(rnd(120, 420));
  }

  const complete = await call(`${API}/testing/session/${sessionId}/complete`, {
    method: 'POST',
    token: account.token,
    body: {},
  });
  if (complete.status !== 201) throw new Error(`complete: ${complete.status} ${JSON.stringify(complete.body)}`);

  return {
    score: Number(complete.body.score),
    theta: Number(complete.body.theta),
    gradeAfter: complete.body.gradeAfter,
    recommendation: complete.body.recommendation,
    suspicious: !!complete.body.antiCheat?.isSuspicious,
    flags: complete.body.antiCheat?.flags || [],
  };
}

async function testingSection(): Promise<{ strong: TestOutcome; strong2: TestOutcome; weak: TestOutcome }> {
  const checks: Check[] = [];

  const strong = await runTest('correct');
  const strong2 = await runTest('correct');
  const weak = await runTest('wrong');

  checks.push({
    name: 'Дискриминативность: сильный кандидат получает высокий score',
    ok: strong.score >= 80 && strong2.score >= 80,
    detail: `score1=${strong.score}, score2=${strong2.score}, θ1=${strong.theta}, θ2=${strong2.theta}`,
  });
  checks.push({
    name: 'Дискриминативность: слабый кандидат получает низкий score',
    ok: weak.score <= 30,
    detail: `score=${weak.score}, θ=${weak.theta}`,
  });
  checks.push({
    name: 'Порядок оценок: сильный ≫ слабый (θ различие ≥ 2)',
    ok: strong.theta - weak.theta >= 2 && gradeIdx(strong.gradeAfter) > gradeIdx(weak.gradeAfter),
    detail: `θ: ${strong.theta} vs ${weak.theta}; грейд: ${strong.gradeAfter} vs ${weak.gradeAfter}`,
  });
  checks.push({
    name: 'Test-retest: повторный прогон даёт близкий результат',
    ok: Math.abs(strong.score - strong2.score) <= 10 && strong.gradeAfter === strong2.gradeAfter,
    detail: `|Δscore| = ${Math.abs(strong.score - strong2.score)}, грейды: ${strong.gradeAfter} / ${strong2.gradeAfter}`,
  });
  checks.push({
    name: 'Рекомендации соответствуют результату',
    ok: strong.recommendation !== 'FAIL' && weak.recommendation === 'FAIL',
    detail: `сильный: ${strong.recommendation}, слабый: ${weak.recommendation}`,
  });
  checks.push({
    name: 'Анти-чит не даёт ложных срабатываний на честном прогоне',
    ok: !strong.suspicious && !strong2.suspicious,
    detail: `flags1: [${strong.flags.join(', ')}] · flags2: [${strong2.flags.join(', ')}]`,
  });

  const table = [
    '| Прогон | Score | θ | Grade after | Recommendation | Suspicious |',
    '|--------|-------|---|-------------|----------------|------------|',
    `| Сильный (1) | ${strong.score} | ${strong.theta} | ${strong.gradeAfter} | ${strong.recommendation} | ${strong.suspicious} |`,
    `| Сильный (2) — test-retest | ${strong2.score} | ${strong2.theta} | ${strong2.gradeAfter} | ${strong2.recommendation} | ${strong2.suspicious} |`,
    `| Слабый | ${weak.score} | ${weak.theta} | ${weak.gradeAfter} | ${weak.recommendation} | ${weak.suspicious} |`,
  ].join('\n');

  add('4. Testing quality', checks, table);
  return { strong, strong2, weak };
}

// ============================================================================
// 5. PERFORMANCE
// ============================================================================
function percentile(values: number[], p: number): number {
  if (!values.length) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.ceil((p / 100) * sorted.length) - 1);
  return sorted[Math.max(0, index)];
}

async function performanceSection(): Promise<{ avgAll: number; searchAvg: number; frontendMs: number }> {
  const candidate = await register('CANDIDATE');
  const employer = await register('EMPLOYER');
  const checks: Check[] = [];
  const rows: string[] = [];

  const targets: Array<{
    title: string;
    run: () => Promise<HttpResult>;
    maxAvg: number;
    maxP95?: number;
  }> = [
    { title: 'GET /api/health', run: () => call(`${API}/health`), maxAvg: 200 },
    { title: 'GET /api/candidates/profile', run: () => call(`${API}/candidates/profile`, { token: candidate.token }), maxAvg: 200 },
    { title: 'GET /api/vacancies?limit=20', run: () => call(`${API}/vacancies?limit=20`), maxAvg: 200 },
    { title: 'GET /api/notifications', run: () => call(`${API}/notifications`, { token: candidate.token }), maxAvg: 200 },
    { title: 'GET /api/employers/dashboard', run: () => call(`${API}/employers/dashboard`, { token: employer.token }), maxAvg: 200 },
    {
      title: 'POST /api/matching/search (limit 10)',
      run: () =>
        call(`${API}/matching/search`, {
          method: 'POST',
          token: employer.token,
          body: { description: 'backend разработчик Node.js PostgreSQL Redis', specialization: 'BACKEND', grade: 'MIDDLE', stack: ['Node.js', 'PostgreSQL'], limit: 10 },
        }),
      maxAvg: 200,
      maxP95: 500,
    },
    {
      title: 'POST /api/matching/semantic (limit 10)',
      run: () =>
        call(`${API}/matching/semantic`, {
          method: 'POST',
          token: employer.token,
          body: { query: 'разработчик с опытом Redis и очередей', limit: 10 },
        }),
      maxAvg: 200,
      maxP95: 500,
    },
  ];

  const ITERATIONS = 30;
  const allAverages: number[] = [];
  let searchAvg = 0;

  for (const target of targets) {
    const times: number[] = [];
    let errors = 0;
    await target.run(); // прогрев
    for (let i = 0; i < ITERATIONS; i++) {
      const res = await target.run();
      times.push(res.ms);
      if (res.status >= 500) errors++;
    }
    const avg = times.reduce((a, b) => a + b, 0) / times.length;
    const p50 = percentile(times, 50);
    const p95 = percentile(times, 95);
    const max = Math.max(...times);
    allAverages.push(avg);
    if (target.title.includes('search')) searchAvg = avg;

    const ok = avg < target.maxAvg && errors === 0 && (target.maxP95 ? p95 < target.maxP95 : true);
    checks.push({
      name: target.title,
      ok,
      detail: `avg=${avg.toFixed(0)} мс, p50=${p50} мс, p95=${p95} мс, max=${max} мс, 5xx=${errors} (цель: avg < ${target.maxAvg} мс${target.maxP95 ? `, p95 < ${target.maxP95} мс` : ''})`,
    });
    rows.push(
      `| ${target.title} | ${avg.toFixed(0)} | ${p50} | ${p95} | ${max} | ${errors} |`,
    );
  }

  const startedFrontend = Date.now();
  const home = await call(`${FRONTEND}/`);
  const frontendMs = Date.now() - startedFrontend;
  checks.push({
    name: 'Frontend: полная загрузка документа < 3000 мс',
    ok: home.status === 200 && frontendMs < 3000,
    detail: `status=${home.status}, ${frontendMs} мс (SSR/TTFB; гидратация измеряется в браузере)`,
  });
  rows.push(`| GET ${FRONTEND}/ (SSR документ) | ${frontendMs} | — | — | ${frontendMs} | 0 |`);

  const avgAll = allAverages.reduce((a, b) => a + b, 0) / allAverages.length;
  checks.push({
    name: 'Среднее время ответа API < 200 мс',
    ok: avgAll < 200,
    detail: `avg по всем endpoint'ам = ${avgAll.toFixed(0)} мс`,
  });

  const table = [
    '| Endpoint | avg, мс | p50 | p95 | max | 5xx |',
    '|----------|---------|-----|-----|-----|-----|',
    ...rows,
  ].join('\n');

  add('5. Performance', checks, table);
  return { avgAll, searchAvg, frontendMs };
}

// ============================================================================
// ОТЧЁТ
// ============================================================================
function writeReport(results: {
  matching: { p5: number; p10: number; mrrAvg: number; ndcg: number; searchMs: number };
  testing: { strong: TestOutcome; strong2: TestOutcome; weak: TestOutcome };
  performance: { avgAll: number; searchAvg: number; frontendMs: number };
}): void {
  const total = sections.reduce((sum, s) => sum + s.checks.length, 0);
  const passed = sections.reduce((sum, s) => sum + s.checks.filter((c) => c.ok).length, 0);

  const lines: string[] = [];
  lines.push('# Отчёт о валидации решения «ФСП · платформа ИТ-вакансий»');
  lines.push('');
  lines.push(`Дата прогона: ${RUN_AT.toLocaleString('ru-RU')}`);
  lines.push('');
  lines.push(`**Итог: ${passed} / ${total} проверок пройдено**`);
  lines.push('');
  lines.push('## Сводка по целям (docs/VALIDATION.md)');
  lines.push('');
  lines.push('| Метрика | Цель | Факт | Статус |');
  lines.push('|---------|------|------|--------|');
  lines.push(`| Precision@5 | ≥ 70% | ${(results.matching.p5 * 100).toFixed(0)}% | ${results.matching.p5 >= 0.7 ? 'PASS' : 'FAIL'} |`);
  lines.push(`| Precision@10 | — | ${(results.matching.p10 * 100).toFixed(0)}% | — |`);
  lines.push(`| MRR | — | ${results.matching.mrrAvg.toFixed(2)} | — |`);
  lines.push(`| nDCG@10 | — | ${results.matching.ndcg.toFixed(3)} | — |`);
  lines.push(`| Поиск (avg) | < 500 мс | ${Math.round(results.matching.searchMs)} мс | ${results.matching.searchMs < 500 ? 'PASS' : 'FAIL'} |`);
  lines.push(`| API (avg) | < 200 мс | ${Math.round(results.performance.avgAll)} мс | ${results.performance.avgAll < 200 ? 'PASS' : 'FAIL'} |`);
  lines.push(`| Frontend (документ) | < 3000 мс | ${Math.round(results.performance.frontendMs)} мс | ${results.performance.frontendMs < 3000 ? 'PASS' : 'FAIL'} |`);
  lines.push(`| Test-retest (Δscore) | ≤ 10 | ${Math.abs(results.testing.strong.score - results.testing.strong2.score)} | ${Math.abs(results.testing.strong.score - results.testing.strong2.score) <= 10 ? 'PASS' : 'FAIL'} |`);
  lines.push(`| Дискриминативность (θ) | ≥ 2 | ${(results.testing.strong.theta - results.testing.weak.theta).toFixed(2)} | ${results.testing.strong.theta - results.testing.weak.theta >= 2 ? 'PASS' : 'FAIL'} |`);
  lines.push('');

  for (const section of sections) {
    lines.push(`## ${section.title}`);
    lines.push('');
    lines.push('| Проверка | Результат | Статус |');
    lines.push('|----------|-----------|--------|');
    for (const check of section.checks) {
      lines.push(`| ${check.name} | ${check.detail} | ${check.ok ? '✅ PASS' : '❌ FAIL'} |`);
    }
    lines.push('');
    if (section.extra) {
      lines.push(section.extra);
      lines.push('');
    }
  }

  lines.push('---');
  lines.push('');
  lines.push('Как воспроизвести:');
  lines.push('');
  lines.push('```bash');
  lines.push('docker compose up -d --build                  # полный стек');
  lines.push('cd backend && npm run validate                # этот отчёт');
  lines.push('```');
  lines.push('');

  fs.mkdirSync(path.dirname(REPORT), { recursive: true });
  fs.writeFileSync(REPORT, lines.join('\n'), 'utf8');

  // eslint-disable-next-line no-console
  console.log(`\nОтчёт: ${REPORT}`);
  // eslint-disable-next-line no-console
  console.log(`Проверок: ${passed}/${total} PASS`);
}

async function main() {
  await smokeSection();
  await securitySection();
  const matching = await matchingSection();
  const testing = await testingSection();
  const performance = await performanceSection();
  writeReport({ matching, testing, performance });

  const failed = sections.flatMap((s) => s.checks.filter((c) => !c.ok));
  if (failed.length) {
    // eslint-disable-next-line no-console
    console.log('\nПроваленные проверки:');
    for (const f of failed) console.log(` - ${f.name}: ${f.detail}`);
  }
  await prisma.$disconnect();
  process.exit(failed.length ? 1 : 0);
}

main().catch(async (error) => {
  // eslint-disable-next-line no-console
  console.error('FATAL:', error);
  await prisma.$disconnect();
  process.exit(2);
});
