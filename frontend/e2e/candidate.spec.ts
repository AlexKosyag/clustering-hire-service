import { test, expect } from '@playwright/test';

const CANDIDATE_EMAIL = 'test@candidate.com';
const CANDIDATE_PASSWORD = 'Test123!';

async function loginAsCandidate(page: any) {
  await page.goto('/login');
  await page.fill('input[type="email"]', CANDIDATE_EMAIL);
  await page.fill('input[type="password"]', CANDIDATE_PASSWORD);
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/\/candidate/);
}

test.describe('Candidate Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsCandidate(page);
  });

  test('should display dashboard with welcome header', async ({ page }) => {
    await expect(page.locator('text=Привет')).toBeVisible();
    await expect(page.locator('text=Уровень')).toBeVisible();
    await expect(page.locator('text=дней стрик')).toBeVisible();
  });

  test('should display stats cards', async ({ page }) => {
    await expect(page.locator('text=Тестирование')).toBeVisible();
    await expect(page.locator('text=Приглашения')).toBeVisible();
    await expect(page.locator('text=Достижения ФСП')).toBeVisible();
    await expect(page.locator('text=Стек')).toBeVisible();
  });

  test('should display category card', async ({ page }) => {
    await expect(page.locator('text=Моя категория')).toBeVisible();
    await expect(page.locator('text=Специализация')).toBeVisible();
  });

  test('should display test CTA', async ({ page }) => {
    await expect(page.locator('text=Тестирование')).toBeVisible();
    await expect(page.locator('text=Пройти тестирование').or(page.locator('text=Тест пройден'))).toBeVisible();
  });

  test('should navigate to profile', async ({ page }) => {
    await page.click('a[href="/candidate/profile"]');
    await expect(page).toHaveURL(/\/candidate\/profile/);
    await expect(page.locator('text=Профиль')).toBeVisible();
  });

  test('should navigate to testing', async ({ page }) => {
    await page.click('a[href="/candidate/test"]');
    await expect(page).toHaveURL(/\/candidate\/test/);
  });

  test('should navigate to invitations', async ({ page }) => {
    await page.click('a[href="/candidate/invitations"]');
    await expect(page).toHaveURL(/\/candidate\/invitations/);
    await expect(page.locator('text=Приглашения')).toBeVisible();
  });
});

test.describe('Candidate Profile', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsCandidate(page);
    await page.goto('/candidate/profile');
  });

  test('should display profile page', async ({ page }) => {
    await expect(page.locator('text=Основная информация')).toBeVisible();
    await expect(page.locator('text=Грейд')).toBeVisible();
    await expect(page.locator('text=Стек технологий')).toBeVisible();
  });

  test('should display resume upload section', async ({ page }) => {
    await expect(page.locator('text=Резюме (PDF)')).toBeVisible();
  });

  test('should open edit modal', async ({ page }) => {
    await page.click('button:has-text("Редактировать")');
    await expect(page.locator('.ant-modal')).toBeVisible();
    await expect(page.locator('text=Имя')).toBeVisible();
  });

  test('should display privacy settings in edit modal', async ({ page }) => {
    await page.click('button:has-text("Редактировать")');
    await expect(page.locator('text=Настройки приватности')).toBeVisible();
  });

  test('should display preview mode', async ({ page }) => {
    await page.click('button:has-text("Превью")');
    await expect(page.locator('text=Как видит работодатель')).toBeVisible();
  });
});

test.describe('Candidate Testing', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsCandidate(page);
    await page.goto('/candidate/test');
  });

  test('should display testing intro or active test', async ({ page }) => {
    // Either showing intro (with specialization selector) or active test
    const hasIntro = await page.locator('text=Специализация').isVisible();
    const hasActiveTest = await page.locator('text=Задача').isVisible();
    expect(hasIntro || hasActiveTest).toBeTruthy();
  });

  test('should show test mode options if on intro', async ({ page }) => {
    const hasIntro = await page.locator('text=Режим теста').isVisible();
    if (hasIntro) {
      await expect(page.locator('text=V2 — Мультиформатный')).toBeVisible();
      await expect(page.locator('text=V1 — Классический')).toBeVisible();
    }
  });
});

test.describe('Candidate Invitations', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsCandidate(page);
    await page.goto('/candidate/invitations');
  });

  test('should display invitations page', async ({ page }) => {
    await expect(page.locator('text=Приглашения')).toBeVisible();
  });

  test('should show empty state or invitations list', async ({ page }) => {
    const hasEmpty = await page.locator('text=Приглашений пока нет').isVisible();
    const hasList = await page.locator('.ant-card').first().isVisible();
    expect(hasEmpty || hasList).toBeTruthy();
  });
});