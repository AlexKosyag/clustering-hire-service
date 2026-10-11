# Концепция: Опциональные доработки

## 1. Обзор

Документ описывает опциональные функции, дающие дополнительные +7% к оценке (критерий 6). Реализуются после завершения обязательного MVP.

## 2. Список опциональных функций

### 2.1. Самостоятельный отклик кандидата на вакансии

**Статус**: Желательно для MVP
**Оценка**: Входит в критерий 6

**Описание**: Кандидат, пока не получивший приглашений, может самостоятельно откликнуться на открытые вакансии.

**Статус в Go-порте**: не реализовано / вне объёма порта — endpoint'ов отклика нет, таблица `applications` удалена миграцией `backend/migrations/000002_remove_applications.up.sql`. Список вакансий (`GET /api/vacancies`) реализован: `backend/internal/modules/vacancies/handler.go`.

**Реализация**:
- Список вакансий для кандидата
- Кнопка "Откликнуться"
- Сопроводительное письмо
- Статусы откликов

**API**:
```http
GET /api/vacancies — список вакансий
POST /api/vacancies/{id}/apply — отклик
GET /api/applications — мои отклики
```

**UI**:
- Страница `/candidate/vacancies` — список вакансий
- `VacancyCard` — карточка вакансии
- `ApplicationModal` — модальное окно отклика

---

### 2.2. Публикация вакансий работодателем

**Статус**: Желательно для MVP
**Оценка**: Входит в критерий 6

**Описание**: Работодатель размещает вакансию напрямую с возможностью редактирования и работы с откликами.

**Статус в Go-порте**: частично реализовано — CRUD вакансий реализован в `backend/internal/modules/vacancies/handler.go` (`POST/PUT/DELETE /api/vacancies`); просмотр откликов (`GET /api/vacancies/{id}/applications`) не реализован / вне объёма порта (таблица `applications` удалена миграцией `backend/migrations/000002_remove_applications.up.sql`).

**Реализация**:
- CRUD для вакансий
- Модерация (опционально)
- Статистика просмотров

**API**:
```http
POST /api/vacancies — создать
PUT /api/vacancies/{id} — редактировать
DELETE /api/vacancies/{id} — удалить
GET /api/vacancies/{id}/applications — отклики
```

---

### 2.3. Регулярные короткие задания от работодателей

**Статус**: Желательно для MVP
**Оценка**: Входит в критерий 6

**Описание**: Система периодически предлагает кандидату задачу от работодателя.

**Статус в Go-порте**: не реализовано / вне объёма порта — есть только модель `daily_tasks` в БД (`backend/migrations/000001_init.up.sql`, `backend/internal/model/model.go`), сервисов и эндпоинтов нет.

**Реализация**:
- Пул заданий по специализациям
- Ежедневная генерация
- Оценка ответов
- Влияние на профиль

**Уже описано в**: `docs/modules/testing-engine.md` (раздел 7)

---

### 2.4. Защита от фиктивных вакансий и недобросовестных работодателей

**Статус**: За рамками MVP (концепция)
**Оценка**: Упоминается в критерии 6

**Описание**: Механизм защиты от фиктивных вакансий и мошеннических работодателей.

**Статус в Go-порте**: не реализовано / вне объёма порта (концепция).

**Концепция**:

#### Верификация компании
```go
// Концепция (не реализовано)
type CompanyVerification struct {
	Status          string                 `json:"status"` // unverified | pending | verified | rejected
	Documents       []VerificationDocument `json:"documents"`
	VerifiedAt      *string                `json:"verifiedAt,omitempty"`
	RejectionReason *string                `json:"rejectionReason,omitempty"`
}

type VerificationDocument struct {
	Type   string `json:"type"`   // inn | ogrn | certificate | other
	URL    string `json:"url"`
	Status string `json:"status"` // pending | approved | rejected
}
```

#### Детекция фиктивных вакансий
```go
// Концепция (не реализовано)
type VacancyFraudDetector struct{}

// Подозрительные признаки:
// - CheckSalaryRange(vacancy)   — слишком высокая ЗП
// - CheckDescription(vacancy)   — подозрительное описание
// - CheckCompanyHistory(employer) — новая компания
// - CheckDuplicateVacancies(vacancy) — дубликаты
// - ClassifyVacancy(vacancy) (ML, future) — legit | suspicious | fake
```

#### Жалобы и модерация
```go
// Концепция (не реализовано)
type Complaint struct {
	ID          string `json:"id"`
	ReporterID  string `json:"reporterId"`
	TargetID    string `json:"targetId"`
	TargetType  string `json:"targetType"` // vacancy | employer
	Reason      string `json:"reason"`     // fake | spam | inappropriate | other
	Description string `json:"description"`
	Status      string `json:"status"`     // pending | reviewed | resolved
}
```

---

### 2.5. Интеграция с ATS-системами работодателей

**Статус**: За рамками MVP (концепция)
**Оценка**: Упоминается в критерии 6

**Описание**: Интеграция с внутренними ATS (Applicant Tracking System) работодателей.

**Статус в Go-порте**: не реализовано / вне объёма порта (концепция).

**Концепция**:

#### Поддерживаемые ATS
- Greenhouse
- Lever
- Workday
- CustomButton (webhook-based)

#### Интеграционный слой
```go
// Концепция (не реализовано)
type ATSIntegration interface {
	// Синхронизация вакансий
	SyncVacancies(ctx context.Context) ([]Vacancy, error)
	// Отправка кандидатов
	SendCandidate(ctx context.Context, candidate Candidate, vacancyId string) error
	// Получение статусов
	GetApplicationStatus(ctx context.Context, applicationId string) (ApplicationStatus, error)
}

type ATSConfig struct {
	APIKey       string `json:"apiKey"`
	WebhookURL   string `json:"webhookUrl,omitempty"`
	SyncInterval int    `json:"syncInterval,omitempty"` // минуты
}
```

#### Webhook для получения обновлений
```http
POST /api/integrations/ats/webhook
Content-Type: application/json

{
  "event": "application_status_changed",
  "applicationId": "ext-123",
  "status": "interview",
  "timestamp": "2026-01-10T12:00:00Z"
}
```

---

### 2.6. Адаптивная версия для мобильных устройств

**Статус**: Опционально для MVP
**Оценка**: Упоминается в критерии 6

**Описание**: Responsive дизайн для мобильных устройств и планшетов.

**Реализация**:
- CSS media queries
- Mobile-first подход
- Touch-friendly UI
- PWA (Progressive Web App)

**Ключевые страницы для мобильной адаптации**:
1. Dashboard — карточки в столбец
2. Тестирование — полноэкранный режим
3. Приглашения — swipe-жесты
4. Поиск — bottom sheet фильтры

---

### 2.7. Semantic Search для кандидатов

**Статус**: Опционально для MVP
**Оценка**: Техническое преимущество

**Описание**: Векторный поиск кандидатов по естественному языку.

**Статус в Go-порте**: частично — семантические эндпоинты есть (`POST /api/matching/semantic`, `POST /api/matching/v2/semantic` — `backend/internal/modules/matching/handler.go`), эмбеддинги может отдавать ml-service (Python, без изменений). В самой Go-сборке JS-пакет `@xenova/transformers` недоступен (нет ONNX-рантайма) — локальный embedding-провайдер деградирует до TF-IDF (`backend/internal/modules/matching/embedding.go`).

**Реализация**:
- pgvector для хранения и поиска векторов — `backend/internal/store/vector.go` (vector(384), cosine distance)
- Cosine similarity для ранжирования
- TF-IDF fallback — `backend/internal/modules/matching/tfidf.go`

**Уже описано в**: `docs/modules/matching-engine.md`

---

### 2.8. PDF парсинг резюме

**Статус**: Опционально для MVP
**Оценка**: Удобство для кандидата

**Описание**: Автоматическое распознавание PDF-резюме и заполнение профиля.

**Статус в Go-порте**: реализовано — `backend/internal/modules/candidates/pdf.go`, эндпоинт `POST /api/candidates/profile/resume` (`backend/internal/modules/candidates/handler.go`).

**Реализация**:
- github.com/ledongthuc/pdf для извлечения текста
- Regex-парсинг полей (SKILLS, опыт, образование, «о себе») — дословный перенос логики
- OpenAI API для структурирования — не используется (вне объёма порта)

```go
// ParsedResume — backend/internal/modules/candidates/pdf.go
type ParsedResume struct {
	Name       *string           `json:"name,omitempty"`
	Email      *string           `json:"email,omitempty"`
	Phone      *string           `json:"phone,omitempty"`
	City       *string           `json:"city,omitempty"`
	Skills     []string          `json:"skills"`
	Experience []ExperienceEntry `json:"experience"`
	Education  []EducationEntry  `json:"education"`
	Summary    *string           `json:"summary,omitempty"`
	RawText    string            `json:"rawText"`
}
```

---

### 2.9. Автогенерация PDF-профиля

**Статус**: Опционально для MVP
**Оценка**: Удобство для кандидата

**Описание**: Генерация стандартизированного PDF-профиля из данных платформы.

**Статус в Go-порте**: не реализовано / вне объёма порта — библиотек генерации PDF в бэкенде нет (только извлечение текста, см. раздел 2.8).

**Реализация**:
- Библиотека генерации PDF на Go (где выбрать)
- Шаблон в стиле ФСП
- Включение достижений

---

### 2.10. Уведомления (Email + In-app)

**Статус**: Опционально для MVP
**Оценка**: UX улучшение

**Описание**: Система уведомлений о приглашениях, заданиях, обновлениях.

**Статус в Go-порте**: частично — in-app read-model реализован в `backend/internal/modules/notifications/` (`GET /api/notifications`, таблица отсутствует, `read` выводится по правилам — BUG-REF: G7). WebSocket-канал, email-уведомления (SMTP), настройки частоты и `POST /api/notifications/read` — не реализованы / вне объёма порта.

**Реализация**:
- In-app уведомления — read-model без таблицы (`backend/internal/modules/notifications/service.go`)
- Email уведомления — не реализованы / вне объёма порта
- Настройки частоты — не реализованы / вне объёма порта

```go
// NotificationItem — backend/internal/modules/notifications/dto.go
type NotificationItem struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"` // INVITATION_RECEIVED | INVITATION_STATUS | TEST_RESULT
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedAt model.Time `json:"createdAt"`
	Read      bool       `json:"read"`
}
```

## 3. Приоритеты реализации

| Функция | Приоритет | Сложность | Оценка |
|---------|-----------|-----------|--------|
| Отклик на вакансии | Высокий | Низкая | +2% |
| Публикация вакансий | Высокий | Низкая | +2% |
| Daily tasks | Высокий | Средняя | +1% |
| PDF парсинг | Средний | Средняя | +1% |
| PDF генерация | Средний | Низкая | +0.5% |
| Мобильная адаптация | Средний | Средняя | +0.5% |
| Защита от мошенничества | Низкий | Высокая | +0.5% |
| Интеграция с ATS | Низкий | Высокая | +0.5% |

## 4. Чеклист валидации

### Отклик на вакансии
- [ ] Список вакансий отображается
- [ ] Отклик отправляется
- [ ] Статус отслеживается

### Публикация вакансий
- [ ] Создание работает
- [ ] Редактирование работает
- [ ] Отклики отображаются

### PDF
- [ ] Парсинг извлекает данные
- [ ] Генерация создаёт PDF

### Мобильная адаптация
- [ ] Dashboard отображается на мобильном
- [ ] Тестирование работает на мобильном
- [ ] Touch-жесты работают

## 5. Промпт для агента

```
Ты — Full-stack разработчик.

ТВОЯ ЗАДАЧА: Реализовать опциональные функции для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/concepts/optional-features.md — ТЗ (этот файл)
- docs/modules/database.md — схема данных
- docs/modules/backend-api.md — API

ЧТО НУЖНО СДЕЛАТЬ (по приоритету):
1. Отклик на вакансии (candidate side)
2. Публикация вакансий (employer side)
3. PDF парсинг резюме
4. PDF генерация профиля
5. Мобильная адаптация (responsive)

СТЕК:
- Go 1.25, gin, pgx/v5 (backend)
- Next.js 14 (frontend)
- github.com/ledongthuc/pdf (парсинг PDF-резюме)

ВАЛИДАЦИЯ:
- Отклик работает
- PDF парсится корректно
- PDF генерируется
- Мобильная версия работает

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Примеры PDF
- Результат тестов
```