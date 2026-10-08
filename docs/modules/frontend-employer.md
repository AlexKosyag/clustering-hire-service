# Модуль: Frontend — Личный кабинет работодателя

## 1. Назначение

Веб-приложение для работодателей. Отвечает за:
- Профиль компании
- Описание потребности в специалистах
- Просмотр подборки кандидатов
- Поиск по базе кандидатов
- Отправка приглашений
- Публикация вакансий

**Зависимости**: auth, backend-api, matching-engine
**Параллельно с**: frontend-candidate

## 2. Архитектура

```
src/app/employer/
├── layout.tsx               # Layout с sidebar
├── page.tsx                 # Dashboard
├── profile/
│   ├── page.tsx             # Профиль компании
│   └── edit/page.tsx        # Редактирование
├── search/
│   ├── page.tsx             # Поиск кандидатов
│   └── results/page.tsx     # Результаты поиска
├── candidates/
│   ├── page.tsx             # Подборка кандидатов
│   └── [id]/page.tsx        # Профиль кандидата
├── invitations/
│   ├── page.tsx             # Отправленные приглашения
│   └── new/page.tsx         # Новое приглашение
├── vacancies/
│   ├── page.tsx             # Список вакансий
│   ├── new/page.tsx         # Создать вакансию
│   └── [id]/
│       ├── page.tsx         # Просмотр вакансии
│       └── edit/page.tsx    # Редактирование
└── components/
    ├── layout/
    │   ├── Sidebar.tsx
    │   └── Header.tsx
    ├── profile/
    │   ├── CompanyCard.tsx
    │   └── CompanyForm.tsx
    ├── search/
    │   ├── SearchBar.tsx
    │   ├── Filters.tsx
    │   ├── CandidateCard.tsx
    │   ├── CandidateList.tsx
    │   └── MatchExplanation.tsx
    ├── candidates/
    │   ├── CandidateProfile.tsx
    │   ├── CandidateDetail.tsx
    │   └── AchievementBadge.tsx
    ├── invitations/
    │   ├── InvitationForm.tsx
    │   ├── InvitationCard.tsx
    │   └── StatusTracker.tsx
    └── vacancies/
        ├── VacancyForm.tsx
        ├── VacancyCard.tsx
        └── ApplicationList.tsx
```

## 3. Страницы и компоненты

### 3.1. Dashboard

**Страница**: `/employer`

**Компоненты**:
- `StatsOverview` — статистика (приглашения, отклики, просмотры)
- `RecentInvitations` — последние приглашения с статусами
- `TopCandidates` — топ-5 рекомендованных кандидатов
- `ActiveVacancies` — активные вакансии

### 3.2. Профиль компании

**Страница**: `/employer/profile`

**Компоненты**:
- `CompanyCard` — карточка компании
- `CompanyForm` — форма редактирования
- `LogoUpload` — загрузка логотипа

### 3.3. Поиск кандидатов (ключевая страница)

**Страница**: `/employer/search`

**Компоненты**:
- `SearchBar` — строка поиска с подсказками
- `Filters` — фильтры (специализация, грейд, стек, достижения)
- `CandidateCard` — карточка кандидата в выдаче
- `CandidateList` — список кандидатов с пагинацией
- `MatchExplanation` — объяснение почему кандидат подходит

**UX-флоу**:
1. Работодатель вводит описание потребности (текст)
2. Система анализирует и предлагает фильтры
3. Работодатель уточняет запрос
4. Система показывает подборку с объяснением
5. Работодатель может отправить приглашение

```tsx
// app/employer/search/page.tsx
export default function SearchPage() {
  const [query, setQuery] = useState('');
  const [filters, setFilters] = useState<SearchFilters>({});
  const { results, isLoading, search } = useCandidateSearch();

  return (
    <div className="flex gap-6">
      <aside className="w-64">
        <Filters filters={filters} onChange={setFilters} />
      </aside>
      <main className="flex-1">
        <SearchBar
          value={query}
          onChange={setQuery}
          onSearch={() => search(query, filters)}
          placeholder="Опишите, какого специалиста вы ищете..."
        />
        {isLoading ? (
          <Skeleton />
        ) : (
          <CandidateList candidates={results}>
            {(candidate) => (
              <CandidateCard
                key={candidate.id}
                candidate={candidate}
                onInvite={() => openInviteModal(candidate.id)}
              >
                <MatchExplanation explanation={candidate.matchExplanation} />
              </CandidateCard>
            )}
          </CandidateList>
        )}
      </main>
    </div>
  );
}
```

### 3.4. Профиль кандидата (вид работодателя)

**Страница**: `/employer/candidates/[id]`

**Компоненты**:
- `CandidateProfile` — профиль (без контактов до ACCEPTED)
- `AchievementBadge` — достижения ФСП
- `MatchExplanation` — объяснение подборки
- `InviteButton` — кнопка приглашения

**Ключевое**: Контактные данные скрыты до принятия приглашения!

```tsx
// components/candidates/CandidateProfile.tsx
export function CandidateProfile({ candidate, isInvited }: Props) {
  return (
    <div>
      <h2>{candidate.displayName}</h2>  {/* "Иван И." — без фамилии */}
      <p>{candidate.specialization} · {candidate.verifiedGrade}</p>
      <StackTags stack={candidate.stack} />
      <AchievementBadges achievements={candidate.achievements} />
      
      {/* Контакты скрыты */}
      {!isInvited && (
        <div className="bg-gray-100 p-4 rounded">
          <p>Контактные данные будут доступны после принятия приглашения</p>
        </div>
      )}
      
      {isInvited && (
        <div>
          <p>Email: {candidate.email}</p>
          <p>Телефон: {candidate.phone}</p>
        </div>
      )}
    </div>
  );
}
```

### 3.5. Приглашения

**Страница**: `/employer/invitations`

**Компоненты**:
- `InvitationCard` — карточка приглашения
- `InvitationForm` — форма нового приглашения
- `StatusTracker` — трекер статуса (отправлено → просмотрено → принято/отклонено)

**Форма приглашения**:
```tsx
// components/invitations/InvitationForm.tsx
export function InvitationForm({ candidateId, onClose }: Props) {
  const form = useForm<InvitationFormData>({
    resolver: zodResolver(invitationSchema),
  });

  return (
    <Modal title="Приглашение кандидату" onClose={onClose}>
      <Form {...form}>
        <FormField name="message" label="Сообщение" type="textarea" />
        <div className="flex gap-4">
          <FormField name="salaryFrom" label="ЗП от" type="number" />
          <FormField name="salaryTo" label="ЗП до" type="number" />
        </div>
        <FormField name="vacancyId" label="Привязать к вакансии" type="select" optional />
        <Button type="submit">Отправить приглашение</Button>
      </Form>
    </Modal>
  );
}
```

### 3.6. Вакансии

**Страница**: `/employer/vacancies`, `/employer/vacancies/new`

**Компоненты**:
- `VacancyForm` — форма создания/редактирования
- `VacancyCard` — карточка вакансии
- `ApplicationList` — список откликов

## 4. State Management

```typescript
// stores/employer.store.ts
interface EmployerState {
  profile: EmployerProfile | null;
  candidates: Candidate[];
  invitations: Invitation[];
  vacancies: Vacancy[];
  isLoading: boolean;
  
  fetchProfile: () => Promise<void>;
  updateProfile: (data: UpdateProfileData) => Promise<void>;
  
  searchCandidates: (query: string, filters: SearchFilters) => Promise<void>;
  fetchCandidateDetail: (id: string) => Promise<CandidateDetail>;
  
  sendInvitation: (data: InvitationData) => Promise<void>;
  fetchInvitations: () => Promise<void>;
  
  createVacancy: (data: VacancyData) => Promise<void>;
  fetchVacancies: () => Promise<void>;
}
```

## 5. Чеклист валидации

### Профиль
- [ ] Создание профиля компании работает
- [ ] Загрузка логотипа работает
- [ ] Редактирование сохраняет данные

### Поиск и подбор
- [ ] Semantic search работает
- [ ] Фильтры работают
- [ ] Подборка кандидатов отображается
- [ ] Match explanation понятен и корректен
- [ ] Пагинация работает

### Приглашения
- [ ] Отправка приглашения работает
- [ ] Статусы обновляются
- [ ] Контакты скрыты до принятия
- [ ] ЗП обязательна в приглашении

### Вакансии
- [ ] Создание вакансии работает
- [ ] Список вакансий отображается
- [ ] Отклики отображаются

### Общее
- [ ] Responsive дизайн
- [ ] Loading states
- [ ] Error handling
- [ ] Навигация работает

## 6. Промпт для агента

```
Ты — Frontend-разработчик, специализирующийся на React/Next.js.

ТВОЯ ЗАДАЧА: Реализовать личный кабинет работодателя для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/frontend-employer.md — ТЗ на модуль (этот файл)
- docs/modules/backend-api.md — API эндпоинты

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать страницы: dashboard, profile, search, candidates, invitations, vacancies
2. Создать компоненты: SearchBar, Filters, CandidateCard, MatchExplanation, InvitationForm
3. Настроить Zustand store: employer
4. Реализовать semantic search UI
5. Реализовать responsive дизайн
6. Написать unit-тесты

СТЕК:
- Next.js 14 (App Router)
- Ant Design 5
- Zustand
- React Hook Form + Zod

ВАЛИДАЦИЯ:
- Все страницы рендерятся
- Поиск работает
- Приглашения отправляются
- Контакты скрыты до принятия

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Скриншоты страниц
- Результат тестов
```