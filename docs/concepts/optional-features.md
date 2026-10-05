# Концепция: Опциональные доработки

## 1. Обзор

Документ описывает опциональные функции, дающие дополнительные +7% к оценке (критерий 6). Реализуются после завершения обязательного MVP.

## 2. Список опциональных функций

### 2.1. Самостоятельный отклик кандидата на вакансии

**Статус**: Желательно для MVP
**Оценка**: Входит в критерий 6

**Описание**: Кандидат, пока не получивший приглашений, может самостоятельно откликнуться на открытые вакансии.

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

**Концепция**:

#### Верификация компании
```typescript
interface CompanyVerification {
  status: 'unverified' | 'pending' | 'verified' | 'rejected';
  documents: VerificationDocument[];
  verifiedAt?: Date;
  rejectionReason?: string;
}

interface VerificationDocument {
  type: 'inn' | 'ogrn' | 'certificate' | 'other';
  url: string;
  status: 'pending' | 'approved' | 'rejected';
}
```

#### Детекция фиктивных вакансий
```typescript
interface VacancyFraudDetector {
  // Подозрительные признаки
  checkSalaryRange(vacancy: Vacancy): boolean;  // Слишком высокая ЗП
  checkDescription(vacancy: Vacancy): boolean;  // Подозрительное описание
  checkCompanyHistory(employer: Employer): boolean; // Новая компания
  checkDuplicateVacancies(vacancy: Vacancy): boolean; // Дубликаты
  
  // ML-based классификация (future)
  classifyVacancy(vacancy: Vacancy): 'legit' | 'suspicious' | 'fake';
}
```

#### Жалобы и модерация
```typescript
interface Complaint {
  id: string;
  reporterId: string;
  targetId: string;
  targetType: 'vacancy' | 'employer';
  reason: 'fake' | 'spam' | 'inappropriate' | 'other';
  description: string;
  status: 'pending' | 'reviewed' | 'resolved';
}
```

---

### 2.5. Интеграция с ATS-системами работодателей

**Статус**: За рамками MVP (концепция)
**Оценка**: Упоминается в критерии 6

**Описание**: Интеграция с внутренними ATS (Applicant Tracking System) работодателей.

**Концепция**:

#### Поддерживаемые ATS
- Greenhouse
- Lever
- Workday
-.CustomButton (webhook-based)

#### Интеграционный слой
```typescript
interface ATSIntegration {
  provider: string;
  config: ATSConfig;
  
  // Синхронизация вакансий
  syncVacancies(): Promise<Vacancy[]>;
  
  // Отправка кандидатов
  sendCandidate(candidate: Candidate, vacancyId: string): Promise<void>;
  
  // Получение статусов
  getApplicationStatus(applicationId: string): Promise<ApplicationStatus>;
}

interface ATSConfig {
  apiKey: string;
  webhookUrl?: string;
  syncInterval?: number; // минуты
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

**Реализация**:
- @xenova/transformers для генерации embeddings
- pgvector для хранения и поиска векторов
- Cosine similarity для ранжирования

**Уже описано в**: `docs/modules/matching-engine.md`

---

### 2.8. PDF парсинг резюме

**Статус**: Опционально для MVP
**Оценка**: Удобство для кандидата

**Описание**: Автоматическое распознавание PDF-резюме и заполнение профиля.

**Реализация**:
- pdf-parse для извлечения текста
- Regex + NLP для парсинга полей
- OpenAI API для структурирования (опционально)

```typescript
interface ParsedResume {
  name?: string;
  email?: string;
  phone?: string;
  skills?: string[];
  experience?: ExperienceEntry[];
  education?: EducationEntry[];
  summary?: string;
}
```

---

### 2.9. Автогенерация PDF-профиля

**Статус**: Опционально для MVP
**Оценка**: Удобство для кандидата

**Описание**: Генерация стандартизированного PDF-профиля из данных платформы.

**Реализация**:
- PDFKit или puppeteer
- Шаблон в стиле ФСП
- Включение достижений

---

### 2.10. Уведомления (Email + In-app)

**Статус**: Опционально для MVP
**Оценка**: UX улучшение

**Описание**: Система уведомлений о приглашениях, заданиях, обновлениях.

**Реализация**:
- In-app уведомления (WebSocket)
- Email уведомления (Nodemailer)
- Настройки частоты

```typescript
interface Notification {
  id: string;
  userId: string;
  type: 'invitation' | 'daily_task' | 'test_reminder' | 'achievement';
  title: string;
  message: string;
  read: boolean;
  createdAt: Date;
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
- NestJS 10 (backend)
- Next.js 14 (frontend)
- pdf-parse (парсинг)
- PDFKit (генерация)

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