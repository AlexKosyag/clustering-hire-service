# Модуль: FSP Integration (Интеграция с Федерацией спортивного программирования)

## 1. Назначение

Модуль отвечает за:
- Связь профиля кандидата с ID участника ФСП
- Получение и отображение достижений
- Обогащение профиля результатами соревнований
- Совместимость с Keycloak (OIDC)

**Зависимости**: database, auth
**Параллельно с**: frontend, matching-engine

## 2. Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                  FSP INTEGRATION MODULE                     │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  1. FSP ID Connection                                 │  │
│  │  - OAuth2/OIDC flow (future)                         │  │
│  │  - Manual ID entry (MVP)                             │  │
│  │  - Verification token                                │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  2. Achievement Fetcher                               │  │
│  │  - Abstract API client                                │  │
│  │  - Mock data for MVP                                  │  │
│  │  - Real API integration (future)                      │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  3. Profile Enrichment                                │  │
│  │  - Add achievements to profile                       │  │
│  │  - Generate embeddings                               │  │
│  │  - Update matching index                             │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 3. API Endpoints

### 3.1. Подключить ФСП ID

```http
POST /api/fsp/connect
Authorization: Bearer {token}
Content-Type: application/json

{
  "fspId": "FSP-12345"
}

Response 200:
{
  "connected": true,
  "fspId": "FSP-12345",
  "achievements": [
    {
      "id": "uuid",
      "name": "Финалист ICPC 2024",
      "category": "competition",
      "year": 2024,
      "place": "Финалист",
      "points": 10
    }
  ]
}
```

### 3.2. Получить достижения

```http
GET /api/fsp/achievements
Authorization: Bearer {token}

Response 200:
{
  "fspId": "FSP-12345",
  "achievements": [
    {
      "id": "uuid",
      "name": "Финалист ICPC 2024",
      "description": "Международная олимпиада по программированию",
      "category": "competition",
      "year": 2024,
      "place": "Финалист",
      "points": 10,
      "verifiedAt": "2026-01-10T12:00:00Z"
    },
    {
      "id": "uuid",
      "name": "Участник Codeforces Round #900",
      "category": "competition",
      "year": 2024,
      "place": "Top 100",
      "points": 5,
      "verifiedAt": "2026-01-10T12:00:00Z"
    }
  ],
  "totalPoints": 15
}
```

### 3.3. Отключить ФСП ID

```http
POST /api/fsp/disconnect
Authorization: Bearer {token}

Response 200:
{
  "connected": false
}
```

## 4. Реализация

### 4.1. FSP Service

```typescript
// services/fsp.service.ts

@Injectable()
export class FSPService {
  constructor(
    private prisma: PrismaService,
    private embeddingService: EmbeddingService,
  ) {}

  /**
   * Подключает ФСП ID к профилю кандидата
   */
  async connectFSP(userId: string, fspId: string): Promise<ConnectionResult> {
    // 1. Проверить что FSP ID не занят другим пользователем
    const existing = await this.prisma.candidateProfile.findFirst({
      where: { fspId },
    });
    if (existing && existing.userId !== userId) {
      throw new ConflictException('FSP ID already connected to another account');
    }

    // 2. Получить достижения (mock для MVP)
    const achievements = await this.fetchAchievements(fspId);

    // 3. Обновить профиль
    await this.prisma.candidateProfile.update({
      where: { userId },
      data: {
        fspId,
        fspConnectedAt: new Date(),
      },
    });

    // 4. Сохранить достижения
    for (const achievement of achievements) {
      await this.prisma.candidateAchievement.upsert({
        where: {
          candidateId_achievementId: {
            candidateId: userId,
            achievementId: achievement.id,
          },
        },
        update: {},
        create: {
          candidateId: userId,
          achievementId: achievement.id,
        },
      });
    }

    // 5. Обновить embedding профиля
    const profile = await this.prisma.candidateProfile.findUnique({
      where: { userId },
      include: { achievements: { include: { achievement: true } } },
    });
    const embedding = await this.embeddingService.generateProfileEmbedding(profile);
    await this.prisma.candidateProfile.update({
      where: { userId },
      data: { profileEmbedding: embedding },
    });

    return { connected: true, fspId, achievements };
  }

  /**
   * Получает достижения участника ФСП
   * 
   * Для MVP — возвращает моковые данные.
   * В продакшене — интеграция с реальным API ФСП.
   */
  async fetchAchievements(fspId: string): Promise<FSPAchievement[]> {
    // MOCK для MVP
    // В продакшене: вызов API ФСП
    // const response = await fetch(`${FSP_API_URL}/participants/${fspId}/achievements`);
    // return response.json();

    return this.getMockAchievements(fspId);
  }

  /**
   * Моковые достижения для демонстрации
   */
  private getMockAchievements(fspId: string): FSPAchievement[] {
    const mockData: Record<string, FSPAchievement[]> = {
      'FSP-12345': [
        {
          id: 'ach-1',
          name: 'Финалист ICPC 2024',
          description: 'Международная олимпиада по программированию',
          category: 'competition',
          year: 2024,
          place: 'Финалист',
          points: 10,
        },
        {
          id: 'ach-2',
          name: 'Победитель Codeforces Round #900',
          category: 'competition',
          year: 2024,
          place: '1 место',
          points: 8,
        },
      ],
      'FSP-67890': [
        {
          id: 'ach-3',
          name: 'Участник VK Cup 2024',
          category: 'competition',
          year: 2024,
          place: 'Top 50',
          points: 5,
        },
      ],
    };

    return mockData[fspId] || [];
  }

  /**
   * Получает профиль с достижениями
   */
  async getProfileWithAchievements(userId: string): Promise<CandidateWithAchievements> {
    const profile = await this.prisma.candidateProfile.findUnique({
      where: { userId },
      include: {
        achievements: {
          include: { achievement: true },
          orderBy: { achievement: { points: 'desc' } },
        },
      },
    });

    if (!profile) {
      throw new NotFoundException('Profile not found');
    }

    return {
      ...profile,
      totalPoints: profile.achievements.reduce(
        (sum, a) => sum + a.achievement.points,
        0,
      ),
    };
  }
}
```

### 4.2. Keycloak Integration (подготовка)

```typescript
// strategies/keycloak.strategy.ts

@Injectable()
export class KeycloakOIDCStrategy extends PassportStrategy(Strategy, 'keycloak') {
  constructor(
    configService: ConfigService,
    private prisma: PrismaService,
  ) {
    super({
      issuer: configService.get('KEYCLOAK_ISSUER'),
      authorizationURL: configService.get('KEYCLOAK_AUTH_URL'),
      tokenURL: configService.get('KEYCLOAK_TOKEN_URL'),
      clientID: configService.get('KEYCLOAK_CLIENT_ID'),
      clientSecret: configService.get('KEYCLOAK_CLIENT_SECRET'),
      callbackURL: configService.get('KEYCLOAK_CALLBACK_URL'),
    });
  }

  async validate(
    accessToken: string,
    refreshToken: string,
    profile: KeycloakProfile,
  ): Promise<User> {
    // 1. Найти или создать пользователя
    let user = await this.prisma.user.findUnique({
      where: { email: profile.email },
    });

    if (!user) {
      // Создать нового пользователя
      user = await this.prisma.user.create({
        data: {
          email: profile.email,
          passwordHash: '', // Пароль не нужен при OIDC
          role: this.mapKeycloakRole(profile),
          isEmailVerified: profile.emailVerified,
        },
      });

      // Создать профиль
      if (user.role === 'CANDIDATE') {
        await this.prisma.candidateProfile.create({
          data: {
            userId: user.id,
            firstName: profile.firstName,
            lastName: profile.lastName,
          },
        });
      }
    }

    return user;
  }

  private mapKeycloakRole(profile: KeycloakProfile): UserRole {
    if (profile.groups?.includes('employers')) return UserRole.EMPLOYER;
    return UserRole.CANDIDATE;
  }
}
```

## 5. Структура данных ФСП (проработка)

### 5.1. Что мы хотим получить от API ФСП

```typescript
interface FSPParticipant {
  id: string;              // FSP ID
  name: string;            // ФИО
  email?: string;          // Email (если разрешено)
  
  achievements: FSPAchievement[];
  competitions: FSPCompetition[];
  ratings: FSPRating[];
}

interface FSPAchievement {
  id: string;
  name: string;            // "Финалист ICPC 2024"
  description?: string;
  category: 'competition' | 'event' | 'certification';
  year: number;
  place?: string;          // "1 место", "Финалист", "Top 100"
  points: number;          // Вес достижения
  verified: boolean;       // Подтверждено ФСП
}

interface FSPCompetition {
  id: string;
  name: string;            // "ICPC World Finals 2024"
  date: string;
  format: 'individual' | 'team';
  result: string;          // "Финалист", "1 место"
  rating?: number;         // Рейтинг после соревнования
}

interface FSPRating {
  system: string;          // "codeforces", "icpc", "fsp"
  current: number;         // Текущий рейтинг
  max: number;             // Максимальный рейтинг
  rank?: string;           // "Master", "Grandmaster"
}
```

### 5.2. Как отображать в профиле

```tsx
// components/fsp/AchievementBadge.tsx
export function AchievementBadge({ achievement }: { achievement: FSPAchievement }) {
  const icon = {
    competition: <TrophyIcon />,
    event: <CalendarIcon />,
    certification: <CertificateIcon />,
  }[achievement.category];

  return (
    <div className="flex items-center gap-2 p-2 bg-yellow-50 rounded">
      {icon}
      <div>
        <p className="font-medium">{achievement.name}</p>
        <p className="text-sm text-gray-500">
          {achievement.place} · {achievement.year}
        </p>
      </div>
      <span className="ml-auto text-sm font-bold text-yellow-600">
        +{achievement.points} очков
      </span>
    </div>
  );
}
```

## 6. Чеклист валидации

### Подключение
- [ ] Подключение ФСП ID работает
- [ ] Достижения загружаются (mock)
- [ ] Достижения сохраняются в БД
- [ ] Дубликат FSP ID отклоняется
- [ ] Отключение работает

### Отображение
- [ ] Достижения отображаются в профиле
- [ ] Бейджи корректно отображают данные
- [ ] Общий балл достижений рассчитывается
- [ ] Достижения влияют на matching score

### Безопасность
- [ ] FSP ID привязан к пользователю
- [ ] Нельзя подключить чужой FSP ID
- [ ] Данные достижений верифицируются

### Future (Keycloak)
- [ ] Заготовка для OIDC стратегии создана
- [ ] Конфигурация Keycloak описана
- [ ] Маппинг ролей работает

## 7. Промпт для агента

```
Ты — Backend-разработчик с опытом интеграции внешних API.

ТВОЯ ЗАДАЧА: Реализовать модуль fsp-integration для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/fsp-integration.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных (FSPAchievement, CandidateAchievement)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/src/modules/fsp/ со всеми файлами
2. Реализовать FSPService — подключение, получение достижений
3. Создать mock данные для демонстрации
4. Реализовать API endpoints
5. Создать DTO с валидацией
6. Написать unit-тесты
7. Подготовить заготовку для Keycloak OIDC

СТЕК:
- NestJS 10
- Prisma 5
- Passport.js

ВАЛИДАЦИЯ:
- Подключение ФСП ID работает
- Mock достижения загружаются
- Достижения сохраняются в БД
- Тесты проходят

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Примеры mock данных
- Результат тестов
```