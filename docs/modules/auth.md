# Модуль: Auth (Аутентификация и авторизация)

## 1. Назначение

Модуль отвечает за:
- Регистрацию пользователей (кандидаты и работодатели)
- Аутентификацию по email + пароль
- JWT-based авторизация (access + refresh tokens)
- RBAC (Role-Based Access Control)
- Верификацию email
- Совместимость с Keycloak (OIDC)

**Зависимости**: database
**Блокирует**: backend-api, testing-engine, frontend

## 2. Архитектура модуля

```
src/modules/auth/
├── auth.module.ts           # NestJS модуль
├── auth.controller.ts       # Эндпоинты
├── auth.service.ts          # Бизнес-логика
├── dto/
│   ├── register.dto.ts      # DTO регистрации
│   ├── login.dto.ts         # DTO входа
│   └── token.dto.ts         # DTO токенов
├── guards/
│   ├── jwt.guard.ts         # Guard для JWT
│   ├── roles.guard.ts       # Guard для ролей
│   └── local.guard.ts       # Guard для login
├── strategies/
│   ├── jwt.strategy.ts      # Passport JWT strategy
│   ├── local.strategy.ts    # Passport Local strategy
│   └── keycloak.strategy.ts # OIDC strategy (future)
├── decorators/
│   ├── roles.decorator.ts   # @Roles() декоратор
│   └── current-user.decorator.ts # @CurrentUser()
└── interfaces/
    └── token-payload.interface.ts
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
  "message": "Logged out successfully"
}
```

### 3.5. Верификация email

```http
GET /api/auth/verify-email?token={verificationToken}

Response 200:
{
  "message": "Email verified successfully"
}

Errors:
- 400: Invalid or expired token
```

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

```typescript
// auth.service.ts

@Injectable()
export class AuthService {
  constructor(
    private prisma: PrismaService,
    private jwtService: JwtService,
    private configService: ConfigService,
    private mailService: MailService,
  ) {}

  async register(dto: RegisterDto): Promise<AuthResponse> {
    // 1. Проверить уникальность email
    const existing = await this.prisma.user.findUnique({
      where: { email: dto.email },
    });
    if (existing) throw new ConflictException('Email already exists');

    // 2. Хешировать пароль
    const passwordHash = await bcrypt.hash(dto.password, 12);

    // 3. Создать пользователя и профиль в транзакции
    const user = await this.prisma.$transaction(async (tx) => {
      const newUser = await tx.user.create({
        data: {
          email: dto.email,
          passwordHash,
          role: dto.role,
          emailVerifyToken: crypto.randomUUID(),
        },
      });

      // Создать пустой профиль в зависимости от роли
      if (dto.role === 'CANDIDATE') {
        await tx.candidateProfile.create({
          data: { userId: newUser.id },
        });
      } else if (dto.role === 'EMPLOYER') {
        await tx.employerProfile.create({
          data: { userId: newUser.id, companyName: '' },
        });
      }

      return newUser;
    });

    // 4. Отправить письмо для верификации
    await this.mailService.sendVerificationEmail(
      user.email,
      user.emailVerifyToken,
    );

    // 5. Сгенерировать токены
    const tokens = await this.generateTokens(user);
    await this.updateRefreshToken(user.id, tokens.refreshToken);

    return { ...tokens, user: this.sanitizeUser(user) };
  }

  async login(dto: LoginDto): Promise<AuthResponse> {
    // 1. Найти пользователя
    const user = await this.prisma.user.findUnique({
      where: { email: dto.email },
    });
    if (!user) throw new UnauthorizedException('Invalid credentials');

    // 2. Проверить пароль
    const isPasswordValid = await bcrypt.compare(
      dto.password,
      user.passwordHash,
    );
    if (!isPasswordValid) {
      throw new UnauthorizedException('Invalid credentials');
    }

    // 3. Проверить верификацию email (опционально для MVP)
    // if (!user.isEmailVerified) {
    //   throw new ForbiddenException('Email not verified');
    // }

    // 4. Сгенерировать токены
    const tokens = await this.generateTokens(user);
    await this.updateRefreshToken(user.id, tokens.refreshToken);

    return { ...tokens, user: this.sanitizeUser(user) };
  }

  async refreshTokens(userId: string, refreshToken: string): Promise<Tokens> {
    const user = await this.prisma.user.findUnique({
      where: { id: userId },
    });
    if (!user || !user.refreshToken) {
      throw new UnauthorizedException('Access denied');
    }

    const rtMatches = await bcrypt.compare(refreshToken, user.refreshToken);
    if (!rtMatches) throw new UnauthorizedException('Access denied');

    const tokens = await this.generateTokens(user);
    await this.updateRefreshToken(user.id, tokens.refreshToken);

    return tokens;
  }

  async logout(userId: string): Promise<void> {
    await this.prisma.user.update({
      where: { id: userId },
      data: { refreshToken: null },
    });
  }

  private async generateTokens(user: User): Promise<Tokens> {
    const payload: TokenPayload = {
      sub: user.id,
      email: user.email,
      role: user.role,
    };

    const [accessToken, refreshToken] = await Promise.all([
      this.jwtService.signAsync(payload, {
        secret: this.configService.get('JWT_SECRET'),
        expiresIn: '15m',
      }),
      this.jwtService.signAsync(payload, {
        secret: this.configService.get('JWT_REFRESH_SECRET'),
        expiresIn: '7d',
      }),
    ]);

    return { accessToken, refreshToken };
  }

  private async updateRefreshToken(
    userId: string,
    refreshToken: string,
  ): Promise<void> {
    const hash = await bcrypt.hash(refreshToken, 12);
    await this.prisma.user.update({
      where: { id: userId },
      data: { refreshToken: hash },
    });
  }

  private sanitizeUser(user: User) {
    const { passwordHash, refreshToken, emailVerifyToken, ...result } = user;
    return result;
  }
}
```

### 4.2. JWT Strategy

```typescript
// strategies/jwt.strategy.ts

@Injectable()
export class JwtStrategy extends PassportStrategy(Strategy) {
  constructor(configService: ConfigService, prisma: PrismaService) {
    super({
      jwtFromRequest: ExtractJwt.fromAuthHeaderAsBearerToken(),
      ignoreExpiration: false,
      secretOrKey: configService.get('JWT_SECRET'),
    });
  }

  async validate(payload: TokenPayload) {
    const user = await this.prisma.user.findUnique({
      where: { id: payload.sub },
      include: {
        candidateProfile: true,
        employerProfile: true,
      },
    });
    if (!user) throw new UnauthorizedException();
    return user;
  }
}
```

### 4.3. Guards

```typescript
// guards/jwt.guard.ts
@Injectable()
export class JwtAuthGuard extends AuthGuard('jwt') {}

// guards/roles.guard.ts
@Injectable()
export class RolesGuard implements CanActivate {
  constructor(private reflector: Reflector) {}

  canActivate(context: ExecutionContext): boolean {
    const requiredRoles = this.reflector.getAllAndOverride<UserRole[]>(
      ROLES_KEY,
      [context.getHandler(), context.getClass()],
    );
    if (!requiredRoles) return true;

    const { user } = context.switchToHttp().getRequest();
    return requiredRoles.some((role) => user.role === role);
  }
}

// guards/roles.decorator.ts
export const ROLES_KEY = 'roles';
export const Roles = (...roles: UserRole[]) =>
  SetMetadata(ROLES_KEY, roles);
```

### 4.4. Refresh Token Guard (cookie-based)

```typescript
// guards/refresh-token.guard.ts
@Injectable()
export class RefreshTokenGuard implements CanActivate {
  canActivate(context: ExecutionContext): boolean {
    const request = context.switchToHttp().getRequest();
    const refreshToken = request.cookies?.refreshToken;
    if (!refreshToken) throw new UnauthorizedException('No refresh token');
    request.body.refreshToken = refreshToken;
    return true;
  }
}
```

## 5. DTO (Data Transfer Objects)

```typescript
// dto/register.dto.ts
export class RegisterDto {
  @IsEmail()
  email: string;

  @IsString()
  @MinLength(8)
  @Matches(/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/, {
    message: 'Password must contain uppercase, lowercase and number',
  })
  password: string;

  @IsEnum(UserRole)
  @IsIn([UserRole.CANDIDATE, UserRole.EMPLOYER])
  role: UserRole;
}

// dto/login.dto.ts
export class LoginDto {
  @IsEmail()
  email: string;

  @IsString()
  password: string;
}
```

## 6. Интеграция с Keycloak (подготовка)

```typescript
// strategies/keycloak.strategy.ts
// Реализуется после хакатона, заготовка:

@Injectable()
export class KeycloakStrategy extends PassportStrategy(Strategy, 'keycloak') {
  constructor(configService: ConfigService) {
    super({
      issuer: configService.get('KEYCLOAK_ISSUER'),
      authorizationURL: configService.get('KEYCLOAK_AUTH_URL'),
      tokenURL: configService.get('KEYCLOAK_TOKEN_URL'),
      clientID: configService.get('KEYCLOAK_CLIENT_ID'),
      clientSecret: configService.get('KEYCLOAK_CLIENT_SECRET'),
      callbackURL: configService.get('KEYCLOAK_CALLBACK_URL'),
    });
  }

  async validate(accessToken: string, refreshToken: string, profile: any) {
    // Map Keycloak profile to local user
    return {
      email: profile.email,
      role: profile.groups?.includes('employers') ? 'EMPLOYER' : 'CANDIDATE',
    };
  }
}
```

## 7. Переменные окружения

```env
# JWT
JWT_SECRET=your-super-secret-jwt-key-min-32-chars
JWT_REFRESH_SECRET=your-super-secret-refresh-key-min-32-chars

# Email (для верификации)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASS=your-app-password
SMTP_FROM=noreply@fsp-platform.ru

# Keycloak (future)
KEYCLOAK_ISSUER=http://localhost:8080/realms/fsp
KEYCLOAK_AUTH_URL=http://localhost:8080/realms/fsp/protocol/openid-connect/auth
KEYCLOAK_TOKEN_URL=http://localhost:8080/realms/fsp/protocol/openid-connect/token
KEYCLOAK_CLIENT_ID=fsp-platform
KEYCLOAK_CLIENT_SECRET=your-client-secret
KEYCLOAK_CALLBACK_URL=http://localhost:3000/api/auth/keycloak/callback
```

## 8. Чеклист валидации

- [ ] Регистрация создаёт User + Profile в транзакции
- [ ] Дубликат email отклоняется с 409
- [ ] Пароль хешируется через bcrypt (12 раундов)
- [ ] Login возвращает access + refresh токены
- [ ] Access token истекает через 15 минут
- [ ] Refresh token истекает через 7 дней
- [ ] Refresh token хранится в HttpOnly cookie
- [ ] Обновление токена работает корректно
- [ ] Logout инвалидирует refresh token
- [ ] RolesGuard блокирует доступ неавторизованным ролям
- [ ] @CurrentUser() декоратор возвращает пользователя
- [ ] Валидация DTO работает (class-validator)
- [ ] Ошибки возвращают корректные HTTP коды

## 9. Промпт для агента

```
Ты — Backend-разработчик, специализирующийся на NestJS и аутентификации.

ТВОЯ ЗАДАЧА: Реализовать модуль auth для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/auth.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных (модуль User, UserRole)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/src/modules/auth/ со всеми файлами
2. Реализовать регистрацию, логин, refresh, logout
3. Реализовать JWT strategy + guards
4. Реализовать Roles guard + decorator
5. Создать DTO с валидацией
6. Подготовить заготовку для Keycloak strategy
7. Написать unit-тесты для auth.service.ts

СТЕК:
- NestJS 10
- Passport.js + @nestjs/passport
- @nestjs/jwt
- bcrypt
- class-validator

ВАЛИДАЦИЯ:
- Все эндпоинты возвращают корректные HTTP коды
- JWT токены валидны и содержат правильный payload
- Guards корректно блокируют доступ
- Тесты проходят

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Покрытие тестами
- Замечания по безопасности
```