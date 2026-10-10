# 03. Аутентификация, авторизация и безопасность

Содержание: [README](README.md) · [01](01-architecture.md) · [02](02-data-model.md) · **03** · [04](04-api-reference.md) · [05](05-matching-engine.md) · [06](06-testing-engine.md) · [07](07-fsp-gamification-notifications.md) · [08](08-infrastructure.md) · [09](09-known-issues.md)

Исходники: `src/modules/auth/*`, `src/common/guards/jwt.guard.ts`, `src/common/decorators/auth.decorator.ts`, `src/common/utils/cookie.util.ts`, `src/common/filters/http-exception.filter.ts`.

---

## 1. Модель безопасности (обзор)

1. **Всё закрыто по умолчанию.** Глобальный `JwtAuthGuard` проверяет JWT на каждом маршруте; открыть маршрут можно только декоратором `@Public()`.
2. **Две роли в работе**: `CANDIDATE` и `EMPLOYER` (плюс `ADMIN` в enum — без собственных маршрутов). Проверка — глобальный `RolesGuard` по метаданным `@Roles(...)`.
3. **Пара токенов**: короткоживущий access (15 мин, stateless) + refresh (7 дней, ротация с хранением bcrypt-хеша в БД).
4. **Контакты кандидата — конфиденциальны** и раскрываются работодателю только после принятия приглашения (см. §7).

## 2. Регистрация — `POST /api/auth/register` (`@Public()`)

DTO (`src/modules/auth/dto/auth.dto.ts`):
| Поле | Валидация |
|------|-----------|
| `email` | `@IsEmail` |
| `password` | минимум 8 символов + regex `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)` («Пароль должен содержать заглавную букву, строчную букву и цифру») |
| `role` | enum `UserRole`, **и** `@IsIn([CANDIDATE, EMPLOYER])` — зарегистрировать ADMIN нельзя |

Логика (`auth.service.ts → register`):
1. Проверка занятости email → `409 ConflictException('Пользователь с таким email уже существует')`.
2. `bcrypt.hash(password, 12)` (12 раундов).
3. Транзакция Prisma: создание `User` (+ `emailVerifyToken: crypto.randomUUID()`) и связанного профиля — кандидату `CandidateProfile`, работодателю `EmployerProfile { companyName: '' }`.
4. Выпуск пары токенов, сохранение bcrypt-хеша refresh-токена в `users.refreshToken`.
5. Ответ: `{ accessToken, refreshToken, user: SafeUser }`, где `SafeUser = {id, email, role, isEmailVerified, createdAt}`. Плюс cookie `refreshToken`.

⚠️ Письмо с подтверждением не отправляется (SMTP-заглушка), эндпоинта верификации нет — `isEmailVerified` остаётся `false`.

## 3. Вход — `POST /api/auth/login` (`@Public()`)

DTO: `email`, `password` (`@IsString`). Любое несовпадение (нет пользователя / неверный пароль) → `401 UnauthorizedException('Неверный email или пароль')` — без утечки информации о существовании аккаунта. Успех — как при регистрации (шаги 4–5).

## 4. Токены и cookie

| Параметр | Access | Refresh |
|----------|--------|---------|
| TTL | **15 минут** (`jwt.accessTokenTtl`) | **7 дней** (`jwt.refreshTokenTtlDays`) |
| Секрет | `JWT_SECRET` | `JWT_REFRESH_SECRET` |
| Payload | `{sub, email, role}` | `{sub, email, role, typ: 'refresh'}` |
| Передача | header `Authorization: Bearer <token>` | cookie `refreshToken` **или** body `refreshToken` |
| Хранение на сервере | нет (stateless) | **bcrypt-хеш** в `users.refreshToken` |

Cookie (`auth.controller.ts`):
```
refreshToken=<jwt>; Path=/; HttpOnly; SameSite=Lax; Max-Age=604800; Secure   (secure только при NODE_ENV=production)
```
`src/common/utils/cookie.util.ts` — минимальный парсер cookie (`parseCookies`) для `POST /api/auth/refresh` без cookie-parser, и `buildCookie` (хелпер).

## 5. Обновление — `POST /api/auth/refresh` (`@Public()`)

Принимает токен из `body.refreshToken` или cookie `refreshToken`. Шаги:
1. `jwt.verify(token, refreshSecret)` → ошибка ⇒ `401 Refresh token недействителен`.
2. Проверка `payload.typ === 'refresh'` — access-токен отвергается.
3. Пользователь должен существовать и иметь сохранённый `refreshToken` в БД.
4. `bcrypt.compare(token, user.refreshToken)` — сверка с хешем.
5. Выпуск новой пары + **перезапись хеша** = **ротация**: старый refresh перестаёт работать.

⚠️ Нет blacklist и нет детекции повторного использования: replay старого токена просто получает 401. Access-токены остаются валидными до истечения 15 минут независимо от logout.

## 6. Выход и текущий пользователь

- `POST /api/auth/logout` (JWT) — `user.update({ refreshToken: null })`, `res.clearCookie('refreshToken', { path: '/' })`, ответ `{ message: 'Выход выполнен' }`.
- `GET /api/auth/me` (JWT) — пользователь + `candidateProfile` + `employerProfile`, **без** `passwordHash`, `refreshToken`, `emailVerifyToken`; `401` если пользователь удалён.

## 7. JWT-strategy (`src/modules/auth/strategies/jwt.strategy.ts`)

- Извлечение: `ExtractJwt.fromAuthHeaderAsBearerToken()`, `ignoreExpiration: false`, секрет из `config('jwt.secret')`.
- `validate(payload)`: `prisma.user.findUnique({ id: payload.sub })` на **каждый запрос**; если пользователя нет → `401 UnauthorizedException('Пользователь не найден')`. В `request.user` попадает `{id, email, role}`.

Следствие: удаление/блокировка пользователя действует немедленно (не дожидаясь истечения access-токена).

## 8. Guard-ы и RBAC (`src/common/guards/jwt.guard.ts`)

**`JwtAuthGuard`** (глобальный, extends `AuthGuard('jwt')`):
- `canActivate`: читает метаданные `IS_PUBLIC_KEY` (`@Public()`) на методе/классе — public-маршруты пропускаются без токена;
- `handleRequest`: при ошибке/отсутствии пользователя → `401 UnauthorizedException('Требуется авторизация')`.

**`RolesGuard`** (глобальный):
- читает `ROLES_KEY` (`@Roles(...)`) на методе/классе; метаданных нет → доступ разрешён (достаточно JWT);
- пользователя нет → `401`; роль не входит в список → `403 ForbiddenException('Недостаточно прав для этого действия')`.

⚠️ `ADMIN` **не** привилегирован автоматически: он проходит только те маршруты, где `@Roles` включает ADMIN явно.

### Сводка по ролям маршрутов

| Раздел API | Доступ |
|-----------|--------|
| `POST /api/auth/register`, `/login`, `/refresh`, `GET /api/health`, `GET /api/vacancies`, `GET /api/vacancies/:id` | `@Public()` |
| `GET/PUT /api/candidates/*`, `/api/testing/*`, `/api/fsp/connect|disconnect|achievements`, `/api/gamification/profile|badges|xp-history` | только `CANDIDATE` |
| `/api/employers/*`, `/api/matching/*`, создание/изменение вакансий, создание/отзыв приглашений | только `EMPLOYER` |
| `GET /api/invitations`, `GET /api/invitations/:id`, `/api/notifications`, `/api/gamification/leaderboard`, `/api/fsp/catalog` | любой авторизованный (данные скоупятся по роли) |

## 9. Валидация и формат ошибок

- Глобальный `ValidationPipe({whitelist, transform, enableImplicitConversion})`: неизвестные поля DTO отбрасываются, query-параметры неявно конвертируются в типы (например, `limit: string → number`).
- `HttpExceptionFilter` — единый JSON `{statusCode, timestamp, path, message}`; сообщения валидации склеиваются через `', '`.
- Типичные ответы: `400` (валидация, бизнес-ограничения), `401` (`Требуется авторизация` / неверные учётные данные), `403` (`Недостаточно прав...` / `Это не ваша вакансия`), `404`, `409` (конфликты состояния), `503` (MinIO недоступен).

## 10. Приватность данных кандидата (важно для сервиса)

Публичное представление кандидата (`src/modules/matching/candidate-view.ts` → `toCandidatePublic`):
- `displayName`: при `showFullName === false` → `Кандидат #<id4>`, иначе «Иван И.» (инициал фамилии);
- email/телефон/ФИО не входят в публичное представление вообще.

Раскрытие контактов работодателю происходит в двух местах и **только при `Invitation.status === ACCEPTED`**:
- `GET /api/employers/candidates/:id` → `{ contacts: {fullName, email, phone, city} | null, contactsLocked, contactsHint }`;
- `GET /api/invitations/:id` (для роли EMPLOYER) → `contacts: {email, phone} | null`.

⚠️ Флаги `showEmail` / `showPhone` при этом **не проверяются** — принятие приглашения открывает полный набор контактов независимо от настроек приватности (см. [09-known-issues.md](09-known-issues.md)).

## 11. Защита от распространённых атак (проверяется автовалидацией)

`scripts/metrics.ts` (раздел Security) и e2e-тесты подтверждают:
- запросы без токена и с невалидным JWT → `401`;
- кросс-рольные обращения (кандидат → `/api/employers/dashboard`, работодатель → `/api/candidates/profile`) → `403`;
- SQL-инъекция в логин → `4xx`, а не `500` (Prisma параметризует запросы);
- слабый пароль → `400`; кандидат не может создать приглашение → `403`;
- `POST /api/matching/search` без токена → `401`; карточка вопроса теста не утеает `correctAnswer`/`isCorrect`.

## 12. Ограничения текущей реализации

- Нет rate-limiting и защиты от brute-force (Redis не используется для локов/счётчиков).
- Нет верификации e-mail и восстановления пароля.
- Refresh-токен хранится один на пользователя (одна активная сессия); нет устройства/списка сессий.
- Нет отозванных access-токенов (blacklist) — logout действует только на refresh.
- CORS ограничен явным списком origin (включая localhost для разработки).
