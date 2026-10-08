# Отчёт о валидации решения «ФСП · платформа ИТ-вакансий»

Дата прогона: 05.10.2026, 21:16:18

**Итог: 41 / 41 проверок пройдено**

## Сводка по целям (docs/VALIDATION.md)

| Метрика | Цель | Факт | Статус |
|---------|------|------|--------|
| Precision@5 | ≥ 70% | 100% | PASS |
| Precision@10 | — | 90% | — |
| MRR | — | 1.00 | — |
| nDCG@10 | — | 1.000 | — |
| Поиск (avg) | < 500 мс | 19 мс | PASS |
| API (avg) | < 200 мс | 7 мс | PASS |
| Frontend (документ) | < 3000 мс | 9 мс | PASS |
| Test-retest (Δscore) | ≤ 10 | 0 | PASS |
| Дискриминативность (θ) | ≥ 2 | 8.00 | PASS |

## 1. Smoke

| Проверка | Результат | Статус |
|----------|-----------|--------|
| GET /api/health | status=200, db=up, dim=384 | ✅ PASS |
| Swagger JSON публикуется | status=200, путей: 41 | ✅ PASS |
| Frontend: главная страница | status=200, 13 мс, 12351 байт | ✅ PASS |
| Frontend: /login | status=200, 6 мс | ✅ PASS |
| Прокси /api через frontend (rewrites) | status=200, 12 мс | ✅ PASS |

## 2. Security

| Проверка | Результат | Статус |
|----------|-----------|--------|
| Доступ без токена → 401 | status=401 | ✅ PASS |
| Невалидный JWT → 401 | status=401 | ✅ PASS |
| RBAC: работодатель не читает профиль кандидата → 403 | status=403 | ✅ PASS |
| RBAC: кандидат не читает дашборд работодателя → 403 | status=403 | ✅ PASS |
| SQL-injection в login → 4xx (не 500) | status=400 | ✅ PASS |
| Слабый пароль при регистрации → 400 | status=400 | ✅ PASS |
| Несуществующая вакансия → 404 | status=404 | ✅ PASS |
| Кандидат не может создать приглашение → 403 | status=403 | ✅ PASS |
| Поиск кандидатов без токена → 401 | status=401 | ✅ PASS |

## 3. Matching quality

| Проверка | Результат | Статус |
|----------|-----------|--------|
| precision@5 · Backend: Node.js + PostgreSQL | 100% (цель ≥ 70%), релевантных в базе: 9 | ✅ PASS |
| hard-фильтр (только BACKEND) · Backend: Node.js + PostgreSQL | кандидатов в выдаче: 13 | ✅ PASS |
| precision@5 · Backend: распределённые системы | 100% (цель ≥ 70%), релевантных в базе: 5 | ✅ PASS |
| hard-фильтр (только BACKEND) · Backend: распределённые системы | кандидатов в выдаче: 6 | ✅ PASS |
| precision@5 · Frontend: React + TypeScript | 100% (цель ≥ 70%), релевантных в базе: 8 | ✅ PASS |
| hard-фильтр (только FRONTEND) · Frontend: React + TypeScript | кандидатов в выдаче: 9 | ✅ PASS |
| precision@5 · DevOps: Kubernetes | 100% (цель ≥ 70%), релевантных в базе: 10 | ✅ PASS |
| hard-фильтр (только DEVOPS) · DevOps: Kubernetes | кандидатов в выдаче: 10 | ✅ PASS |
| precision@5 · Mobile: Flutter | 100% (цель ≥ 70%), релевантных в базе: 10 | ✅ PASS |
| hard-фильтр (только MOBILE) · Mobile: Flutter | кандидатов в выдаче: 11 | ✅ PASS |
| precision@5 · Data Science: ML-модели | 100% (цель ≥ 70%), релевантных в базе: 7 | ✅ PASS |
| hard-фильтр (только DATA_SCIENCE) · Data Science: ML-модели | кандидатов в выдаче: 9 | ✅ PASS |

| Need | Канд. | P@5 | P@10 | R@10 | MRR | nDCG@10 | Поиск | Hard-фильтр |
|------|------|-----|------|------|-----|---------|-------|-------------|
| Backend: Node.js + PostgreSQL | 13 | 100% | 90% | 100% | 1.00 | 0.997 | 16 мс | да |
| Backend: распределённые системы | 6 | 100% | 83% | 100% | 1.00 | 1.000 | 19 мс | да |
| Frontend: React + TypeScript | 9 | 100% | 89% | 100% | 1.00 | 1.000 | 20 мс | да |
| DevOps: Kubernetes | 10 | 100% | 100% | 100% | 1.00 | 1.000 | 18 мс | да |
| Mobile: Flutter | 11 | 100% | 100% | 100% | 1.00 | 1.000 | 21 мс | да |
| Data Science: ML-модели | 9 | 100% | 78% | 100% | 1.00 | 1.000 | 21 мс | да |
| **Среднее** | — | **100%** | **90%** | — | **1.00** | **1.000** | **19 мс** | — |

## 4. Testing quality

| Проверка | Результат | Статус |
|----------|-----------|--------|
| Дискриминативность: сильный кандидат получает высокий score | score1=100, score2=100, θ1=4, θ2=4 | ✅ PASS |
| Дискриминативность: слабый кандидат получает низкий score | score=0.3, θ=-4 | ✅ PASS |
| Порядок оценок: сильный ≫ слабый (θ различие ≥ 2) | θ: 4 vs -4; грейд: PRINCIPAL vs JUNIOR | ✅ PASS |
| Test-retest: повторный прогон даёт близкий результат | |Δscore| = 0, грейды: PRINCIPAL / PRINCIPAL | ✅ PASS |
| Рекомендации соответствуют результату | сильный: UPGRADE, слабый: FAIL | ✅ PASS |
| Анти-чит не даёт ложных срабатываний на честном прогоне | flags1: [] · flags2: [] | ✅ PASS |

| Прогон | Score | θ | Grade after | Recommendation | Suspicious |
|--------|-------|---|-------------|----------------|------------|
| Сильный (1) | 100 | 4 | PRINCIPAL | UPGRADE | false |
| Сильный (2) — test-retest | 100 | 4 | PRINCIPAL | UPGRADE | false |
| Слабый | 0.3 | -4 | JUNIOR | FAIL | false |

## 5. Performance

| Проверка | Результат | Статус |
|----------|-----------|--------|
| GET /api/health | avg=2 мс, p50=2 мс, p95=3 мс, max=4 мс, 5xx=0 (цель: avg < 200 мс) | ✅ PASS |
| GET /api/candidates/profile | avg=6 мс, p50=6 мс, p95=8 мс, max=8 мс, 5xx=0 (цель: avg < 200 мс) | ✅ PASS |
| GET /api/vacancies?limit=20 | avg=6 мс, p50=5 мс, p95=10 мс, max=12 мс, 5xx=0 (цель: avg < 200 мс) | ✅ PASS |
| GET /api/notifications | avg=8 мс, p50=7 мс, p95=12 мс, max=18 мс, 5xx=0 (цель: avg < 200 мс) | ✅ PASS |
| GET /api/employers/dashboard | avg=6 мс, p50=6 мс, p95=7 мс, max=8 мс, 5xx=0 (цель: avg < 200 мс) | ✅ PASS |
| POST /api/matching/search (limit 10) | avg=6 мс, p50=6 мс, p95=7 мс, max=7 мс, 5xx=0 (цель: avg < 200 мс, p95 < 500 мс) | ✅ PASS |
| POST /api/matching/semantic (limit 10) | avg=14 мс, p50=14 мс, p95=15 мс, max=17 мс, 5xx=0 (цель: avg < 200 мс, p95 < 500 мс) | ✅ PASS |
| Frontend: полная загрузка документа < 3000 мс | status=200, 9 мс (SSR/TTFB; гидратация измеряется в браузере) | ✅ PASS |
| Среднее время ответа API < 200 мс | avg по всем endpoint'ам = 7 мс | ✅ PASS |

| Endpoint | avg, мс | p50 | p95 | max | 5xx |
|----------|---------|-----|-----|-----|-----|
| GET /api/health | 2 | 2 | 3 | 4 | 0 |
| GET /api/candidates/profile | 6 | 6 | 8 | 8 | 0 |
| GET /api/vacancies?limit=20 | 6 | 5 | 10 | 12 | 0 |
| GET /api/notifications | 8 | 7 | 12 | 18 | 0 |
| GET /api/employers/dashboard | 6 | 6 | 7 | 8 | 0 |
| POST /api/matching/search (limit 10) | 6 | 6 | 7 | 7 | 0 |
| POST /api/matching/semantic (limit 10) | 14 | 14 | 15 | 17 | 0 |
| GET http://localhost:8080/ (SSR документ) | 9 | — | — | 9 | 0 |

---

Как воспроизвести:

```bash
docker compose up -d --build                  # полный стек
cd backend && npm run validate                # этот отчёт
```
