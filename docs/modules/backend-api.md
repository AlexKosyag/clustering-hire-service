# Модуль: Backend API (Основной API)

## 1. Назначение

Основной API-слой, объединяющий все модули. Отвечает за:
- CRUD операции для всех сущностей
- Оркестрацию бизнес-логики
- Валидацию входных данных
- API Gateway функции

**Зависимости**: database, auth
**Блокирует**: frontend, matching-engine

## 2. Архитектура

```
src/modules/
├── users/               # Управление пользователями
├── candidates/          # Профили кандидатов
├── employers/           # Профили работодателей
├── vacancies/           # Вакансии (optional)
├── invitations/         # Приглашения
├── notifications/       # Уведомления
└── common/              # Shared utilities
    ├── guards/
    ├── decorators/
    ├── pipes/
    ├── filters/
    └── interceptors/
```

## 3. API Endpoints

### 3.1. Candidates

#### Получить профиль
```http
GET /api/candidates/profile
Authorization: Bearer {candidate-token}

Response 200:
{
  "id": "uuid",
  "firstName": "Иван",
  "lastName": "Иванов",
  "specialization": "BACKEND",
  "declaredGrade": "MIDDLE",
  "verifiedGrade": "MIDDLE_PLUS",
  "stack": ["Node.js", "PostgreSQL", "Docker"],
  "testStatus": "PASSED",
  "achievements": [...],
  "resumePdfUrl": "https://..."
}
```

#### Обновить профиль
```http
PUT /api/candidates/profile
Authorization: Bearer {candidate-token}
Content-Type: application/json

{
  "firstName": "Иван",
  "lastName": "Иванов",
  "phone": "+7 999 123-45-67",
  "city": "Москва",
  "bio": "Backend-разработчик с 4-летним опытом...",
  "specialization": "BACKEND",
  "declaredGrade": "MIDDLE",
  "stack": ["Node.js", "PostgreSQL", "Docker", "Redis"],
  "roles": ["Backend Developer", "Team Lead"],
  "experienceYears": 4,
  "softSkills": ["communication", "leadership"]
}

Response 200: { ... updated profile ... }
```

#### Загрузить резюме PDF
```http
POST /api/candidates/profile/resume
Authorization: Bearer {candidate-token}
Content-Type: multipart/form-data

file: [PDF file]

Response 200:
{
  "resumePdfUrl": "https://minio.../resumes/user123/resume.pdf",
  "parsed": {
    "name": "Иван Иванов",
    "skills": ["Node.js", "PostgreSQL"],
    "experience": "4 года"
  }
}
```

#### Получить категорию
```http
GET /api/candidates/category
Authorization: Bearer {candidate-token}

Response 200:
{
  "specialization": "BACKEND",
  "declaredGrade": "MIDDLE",
  "verifiedGrade": "MIDDLE_PLUS",
  "testStatus": "PASSED",
  "lastTestDate": "2026-01-10T12:00:00Z",
  "canRetakeAt": "2026-04-10T12:00:00Z"
}
```

### 3.2. Employers

#### Получить профиль
```http
GET /api/employers/profile
Authorization: Bearer {employer-token}

Response 200:
{
  "id": "uuid",
  "companyName": "ООО ТехноСтар",
  "companyDescription": "Разработка SaaS-продуктов",
  "industry": "IT",
  "website": "https://technostar.ru",
  "logoUrl": "https://..."
}
```

#### Обновить профиль
```http
PUT /api/employers/profile
Authorization: Bearer {employer-token}
Content-Type: application/json

{
  "companyName": "ООО ТехноСтар",
  "companyDescription": "Разработка SaaS-продуктов для бизнеса",
  "industry": "IT",
  "website": "https://technostar.ru",
  "contactPerson": "Петр Петров",
  "contactEmail": "hr@technostar.ru",
  "contactPhone": "+7 999 987-65-43"
}
```

#### Описать потребность (ключевой эндпоинт)
```http
POST /api/employers/need
Authorization: Bearer {employer-token}
Content-Type: application/json

{
  "description": "Ищем backend-разработчика для работы над высоконагруженным API. Нужен опыт с Node.js и PostgreSQL, понимание микросервисной архитектуры.",
  "specialization": "BACKEND",
  "grade": "MIDDLE_PLUS",
  "stack": ["Node.js", "PostgreSQL", "Docker", "Redis"]
}

Response 201:
{
  "id": "uuid",
  "description": "...",
  "specialization": "BACKEND",
  "candidatesCount": 15,
  "topCandidates": [
    {
      "id": "uuid",
      "displayName": "Иван И.",
      "matchScore": 92.3,
      "matchExplanation": { ... }
    }
  ]
}
```

#### Поиск по базе кандидатов
```http
GET /api/employers/candidates/search?specialization=BACKEND&grade=MIDDLE&stack=Node.js,PostgreSQL&hasAchievements=true&limit=20&offset=0
Authorization: Bearer {employer-token}

Response 200:
{
  "total": 45,
  "candidates": [
    {
      "id": "uuid",
      "displayName": "Иван И.",
      "specialization": "BACKEND",
      "verifiedGrade": "MIDDLE",
      "stack": ["Node.js", "PostgreSQL"],
      "achievements": ["Финалист ICPC 2024"],
      "lastActive": "2026-01-10"
    }
  ]
}
```

### 3.3. Invitations

#### Отправить приглашение
```http
POST /api/invitations
Authorization: Bearer {employer-token}
Content-Type: application/json

{
  "candidateId": "uuid",
  "vacancyId": "uuid",  // optional
  "message": "Мы заинтересованы в вашем опыте работы с микросервисами. Приглашаем на позицию Senior Backend Developer.",
  "salaryFrom": 200000,
  "salaryTo": 300000
}

Response 201:
{
  "id": "uuid",
  "status": "SENT",
  "createdAt": "2026-01-10T12:00:00Z"
}
```

#### Получить входящие приглашения (кандидат)
```http
GET /api/invitations
Authorization: Bearer {candidate-token}

Response 200:
{
  "invitations": [
    {
      "id": "uuid",
      "companyName": "ООО ТехноСтар",
      "message": "Мы заинтересованы в вашем опыте...",
      "salaryFrom": 200000,
      "salaryTo": 300000,
      "status": "SENT",
      "createdAt": "2026-01-10T12:00:00Z"
    }
  ]
}
```

#### Принять/отклонить приглашение
```http
PUT /api/invitations/{id}/accept
Authorization: Bearer {candidate-token}

Response 200:
{
  "id": "uuid",
  "status": "ACCEPTED",
  "employerContacts": {
    "companyName": "ООО ТехноСтар",
    "contactPerson": "Петр Петров",
    "contactEmail": "hr@technostar.ru",
    "contactPhone": "+7 999 987-65-43"
  }
}

---

PUT /api/invitations/{id}/reject
Authorization: Bearer {candidate-token}

Response 200:
{
  "id": "uuid",
  "status": "REJECTED"
}
```

### 3.4. Vacancies (optional)

#### Создать вакансию
```http
POST /api/vacancies
Authorization: Bearer {employer-token}
Content-Type: application/json

{
  "title": "Senior Backend Developer",
  "description": "Разработка высоконагруженного API...",
  "specialization": "BACKEND",
  "grade": "SENIOR",
  "stack": ["Node.js", "PostgreSQL", "Docker"],
  "salaryFrom": 250000,
  "salaryTo": 400000
}

Response 201: { ... created vacancy ... }
```

#### Список вакансий
```http
GET /api/vacancies?specialization=BACKEND&grade=SENIOR&limit=20

Response 200:
{
  "total": 50,
  "vacancies": [
    {
      "id": "uuid",
      "title": "Senior Backend Developer",
      "companyName": "ООО ТехноСтар",
      "specialization": "BACKEND",
      "grade": "SENIOR",
      "salaryFrom": 250000,
      "salaryTo": 400000,
      "createdAt": "2026-01-10"
    }
  ]
}
```

#### Откликнуться на вакансию
```http
POST /api/vacancies/{id}/apply
Authorization: Bearer {candidate-token}
Content-Type: application/json

{
  "coverLetter": "Меня заинтересовала ваша вакансия..."
}

Response 201: { ... application ... }
```

## 4. Общие компоненты

### 4.1. Global Exception Filter

```typescript
// common/filters/http-exception.filter.ts

@Catch(HttpException)
export class HttpExceptionFilter implements ExceptionFilter {
  catch(exception: HttpException, host: ArgumentsHost) {
    const ctx = host.switchToHttp();
    const response = ctx.getResponse<Response>();
    const request = ctx.getRequest<Request>();
    const status = exception.getStatus();
    const message = exception.getResponse();

    response.status(status).json({
      statusCode: status,
      timestamp: new Date().toISOString(),
      path: request.url,
      message: typeof message === 'string' ? message : (message as any).message,
    });
  }
}
```

### 4.2. Logging Interceptor

```typescript
// common/interceptors/logging.interceptor.ts

@Injectable()
export class LoggingInterceptor implements NestInterceptor {
  private logger = new Logger('HTTP');

  intercept(context: ExecutionContext, next: CallHandler): Observable<any> {
    const req = context.switchToHttp().getRequest();
    const { method, url } = req;
    const now = Date.now();

    return next.handle().pipe(
      tap(() => {
        const res = context.switchToHttp().getResponse();
        this.logger.log(`${method} ${url} ${res.statusCode} ${Date.now() - now}ms`);
      }),
    );
  }
}
```

### 4.3. Transform Interceptor

```typescript
// common/interceptors/transform.interceptor.ts

@Injectable()
export class TransformInterceptor<T> implements NestInterceptor<T, Response<T>> {
  intercept(context: ExecutionContext, next: CallHandler): Observable<Response<T>> {
    return next.handle().pipe(
      map((data) => ({
        data,
        timestamp: new Date().toISOString(),
      })),
    );
  }
}
```

## 5. Чеклист валидации

### CRUD операции
- [ ] Все GET эндпоинты возвращают данные
- [ ] Все POST эндпоинты создают сущности
- [ ] Все PUT эндпоинты обновляют сущности
- [ ] Валидация DTO работает
- [ ] Ошибки возвращают корректные HTTP коды

### Бизнес-логика
- [ ] Кандидат не может обновить чужой профиль
- [ ] Работодатель не видит контакты кандидата до ACCEPTED
- [ ] Приглашение создаётся со статусом SENT
- [ ] Статус приглашения обновляется корректно
- [ ] Пагинация работает

### Безопасность
- [ ] Все эндпоинты защищены JWT
- [ ] RBAC работает (кандидат/работодатель)
- [ ] Rate limiting настроен
- [ ] CORS настроен

## 6. Промпт для агента

```
Ты — Backend-разработчик, специализирующийся на NestJS.

ТВОЯ ЗАДАЧА: Реализовать основной Backend API для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/backend-api.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных
- docs/modules/auth.md — модуль аутентификации

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать модули: users, candidates, employers, vacancies, invitations, notifications
2. Реализовать все CRUD операции
3. Создать DTO с валидацией
4. Реализовать guards и interceptors
5. Настроить Swagger (OpenAPI)
6. Написать e2e тесты для ключевых сценариев

СТЕК:
- NestJS 10
- Prisma 5
- class-validator
- @nestjs/swagger

ВАЛИДАЦИЯ:
- Swagger UI доступен на /api/docs
- Все эндпоинты документированы
- Тесты проходят

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Скриншот Swagger UI
- Результат тестов
```