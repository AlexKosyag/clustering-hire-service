# Модуль: Auth (Аутентификация и авторизация)

## 1. Назначение

Модуль отвечает за:
- Регистрацию пользователей (кандидаты и работодатели)
- Аутентификацию по email + пароль
- JWT-based авторизация (access + refresh tokens)
- RBAC (Role-Based Access Control)
- Верификацию email (токен генерируется, но письмо не отправляется — BUG-REF: S3)

**Зависимости**: database
**Блокирует**: backend-api, testing-engine, frontend

## 2. Архитектура модуля

```
backend/internal/modules/auth/
├── service.go          # Бизнес-логика: register, login, refresh, logout, me
│                       #   + NewAuthGuard (поведение JwtStrategy + JwtAuthGuard)
├── handler.go          # Маршруты /auth + cookie refresh-токена
└── dto.go              # RegisterDto/LoginDto, валидация (сообщения class-validator)

backend/internal/common/
├── jwt.go              # IssueAccessToken/IssueRefreshToken/ParseToken
├── router.go           # RouteOpts (Public/Roles) — guard-логика маршрутов
└── errors.go           # HTTPError, русские тексты ошибок
```

## 3. API Endpoints

### 3.1. Регистрация

```http
POST /api/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "role": "CANDIDATE" | "EMPLOYER"
}

Response 201:
{
  "id": "uuid",
  "email": "user@example.com",
  "role": "CANDIDATE",
  "isEmailVerified": false,
  "accessToken": "eyJhbG...",
  "refreshToken": "eyJhbG..."
}

Errors:
- 400: Validation error (invalid email, weak password)
- 409: Email already exists
```

### 3.2. Вход

```http
POST /api/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "SecurePass123!"
}

Response 200:
{
  "accessToken": "eyJhbG...",
  "refreshToken": "eyJhbG...",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "role": "CANDIDATE"
  }
}

Errors:
- 401: Invalid credentials
- 403: Email not verified
```

### 3.3. Обновление токена

```http
POST /api/auth/refresh
Cookie: refreshToken=eyJhbG...
# или тело: { "refreshToken": "eyJhbG..." }

Response 200:
{
  "accessToken": "eyJhbG...",
  "refreshToken": "eyJhbG..."
}

Errors:
- 401: Invalid refresh token
```

### 3.4. Выход

```http
POST /api/auth/logout
Authorization: Bearer {accessToken}

Response 200:
{
  "message": "Выход выполнен"
}
```

### 3.5. Верификация email

**Не реализовано.** Эндпоинта `GET /api/auth/verify-email` нет: токен
`email_verify_token` генерируется при регистрации, но письмо не отправляется и
подтверждение не выполняется (`isEmailVerified` остаётся `false`).
Смотрите BUG-REF: S3 в temp-docs/09-known-issues.md.

### 3.6. Текущий пользователь

```http
GET /api/auth/me
Authorization: Bearer {accessToken}

Response 200:
{
  "id": "uuid",
  "email": "user@example.com",
  "role": "CANDIDATE",
  "isEmailVerified": true,
  "profile": { ... }
}
```

## 4. Реализация

### 4.1. Auth Service

```go
// internal/modules/auth/service.go (аналог auth.service.ts)

// register — POST /auth/register:
//  1. SELECT EXISTS(email) → 409 «Пользователь с таким email уже существует»
//     (в гонке двух регистраций 409 даёт и нарушение уникальности 23505)
//  2. bcrypt hash пароля (cost 12)
//  3. Транзакция: INSERT users (+ email_verify_token) и профиль по роли —
//     candidate_profiles / employer_profiles (как prisma.$transaction)
//  4. Выпуск пары токенов + bcrypt-хеш refresh-токена → users.refresh_token
//
// BUG-REF: S3 — письмо для верификации не отправляется.

// login: нет пользователя и неверный пароль → одинаковый 401
// «Неверный email или пароль» (без утечки существования аккаунта).
func (s *Service) login(ctx context.Context, dto *LoginDto) (*AuthResponse, error)

// refresh — ротация пары токенов:
//  jwt.verify(refreshSecret) → проверка typ === "refresh" → пользователь существует
//  → bcrypt.compare с сохранённым хешом → новая пара.
// BUG-REF: S4 — один bcrypt-хеш на пользователя: списка сессий/устройств нет,
// детекции повторного использования токена нет.
func (s *Service) refresh(ctx context.Context, refreshToken string) (*TokensResponse, error)

// logout — users.refresh_token = NULL.
// BUG-REF: S4 — access-токены не отзываются и живут до истечения 15 минут.
func (s *Service) logout(ctx context.Context, userID string) error

// generateTokens: access — JWT_SECRET, 15 минут; refresh — JWT_REFRESH_SECRET, 7 дней.
```

### 4.2. Guard (JwtStrategy + JwtAuthGuard)

```go
// internal/modules/auth/service.go — NewAuthGuard:
// Bearer из Authorization → common.ParseToken → SELECT пользователя по payload.sub
// → common.SetUser(c, user).
// Нет токена / невалидный / истёк → 401 «Требуется авторизация»;
// пользователя нет в БД → 401 «Пользователь не найден».
// Пользователь подгружается из БД на каждый запрос (как validate() в jwt.strategy.ts).
func NewAuthGuard(db *store.DB, jwtSecret string) func(c *gin.Context) error
```

### 4.3. Guard-маршруты (RolesGuard + @Roles/@CurrentUser)

```go
// internal/modules/auth/handler.go
func (h *Handler) Register(r *common.Router) {
    r.POST("/auth/register", common.RouteOpts{Public: true}, h.register)
    r.POST("/auth/login",    common.RouteOpts{Public: true}, h.login)
    r.POST("/auth/refresh",  common.RouteOpts{Public: true}, h.refresh)
    r.POST("/auth/logout",   common.RouteOpts{},            h.logout) // JWT
    r.GET("/auth/me",        common.RouteOpts{},            h.me)     // JWT
}

// Ограничение по роли (аналог @Roles('EMPLOYER')):
//   common.RouteOpts{Roles: []model.UserRole{model.RoleEmployer}}
// Текущий пользователь в хендлере (аналог @CurrentUser()):
//   u := common.MustUser(c)
```

### 4.4. Refresh token: cookie + тело (вместо RefreshTokenGuard)

```go
// handler.go — токен берётся из body.refreshToken || cookie refreshToken
// (воспроизведено JS-правило «truthy или»: jsTruthyString).
// Cookie: HttpOnly, SameSite=Lax, Path=/, Max-Age=604800,
// Secure при NODE_ENV=production; logout очищает cookie.
```

## 5. DTO (Data Transfer Objects)

```go
// internal/modules/auth/dto.go
// Приведение типов и порядок сообщений повторяют class-validator:
// email → password → role; для password — снизу вверх @Matches → @MinLength(8) → @IsString.

type RegisterDto struct {
    Email    jsValue // @IsEmail
    Password jsValue // @Matches(заглавная+строчная+цифра), @MinLength(8), @IsString
    Role     jsValue // @IsIn([CANDIDATE, EMPLOYER]) + @IsEnum — design:type Object, без приведения
}

type LoginDto struct {
    Email    jsValue // @IsEmail
    Password jsValue // @IsString
}

// Сообщения:
//  - «Пароль должен содержать заглавную букву, строчную букву и цифру»
//  - «password must be longer than or equal to 8 characters»
//  - «email must be an email» и т.п. (см. internal/common/validate.go)
```

## 6. Интеграция с Keycloak (подготовка)

**Не реализовано, вне объёма.** Аутентификация — только локальный JWT
(login/refresh/logout). OIDC/Keycloak-провайдер в коде отсутствует.

## 7. Переменные окружения

```env
# JWT
JWT_SECRET=your-super-secret-jwt-key-min-32-chars
JWT_REFRESH_SECRET=your-super-secret-refresh-key-min-32-chars

# Прочее, влияющее на auth-сценарии
NODE_ENV=development           # в production cookie выставляется с Secure
```

SMTP и Keycloak-переменные в конфигурации не читаются (верификация e-mail и
OIDC не реализованы).

## 8. Чеклист валидации

- [ ] Регистрация создаёт User + Profile в транзакции
- [ ] Дубликат email отклоняется с 409
- [ ] Пароль хешируется через bcrypt (12 раундов)
- [ ] Login возвращает access + refresh токены
- [ ] Access token истекает через 15 минут
- [ ] Refresh token истекает через 7 дней
- [ ] Refresh token выставляется в HttpOnly cookie и возвращается в теле
- [ ] Обновление токена работает корректно (ротация)
- [ ] Logout инвалидирует refresh token и очищает cookie
- [ ] Guard блокирует доступ неавторизованным (401), RouteOpts.Roles — по ролям (403)
- [ ] common.MustUser возвращает пользователя в хендлере
- [ ] Валидация DTO работает (сообщения class-validator)
- [ ] Ошибки возвращают корректные HTTP коды

## 9. Промпт для агента

```
Ты — Backend-разработчик, специализирующийся на Go и аутентификации.

ТВОЯ ЗАДАЧА: Реализовать модуль auth для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/auth.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных (модуль User, UserRole)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/internal/modules/auth/ (service.go, handler.go, dto.go)
2. Реализовать регистрацию, логин, refresh, logout
3. Реализовать guard (NewAuthGuard) + RouteOpts (Public/Roles)
4. Реализовать доступ к текущему пользователю (MustUser)
5. Создать DTO с валидацией (сообщения class-validator)
6. Проверить сценарии по чеклисту (§8)

СТЕК:
- Go 1.25 + gin
- golang-jwt/v5
- bcrypt (x/crypto)
- pgx/v5 (сырой SQL)

ВАЛИДАЦИЯ:
- Все эндпоинты возвращают корректные HTTP коды
- JWT токены валидны и содержат правильный payload
- Guard корректно блокирует доступ
- go test ./... проходит

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Покрытие тестами
- Замечания по безопасности
```
