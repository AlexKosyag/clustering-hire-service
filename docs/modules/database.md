# Модуль: Database (PostgreSQL + Redis + MinIO)

## 1. Назначение

Слой хранения данных платформы. PostgreSQL — основная реляционная БД для всех бизнес-сущностей. Redis — кэш, сессии, rate limiting. MinIO — хранилище файлов (резюме PDF, аватары).

**Зависимости**: НЕТ (фундаментальный модуль)
**Блокирует**: auth, backend-api, testing-engine, matching-engine

## 2. Схема данных PostgreSQL

### 2.1. ER-диаграмма (текстовое описание)

```
User (1) ──── (1) CandidateProfile
User (1) ──── (1) EmployerProfile
CandidateProfile (1) ──── (N) TestSession
CandidateProfile (1) ──── (N) CandidateAchievement
CandidateProfile (1) ──── (N) Invitation (as candidate)
EmployerProfile (1) ──── (N) Vacancy
EmployerProfile (1) ──── (N) Invitation (as employer)
TestSession (1) ──── (N) TestAnswer
TestCategory (1) ──── (N) TestQuestion
TestQuestion (1) ──── (N) TestAnswer
FSPAchievement (1) ──── (N) CandidateAchievement
```

### 2.2. Полная схема Prisma

```prisma
// prisma/schema.prisma

generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

// ============ ПОЛЬЗОВАТЕЛИ ============

enum UserRole {
  CANDIDATE
  EMPLOYER
  ADMIN
}

model User {
  id            String    @id @default(uuid())
  email         String    @unique
  passwordHash  String    @map("password_hash")
  role          UserRole
  isEmailVerified Boolean @default(false) @map("is_email_verified")
  emailVerifyToken String? @map("email_verify_token")
  refreshToken  String?   @map("refresh_token")
  createdAt     DateTime  @default(now()) @map("created_at")
  updatedAt     DateTime  @updatedAt @map("updated_at")

  candidateProfile CandidateProfile?
  employerProfile  EmployerProfile?

  @@map("users")
}

// ============ КАНДИДАТЫ ============

enum Specialization {
  BACKEND
  FRONTEND
  FULLSTACK
  MOBILE
  DEVOPS
  DATA_SCIENCE
  QA
  SECURITY
  EMBEDDED
  GAME_DEV
  ML_AI
  DESIGN_UX
  MANAGEMENT
}

enum Grade {
  JUNIOR
  MIDDLE
  MIDDLE_PLUS
  SENIOR
  LEAD
  PRINCIPAL
}

enum TestStatus {
  NOT_STARTED
  IN_PROGRESS
  COMPLETED
  FAILED
  PASSED
  UPGRADED
}

model CandidateProfile {
  id              String    @id @default(uuid())
  userId          String    @unique @map("user_id")
  
  // Основная информация
  firstName       String?   @map("first_name")
  lastName        String?   @map("last_name")
  middleName      String?   @map("middle_name")
  phone           String?
  city            String?
  bio             String?   @db.Text
  avatarUrl       String?   @map("avatar_url")
  
  // Профессиональные данные
  specialization  Specialization?
  declaredGrade   Grade?    @map("declared_grade")    // Самооценка
  verifiedGrade   Grade?    @map("verified_grade")    // Подтверждённый тестом
  stack           String[]  // Технологический стек
  roles           String[]  // Роли (team lead, architect, etc.)
  experienceYears Float?    @map("experience_years")
  softSkills      String[]  @map("soft_skills")
  
  // FSP данные
  fspId           String?   @unique @map("fsp_id")   // ID участника ФСП
  fspConnectedAt  DateTime? @map("fsp_connected_at")
  
  // Статус тестирования
  testStatus      TestStatus @default(NOT_STARTED) @map("test_status")
  lastTestDate    DateTime?  @map("last_test_date")
  testCooldownUntil DateTime? @map("test_cooldown_until") // Не раньше чем...
  
  // Resume
  resumePdfUrl    String?   @map("resume_pdf_url")
  resumeParsed    Json?     @map("resume_parsed")    // Распарсенное резюме
  
  // Настройки приватности
  showEmail       Boolean   @default(false) @map("show_email")
  showPhone       Boolean   @default(false) @map("show_phone")
  showFullName    Boolean   @default(true) @map("show_full_name")
  consentToProcessing Boolean @default(false) @map("consent_to_processing")
  
  // Векторное представление для semantic matching
  profileEmbedding Unsupported("vector(384)")? @map("profile_embedding")
  
  createdAt       DateTime  @default(now()) @map("created_at")
  updatedAt       DateTime  @updatedAt @map("updated_at")

  user            User      @relation(fields: [userId], references: [id], onDelete: Cascade)
  testSessions    TestSession[]
  achievements    CandidateAchievement[]
  invitations     Invitation[] @relation("CandidateInvitations")
  dailyTasks      DailyTask[]

  @@index([specialization, verifiedGrade])
  @@index([testStatus])
  @@index([fspId])
  @@map("candidate_profiles")
}

// ============ РАБОТОДАТЕЛИ ============

model EmployerProfile {
  id              String    @id @default(uuid())
  userId          String    @unique @map("user_id")
  
  companyName     String    @map("company_name")
  companyDescription String? @map("company_description") @db.Text
  industry        String?
  website         String?
  logoUrl         String?   @map("logo_url")
  contactPerson   String?   @map("contact_person")
  contactEmail    String?   @map("contact_email")
  contactPhone    String?   @map("contact_phone")
  
  // Верификация (за рамками MVP)
  isVerified      Boolean   @default(false) @map("is_verified")
  verifiedAt      DateTime? @map("verified_at")
  
  createdAt       DateTime  @default(now()) @map("created_at")
  updatedAt       DateTime  @updatedAt @map("updated_at")

  user            User      @relation(fields: [userId], references: [id], onDelete: Cascade)
  vacancies       Vacancy[]
  invitations     Invitation[] @relation("EmployerInvitations")
  needs           EmployerNeed[]

  @@map("employer_profiles")
}

// ============ ВАКАНСИИ ============

model Vacancy {
  id              String    @id @default(uuid())
  employerId      String    @map("employer_id")
  
  title           String
  description     String    @db.Text
  specialization  Specialization
  grade           Grade
  stack           String[]
  salaryFrom      Int       @map("salary_from")     // В рублях
  salaryTo        Int       @map("salary_to")        // В рублях
  
  isActive        Boolean   @default(true) @map("is_active")
  
  createdAt       DateTime  @default(now()) @map("created_at")
  updatedAt       DateTime  @updatedAt @map("updated_at")

  employer        EmployerProfile @relation(fields: [employerId], references: [id])

  @@index([specialization, grade])
  @@index([isActive])
  @@map("vacancies")
}

// ============ ПОТРЕБНОСТИ РАБОТОДАТЕЛЯ (для matching) ============

model EmployerNeed {
  id              String    @id @default(uuid())
  employerId      String    @map("employer_id")
  
  description     String    @db.Text    // Текстовое описание потребности
  specialization  Specialization?
  grade           Grade?
  stack           String[]
  
  // Векторное представление для semantic matching
  needEmbedding   Unsupported("vector(384)")? @map("need_embedding")
  
  createdAt       DateTime  @default(now()) @map("created_at")

  employer        EmployerProfile @relation(fields: [employerId], references: [id])

  @@map("employer_needs")
}

// ============ ПРИГЛАШЕНИЯ ============

enum InvitationStatus {
  SENT            // Отправлено
  VIEWED          // Просмотрено
  ACCEPTED        // Принято
  REJECTED        // Отклонено
  WITHDRAWN       // Отозвано работодателем
}

model Invitation {
  id              String    @id @default(uuid())
  
  employerId      String    @map("employer_id")
  candidateId     String    @map("candidate_id")
  vacancyId       String?   @map("vacancy_id")  // Может быть без привязки к вакансии
  
  message         String?   @db.Text             // Описание предложения
  salaryFrom      Int       @map("salary_from")
  salaryTo        Int       @map("salary_to")
  
  status          InvitationStatus @default(SENT)
  statusUpdatedAt DateTime  @default(now()) @map("status_updated_at")
  
  createdAt       DateTime  @default(now()) @map("created_at")
  updatedAt       DateTime  @updatedAt @map("updated_at")

  employer        EmployerProfile @relation("EmployerInvitations", fields: [employerId], references: [id])
  candidate       CandidateProfile @relation("CandidateInvitations", fields: [candidateId], references: [id])
  vacancy         Vacancy? @relation(fields: [vacityId], references: [id])

  @@index([candidateId, status])
  @@index([employerId, status])
  @@map("invitations")
}

// ============ ТЕСТИРОВАНИЕ ============

model TestCategory {
  id              String    @id @default(uuid())
  name            String    @unique
  description     String?
  specialization  Specialization
  
  questions       TestQuestion[]
  sessions        TestSession[]

  @@map("test_categories")
}

model TestQuestion {
  id              String    @id @default(uuid())
  categoryId      String    @map("category_id")
  
  // Тип вопроса
  questionType    String    @map("question_type") // multiple_choice, coding, open_text, scenario
  
  // Содержание
  text            String    @db.Text
  codeSnippet     String?   @map("code_snippet") @db.Text  // Для задач с кодом
  options         Json?     // Варианты ответов [{id, text, isCorrect}]
  correctAnswer   String?   @map("correct_answer") @db.Text
  explanation     String?   @db.Text  // Объяснение правильного ответа
  
  // Параметры для CAT
  difficulty      Float     @default(0.5)  // 0.0 (легко) - 1.0 (сложно)
  discrimination  Float     @default(1.0)  // Дискриминативность
  guessing        Float     @default(0.25) // Вероятность угадывания
  
  // Anti-cheat
  isUnique        Boolean   @default(false) @map("is_unique") // Уникальные задания
  poolTag         String?   @map("pool_tag") // Тег пула для группировки
  
  category        TestCategory @relation(fields: [categoryId], references: [id])
  answers         TestAnswer[]

  @@index([categoryId, difficulty])
  @@map("test_questions")
}

enum SessionStatus {
  IN_PROGRESS
  COMPLETED
  TIMED_OUT
  ABANDONED
}

model TestSession {
  id              String    @id @default(uuid())
  candidateId     String    @map("candidate_id")
  categoryId      String    @map("category_id")
  
  status          SessionStatus @default(IN_PROGRESS)
  startedAt       DateTime  @default(now()) @map("started_at")
  completedAt     DateTime? @map("completed_at")
  timeLimitMinutes Int      @default(60) @map("time_limit_minutes")
  
  // Результаты
  score           Float?    // 0-100
  rawScore        Float?    @map("raw_score")
  maxScore        Float?    @map("max_score")
  gradeBefore     Grade?    @map("grade_before")
  gradeAfter      Grade?    @map("grade_after")
  
  // CAT параметры
  currentDifficulty Float   @default(0.5) @map("current_difficulty")
  thetaEstimate   Float?    @map("theta_estimate") // IRT theta
  
  candidate       CandidateProfile @relation(fields: [candidateId], references: [id])
  category        TestCategory @relation(fields: [categoryId], references: [id])
  answers         TestAnswer[]

  @@index([candidateId, status])
  @@map("test_sessions")
}

model TestAnswer {
  id              String    @id @default(uuid())
  sessionId       String    @map("session_id")
  questionId      String    @map("question_id")
  
  answerText      String?   @map("answer_text") @db.Text  // Ответ кандидата
  isCorrect       Boolean?
  score           Float?    // 0-1
  timeSpentSeconds Int?     @map("time_spent_seconds")
  
  answeredAt      DateTime  @default(now()) @map("answered_at")

  session         TestSession @relation(fields: [sessionId], references: [id])
  question        TestQuestion @relation(fields: [questionId], references: [id])

  @@unique([sessionId, questionId])
  @@map("test_answers")
}

// ============ ДОСТИЖЕНИЯ ФСП ============

model FSPAchievement {
  id              String    @id @default(uuid())
  name            String
  description     String?
  category        String    // competition, event, certification
  year            Int?
  place           String?   // "1 место", "Финалист", etc.
  points          Int       @default(0) // Вес достижения
  
  candidateLinks  CandidateAchievement[]

  @@map("fsp_achievements")
}

model CandidateAchievement {
  id              String    @id @default(uuid())
  candidateId     String    @map("candidate_id")
  achievementId   String    @map("achievement_id")
  verifiedAt      DateTime  @default(now()) @map("verified_at")
  
  candidate       CandidateProfile @relation(fields: [candidateId], references: [id])
  achievement     FSPAchievement @relation(fields: [achievementId], references: [id])

  @@unique([candidateId, achievementId])
  @@map("candidate_achievements")
}

// ============ ЕЖЕДНЕВНЫЕ ЗАДАНИЯ ============

enum DailyTaskStatus {
  PENDING
  IN_PROGRESS
  COMPLETED
  SKIPPED
  EXPIRED
}

model DailyTask {
  id              String    @id @default(uuid())
  candidateId     String    @map("candidate_id")
  
  title           String
  description     String    @db.Text
  difficulty      String    // easy, medium, hard
  expectedTimeMinutes Int   @map("expected_time_minutes")
  
  status          DailyTaskStatus @default(PENDING)
  candidateAnswer String?   @map("candidate_answer") @db.Text
  score           Float?
  
  assignedAt      DateTime  @default(now()) @map("assigned_at")
  dueAt           DateTime  @map("due_at")
  completedAt     DateTime? @map("completed_at")
  
  candidate       CandidateProfile @relation(fields: [candidateId], references: [id])

  @@index([candidateId, status])
  @@map("daily_tasks")
}
```

## 3. Redis структуры

### 3.1. Сессии пользователей
```
Key: session:{userId}
Value: { refreshToken, role, lastActive }
TTL: 7 days
```

### 3.2. Rate limiting
```
Key: ratelimit:{ip}:{endpoint}
Value: counter
TTL: 60 seconds (100 requests)
```

### 3.3. Кэш подборок кандидатов
```
Key: matching:cache:{employerNeedHash}
Value: JSON array of candidate IDs with scores
TTL: 15 minutes
```

### 3.4. Тестовые сессии (для быстрого доступа)
```
Key: test:session:{sessionId}
Value: { currentQuestion, answers, timeRemaining }
TTL: session duration + 1 hour
```

## 4. MinIO buckets

```
fsp-platform/
├── resumes/           # PDF резюме кандидатов
│   └── {userId}/resume.pdf
├── avatars/           # Аватары пользователей
│   └── {userId}/avatar.{ext}
├── logos/             # Логотипы компаний
│   └── {employerId}/logo.{ext}
└── generated/         # Сгенерированные PDF профилей
    └── {userId}/profile.pdf
```

## 5. Тестовые данные (seed)

### 5.1. Тестовые категории и вопросы

```typescript
// prisma/seed.ts

const testCategories = [
  {
    name: "Backend Development",
    specialization: "BACKEND",
    questions: [
      // Легкие (difficulty: 0.2)
      {
        questionType: "multiple_choice",
        text: "Что такое REST API?",
        options: [
          { id: "a", text: "Протокол передачи файлов", isCorrect: false },
          { id: "b", text: "Архитектурный стиль для веб-сервисов", isCorrect: true },
          { id: "c", text: "База данных", isCorrect: false },
          { id: "d", text: "Язык программирования", isCorrect: false },
        ],
        difficulty: 0.2,
      },
      // Средние (difficulty: 0.5)
      {
        questionType: "coding",
        text: "Напишите SQL-запрос для получения топ-5 пользователей по количеству заказов",
        codeSnippet: `-- Таблицы:\n-- users(id, name)\n-- orders(id, user_id, amount, created_at)`,
        correctAnswer: `SELECT u.name, COUNT(o.id) as order_count\nFROM users u\nJOIN orders o ON u.id = o.user_id\nGROUP BY u.id, u.name\nORDER BY order_count DESC\nLIMIT 5;`,
        difficulty: 0.5,
      },
      // Сложные (difficulty: 0.8)
      {
        questionType: "scenario",
        text: "Ваше API получает 10,000 запросов в секунду. База данных не справляется. Опишите стратегию оптимизации.",
        difficulty: 0.8,
      },
    ],
  },
  // ...更多 категории
];
```

### 5.2. Тестовые пользователи

```
Кандидаты:
- Иванов Иван (BACKEND, MIDDLE, stack: [Node.js, PostgreSQL])
- Петрова Анна (FRONTEND, JUNIOR, stack: [React, TypeScript])
- Сидоров Дмитрий (FULLSTACK, SENIOR, stack: [Python, React, Docker])

Работодатели:
- ООО "ТехноСтар" (IT, разработка ПО)
- Яндекс (Интернет-сервисы)
```

## 6. Чеклист валидации модуля

- [ ] Prisma schema синтаксически корректен (`npx prisma validate`)
- [ ] Миграции создаются без ошибок (`npx prisma migrate dev`)
- [ ] Все связи (relations) корректно определены
- [ ] Индексы созданы для частых запросов
- [ ] Seed-скрипт заполняет тестовые данные
- [ ] Redis подключается и базовые операции работают
- [ ] MinIO bucket создан, загрузка/скачивание файлов работает
- [ ] Транзакции работают корректно (проверить на invitation flow)
- [ ] Soft delete не нужен (hard delete с каскадом через onDelete: Cascade)
- [ ] Все enum-ы содержат нужные значения

## 7. Промпт для агента

```
Ты — Backend-разработчик, специализирующийся на проектировании баз данных.

ТВОЯ ЗАДАЧА: Реализовать модуль database для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/database.md — ТЗ на модуль (этот файл)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать файл backend/prisma/schema.prisma с полной схемой данных
2. Создать файл backend/prisma/seed.ts с тестовыми данными
3. Создать файл backend/package.json с зависимостями (prisma, @prisma/client)
4. Создать файл backend/.env.example с переменными окружения
5. Создать docker-compose.yml с PostgreSQL, Redis, MinIO
6. Проверить что schema проходит валидацию

СТЕК:
- PostgreSQL 16
- Prisma 5
- Redis 7
- MinIO

ВАЛИДАЦИЯ:
- Запусти `npx prisma validate` — должен пройти без ошибок
- Запусти `npx prisma generate` — должен сгенерировать клиент
- Проверь что все связи корректны

ФОРМАТ ОТВЕТА: 
- Список созданных файлов с кратким описанием
- Результат валидации
- Любые замечания по схеме
```