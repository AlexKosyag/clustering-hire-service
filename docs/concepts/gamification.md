# Концепция: Gamification (Геймификация)

## 1. Обзор

Геймификация — опциональный модуль, повышающий вовлечённость кандидатов и работодателей. Не является обязательным для MVP, но даёт +7% к оценке (критерий 6).

**Статус**: Концепция (реализация опциональна)
**Зависимости**: database, backend-api, testing-engine

## 2. Элементы геймификации

### 2.1. Бейджи и достижения

**Цель**: Визуализировать прогресс и достижения кандидата.

```typescript
// Система бейджей
interface Badge {
  id: string;
  name: string;           // "Первый тест"
  description: string;    // "Прошёл первое тестирование"
  icon: string;           // Emoji или URL иконки
  category: 'testing' | 'profile' | 'activity' | 'fsp';
  rarity: 'common' | 'rare' | 'epic' | 'legendary';
  condition: BadgeCondition;
}

interface BadgeCondition {
  type: 'test_completed' | 'grade_achieved' | 'profile_complete' | 
        'streak' | 'invitations_received' | 'fsp_connected';
  value?: number;         // Пороговое значение
}
```

**Примеры бейджей**:

| Бейдж | Условие | Редкость |
|-------|---------|----------|
| Первый тест | Завершить первое тестирование | Common |
| Junior | Подтвердить грейд Junior | Common |
| Middle | Подтвердить грейд Middle | Rare |
| Senior | Подтвердить грейд Senior | Epic |
| Full Stack | Заполнить профиль на 100% | Rare |
| Олимпиец | Подключить ФСП ID | Rare |
| Финалист | Получить достижение "Финалист" | Epic |
| Стратег | 7 дней подряд выполнять задания | Epic |
| Легенда | Получить 10+ приглашений | Legendary |

### 2.2. Уровни и опыт (XP)

**Цель**: Мотивировать кандидатов к регулярной активности.

```typescript
interface Level {
  level: number;
  name: string;          // "Новичок", "Ученик", "Мастер", "Легенда"
  xpRequired: number;    // XP для достижения уровня
  perks: string[];       // Бонусы уровня
}

// XP начисляется за:
const XP_REWARDS = {
  TEST_COMPLETED: 100,      // Завершение теста
  TEST_PASSED: 200,         // Успешное прохождение
  GRADE_UPGRADED: 300,      // Повышение грейда
  PROFILE_COMPLETE: 50,     // Заполненный профиль
  DAILY_TASK_DONE: 25,      // Ежедневное задание
  STREAK_3_DAYS: 50,        // 3 дня подряд
  STREAK_7_DAYS: 150,       // 7 дней подряд
  INVITATION_RECEIVED: 10,  // Получено приглашение
  FSP_CONNECTED: 100,       // Подключён ФСП
};
```

### 2.3. Стрики (серии активности)

**Цель**: Мотивировать к регулярной активности.

```typescript
interface Streak {
  current: number;       // Текущая серия дней
  longest: number;       // Лучшая серия
  lastActiveDate: string;
  multiplier: number;    // Множитель XP (1x → 1.5x → 2x)
}

// Логика стрика:
// - Кандидат выполняет задание или проходит тест → стрик +1
// - Пропуск дня → стрик сбрасывается
// - Каждые 7 дней → множитель увеличивается
```

### 2.4. Рейтинг кандидатов (Leaderboard)

**Цель**: Создать здоровую конкуренцию.

```typescript
interface Leaderboard {
  type: 'global' | 'specialization' | 'monthly';
  entries: LeaderboardEntry[];
}

interface LeaderboardEntry {
  rank: number;
  candidateId: string;
  displayName: string;   // "Иван И."
  xp: number;
  level: number;
  badges: Badge[];
}

// Рейтинги:
// 1. Общий рейтинг по XP
// 2. Рейтинг по специализации (Backend, Frontend, etc.)
// 3. Месячный рейтинг (сброс каждый месяц)
```

### 2.5. Прогресс-бары

**Цель**: Визуализировать прогресс.

```typescript
// Прогресс заполнения профиля
interface ProfileProgress {
  total: number;         // 100%
  filled: number;        // Заполнено %
  sections: {
    basic: boolean;      // ФИО, контакты
    professional: boolean; // Стек, грейд, опыт
    testing: boolean;    // Тест пройден
    fsp: boolean;        // ФСП подключён
    resume: boolean;     // Резюме загружено
  };
}
```

## 3. UI компоненты

### 3.1. Badge Display

```tsx
// components/gamification/BadgeDisplay.tsx
export function BadgeDisplay({ badge }: { badge: Badge }) {
  const rarityColor = {
    common: 'bg-gray-100 text-gray-700',
    rare: 'bg-blue-100 text-blue-700',
    epic: 'bg-purple-100 text-purple-700',
    legendary: 'bg-yellow-100 text-yellow-700',
  }[badge.rarity];

  return (
    <div className={`flex items-center gap-2 p-2 rounded ${rarityColor}`}>
      <span className="text-2xl">{badge.icon}</span>
      <div>
        <p className="font-medium">{badge.name}</p>
        <p className="text-xs opacity-70">{badge.description}</p>
      </div>
    </div>
  );
}
```

### 3.2. Level Progress

```tsx
// components/gamification/LevelProgress.tsx
export function LevelProgress({ level, xp, xpToNext }: Props) {
  const progress = (xp / xpToNext) * 100;

  return (
    <div className="flex items-center gap-4">
      <div className="w-12 h-12 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold">
        {level}
      </div>
      <div className="flex-1">
        <div className="flex justify-between text-sm mb-1">
          <span>Уровень {level}</span>
          <span>{xp}/{xpToNext} XP</span>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-2">
          <div
            className="bg-blue-500 h-2 rounded-full transition-all"
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>
    </div>
  );
}
```

### 3.3. Streak Counter

```tsx
// components/gamification/StreakCounter.tsx
export function StreakCounter({ streak }: { streak: Streak }) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-2xl">🔥</span>
      <div>
        <p className="font-bold text-lg">{streak.current} дней</p>
        <p className="text-xs text-gray-500">Серия активности</p>
      </div>
      {streak.multiplier > 1 && (
        <span className="ml-2 px-2 py-1 bg-yellow-100 text-yellow-700 rounded text-sm">
          x{streak.multiplier} XP
        </span>
      )}
    </div>
  );
}
```

## 4. API Endpoints

### 4.1. Получить бейджи

```http
GET /api/gamification/badges
Authorization: Bearer {token}

Response 200:
{
  "badges": [
    {
      "id": "badge-1",
      "name": "Первый тест",
      "description": "Прошёл первое тестирование",
      "icon": "🎯",
      "category": "testing",
      "rarity": "common",
      "earnedAt": "2026-01-10T12:00:00Z"
    }
  ],
  "totalBadges": 5,
  "availableBadges": 15
}
```

### 4.2. Получить уровень и XP

```http
GET /api/gamification/level
Authorization: Bearer {token}

Response 200:
{
  "level": 5,
  "name": "Ученик",
  "xp": 450,
  "xpToNextLevel": 500,
  "xpHistory": [
    { "action": "TEST_PASSED", "xp": 200, "date": "2026-01-10" },
    { "action": "DAILY_TASK_DONE", "xp": 25, "date": "2026-01-11" }
  ]
}
```

### 4.3. Получить стрик

```http
GET /api/gamification/streak
Authorization: Bearer {token}

Response 200:
{
  "current": 5,
  "longest": 12,
  "lastActiveDate": "2026-01-15",
  "multiplier": 1.0,
  "nextMultiplierAt": 7
}
```

### 4.4. Рейтинг

```http
GET /api/gamification/leaderboard?type=specialization&spec=BACKEND&limit=20

Response 200:
{
  "type": "specialization",
  "specialization": "BACKEND",
  "entries": [
    {
      "rank": 1,
      "displayName": "Дмитрий С.",
      "xp": 2500,
      "level": 8,
      "badges": ["🏆", "⭐", "🔥"]
    }
  ],
  "myRank": 15
}
```

## 5. Интеграция с существующими модулями

### 5.1. Testing Engine → Gamification

```typescript
// После завершения теста:
async function onTestCompleted(session: TestSession) {
  // Начислить XP
  await gamificationService.awardXP(session.candidateId, XP_REWARDS.TEST_COMPLETED);
  
  if (session.recommendation === 'PASS') {
    await gamificationService.awardXP(session.candidateId, XP_REWARDS.TEST_PASSED);
  }
  
  if (session.recommendation === 'UPGRADE') {
    await gamificationService.awardXP(session.candidateId, XP_REWARDS.GRADE_UPGRADED);
  }
  
  // Проверить бейджи
  await gamificationService.checkBadges(session.candidateId, 'test_completed');
}
```

### 5.2. Daily Tasks → Gamification

```typescript
// После выполнения задания:
async function onDailyTaskCompleted(task: DailyTask) {
  await gamificationService.awardXP(task.candidateId, XP_REWARDS.DAILY_TASK_DONE);
  await gamificationService.updateStreak(task.candidateId);
}
```

### 5.3. Invitations → Gamification

```typescript
// После получения приглашения:
async function onInvitationReceived(invitation: Invitation) {
  await gamificationService.awardXP(invitation.candidateId, XP_REWARDS.INVITATION_RECEIVED);
  await gamificationService.checkBadges(invitation.candidateId, 'invitation_received');
}
```

## 6. Чеклист валидации

- [ ] Бейджи начисляются при выполнении условий
- [ ] XP начисляется за все действия
- [ ] Уровни рассчитываются корректно
- [ ] Стрики отслеживаются
- [ ] Множитель работает
- [ ] Leaderboard отображается
- [ ] UI компоненты рендерятся

## 7. Промпт для агента

```
Ты — Full-stack разработчик с опытом в геймификации.

ТВОЯ ЗАДАЧА: Реализовать модуль gamification для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/concepts/gamification.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/src/modules/gamification/
2. Реализовать GamificationService — XP, levels, badges, streaks
3. Создать API endpoints
4. Создать UI компоненты: BadgeDisplay, LevelProgress, StreakCounter, Leaderboard
5. Интегрировать с testing и daily tasks
6. Написать unit-тесты

СТЕК:
- NestJS 10 (backend)
- React + Ant Design (frontend)

ВАЛИДАЦИЯ:
- Бейджи начисляются
- XP рассчитывается
- UI отображается корректно

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Примеры бейджей
- Результат тестов
```