# Модуль: Frontend — Личный кабинет кандидата

## 1. Назначение

Веб-приложение для кандидатов. Отвечает за:
- Регистрацию и авторизацию
- Заполнение профиля и резюме
- Прохождение тестирования
- Просмотр приглашений
- Управление настройками приватности

**Зависимости**: auth, backend-api, testing-engine
**Параллельно с**: frontend-employer

## 2. Архитектура

```
src/app/
├── (auth)/
│   ├── login/page.tsx           # Страница входа
│   ├── register/page.tsx        # Страница регистрации
│   └── layout.tsx               # Layout без sidebar
├── candidate/
│   ├── layout.tsx               # Layout с sidebar
│   ├── page.tsx                 # Dashboard (главная)
│   ├── profile/
│   │   ├── page.tsx             # Просмотр профиля
│   │   └── edit/page.tsx        # Редактирование профиля
│   ├── testing/
│   │   ├── page.tsx             # Тестирование (до/после)
│   │   └── session/[id]/page.tsx # Прохождение теста
│   ├── invitations/
│   │   └── page.tsx             # Входящие приглашения
│   ├── vacancies/
│   │   └── page.tsx             # Список вакансий
│   └── settings/
│       └── page.tsx             # Настройки
└── components/
    ├── layout/
    │   ├── Sidebar.tsx
    │   ├── Header.tsx
    │   └── MobileNav.tsx
    ├── profile/
    │   ├── ProfileCard.tsx
    │   ├── ProfileForm.tsx
    │   ├── ResumeUpload.tsx
    │   └── AchievementBadge.tsx
    ├── testing/
    │   ├── QuestionCard.tsx
    │   ├── Timer.tsx
    │   ├── ProgressBar.tsx
    │   ├── CodeEditor.tsx
    │   └── ResultSummary.tsx
    ├── invitations/
    │   ├── InvitationCard.tsx
    │   └── InvitationDetail.tsx
    └── ui/
        ├── Button.tsx
        ├── Input.tsx
        ├── Modal.tsx
        └── Toast.tsx
```

## 3. Страницы и компоненты

### 3.1. Регистрация / Вход

**Страница**: `/login`, `/register`

**Компоненты**:
- `AuthForm` — форма с email/password
- `RoleSelector` — выбор роли (кандидат/работодатель)
- `SocialLogin` — кнопки OAuth (future)

**UX**:
- Валидация на клиенте (Zod)
- Показ ошибок от API
- Редирект после успешного входа

```tsx
// app/(auth)/register/page.tsx
export default function RegisterPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full space-y-8">
        <div>
          <Image src="/fsp-logo.svg" alt="ФСП" width={120} height={40} />
          <h2>Регистрация на платформе</h2>
        </div>
        <RegisterForm />
      </div>
    </div>
  );
}
```

### 3.2. Dashboard (Главная)

**Страница**: `/candidate`

**Компоненты**:
- `CategoryStatus` — текущая категория и грейд
- `TestStatus` — статус тестирования (пройден/не пройден/cooldown)
- `RecentInvitations` — последние 3 приглашения
- `DailyTask` — ежедневное задание (если есть)
- `AchievementsSummary` — краткая сводка достижений

**UX**:
- Быстрый доступ к ключевым действиям
- Визуализация статуса (цветовые индикаторы)
- Адаптивная сетка

### 3.3. Профиль

**Страница**: `/candidate/profile`, `/candidate/profile/edit`

**Компоненты**:
- `ProfileCard` — карточка профиля (view mode)
- `ProfileForm` — форма редактирования
- `ResumeUpload` — загрузка PDF с парсингом
- `AchievementBadge` — бейдж достижения ФСП
- `StackTags` — теги технологического стека

**Функциональность**:
- Ручное заполнение по форме
- Загрузка PDF с автораспознаванием
- Автогенерация стандартизированного PDF-профиля
- Превью профиля (как видит работодатель)

```tsx
// components/profile/ProfileForm.tsx
export function ProfileForm({ profile }: { profile: CandidateProfile }) {
  const form = useForm<ProfileFormData>({
    resolver: zodResolver(profileSchema),
    defaultValues: profile,
  });

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)}>
        <FormField name="firstName" label="Имя" />
        <FormField name="lastName" label="Фамилия" />
        <FormField name="specialization" label="Специализация" type="select" />
        <FormField name="declaredGrade" label="Предполагаемый грейд" type="select" />
        <StackSelector name="stack" label="Технологический стек" />
        <FormField name="experienceYears" label="Опыт (лет)" type="number" />
        <FormField name="bio" label="О себе" type="textarea" />
        <ResumeUpload onUpload={handleResumeUpload} />
        <Button type="submit">Сохранить</Button>
      </form>
    </Form>
  );
}
```

### 3.4. Тестирование

**Страница**: `/candidate/testing`, `/candidate/testing/session/[id]`

**Компоненты**:
- `TestIntro` — введение, правила, кнопка старта
- `QuestionCard` — отображение вопроса
- `CodeEditor` — редактор кода (Monaco Editor) для coding-вопросов
- `Timer` — обратный отсчёт
- `ProgressBar` — прогресс-бар
- `ResultSummary` — результаты теста

**UX-флоу**:
1. Кандидат нажимает "Начать тест"
2. Показывается первый вопрос с таймером
3. Кандидат отвечает → следующий вопрос
4. После последнего вопроса → результаты
5. Если cooldown активен → показать дату следующей попытки

```tsx
// app/candidate/testing/session/[id]/page.tsx
export default function TestSessionPage({ params }: { params: { id: string } }) {
  const { session, currentQuestion, submitAnswer, isSubmitting } = useTestSession(params.id);

  if (session.status === 'COMPLETED') {
    return <ResultSummary session={session} />;
  }

  return (
    <div className="max-w-3xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <ProgressBar current={session.answeredCount} total={session.totalQuestions} />
        <Timer secondsRemaining={session.timeRemainingSeconds} />
      </div>
      
      <QuestionCard
        question={currentQuestion}
        onAnswer={(answer) => submitAnswer(currentQuestion.id, answer)}
        isSubmitting={isSubmitting}
      />
    </div>
  );
}
```

### 3.5. Приглашения

**Страница**: `/candidate/invitations`

**Компоненты**:
- `InvitationCard` — карточка приглашения
- `InvitationDetail` — детали с описанием и ЗП
- `StatusBadge` — статус (новое/принято/отклонено)

**UX**:
- Список с фильтрацией по статусу
- Кнопки "Принять" / "Отклонить"
- При принятии — показ контактов работодателя
- Уведомление о новом приглашении (badge на иконке)

### 3.6. Настройки

**Страница**: `/candidate/settings`

**Компоненты**:
- `PrivacySettings` — настройки приватности
- `FSPConnection` — связь с ФСП ID
- `NotificationSettings` — настройки уведомлений
- `ConsentForm` — согласие на обработку данных

## 4. State Management

```typescript
// stores/auth.store.ts
interface AuthState {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (data: RegisterData) => Promise<void>;
  logout: () => void;
  refreshAccessToken: () => Promise<void>;
}

// stores/candidate.store.ts
interface CandidateState {
  profile: CandidateProfile | null;
  category: CategoryInfo | null;
  invitations: Invitation[];
  isLoading: boolean;
  fetchProfile: () => Promise<void>;
  updateProfile: (data: UpdateProfileData) => Promise<void>;
  fetchInvitations: () => Promise<void>;
  acceptInvitation: (id: string) => Promise<void>;
  rejectInvitation: (id: string) => Promise<void>;
}

// stores/testing.store.ts
interface TestingState {
  session: TestSession | null;
  currentQuestion: Question | null;
  isSubmitting: boolean;
  startTest: (specialization: string, grade: string) => Promise<void>;
  submitAnswer: (questionId: string, answer: string) => Promise<void>;
  completeTest: () => Promise<TestResult>;
}
```

## 5. API Client

```typescript
// lib/api.ts
import axios from 'axios';

const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL,
  withCredentials: true,
});

// Interceptor для обновления токена
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;
      const { data } = await axios.post(
        `${process.env.NEXT_PUBLIC_API_URL}/api/auth/refresh`,
        {},
        { withCredentials: true }
      );
      originalRequest.headers.Authorization = `Bearer ${data.accessToken}`;
      return api(originalRequest);
    }
    return Promise.reject(error);
  }
);

export default api;
```

## 6. Чеклист валидации

### Навигация
- [ ] Все страницы доступны через sidebar
- [ ] Редирект неавторизованных пользователей на /login
- [ ] Редирект после логина на /candidate
- [ ] Mobile навигация работает

### Профиль
- [ ] Заполнение профиля работает
- [ ] Загрузка PDF работает
- [ ] Парсинг PDF извлекает данные
- [ ] Превью профиля корректно

### Тестирование
- [ ] Тест стартует
- [ ] Вопросы отображаются корректно
- [ ] Таймер работает
- [ ] Ответы отправляются
- [ ] Результаты отображаются
- [ ] Cooldown отображается

### Приглашения
- [ ] Список приглашений загружается
- [ ] Принятие/отклонение работает
- [ ] Контакты показываются после принятия

### Общее
- [ ] Responsive дизайн (мобильные)
- [ ] Loading states
- [ ] Error handling
- [ ] Toast уведомления

## 7. Промпт для агента

```
Ты — Frontend-разработчик, специализирующийся на React/Next.js.

ТВОЯ ЗАДАЧА: Реализовать личный кабинет кандидата для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/frontend-candidate.md — ТЗ на модуль (этот файл)
- docs/modules/backend-api.md — API эндпоинты

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать Next.js проект с App Router
2. Реализовать страницы: login, register, dashboard, profile, testing, invitations, settings
3. Создать компоненты: ProfileForm, QuestionCard, Timer, InvitationCard
4. Настроить Zustand stores: auth, candidate, testing
5. Настроить API клиент с interceptors
6. Реализовать responsive дизайн
7. Написать unit-тесты для ключевых компонентов

СТЕК:
- Next.js 14 (App Router)
- Ant Design 5
- Zustand
- React Hook Form + Zod
- Axios + React Query
- Monaco Editor (для кода)

ВАЛИДАЦИЯ:
- Все страницы рендерятся без ошибок
- Навигация работает
- API интеграция работает
- Responsive на мобильных

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Скриншоты страниц
- Результат тестов
```