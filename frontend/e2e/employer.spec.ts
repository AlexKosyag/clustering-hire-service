import { test, expect } from '@playwright/test';

const EMPLOYER_EMAIL = 'test@employer.com';
const EMPLOYER_PASSWORD = 'Test123!';

async function loginAsEmployer(page: any) {
  await page.goto('/login');
  await page.fill('input[type="email"]', EMPLOYER_EMAIL);
  await page.fill('input[type="password"]', EMPLOYER_PASSWORD);
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/\/employer/);
}

test.describe('Employer Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsEmployer(page);
  });

  test('should display employer dashboard', async ({ page }) => {
    await expect(page.locator('text=Дашборд')).toBeVisible();
  });

  test('should navigate to search', async ({ page }) => {
    await page.click('a[href="/employer/search"]');
    await expect(page).toHaveURL(/\/employer\/search/);
  });

  test('should navigate to vacancies', async ({ page }) => {
    await page.click('a[href="/employer/vacancies"]');
    await expect(page).toHaveURL(/\/employer\/vacancies/);
  });
});

test.describe('Employer Search V2', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsEmployer(page);
    await page.goto('/employer/search');
  });

  test('should display search page', async ({ page }) => {
    await expect(page.locator('text=Поиск кандидатов')).toBeVisible();
    await expect(page.locator('text=Описание потребности')).toBeVisible();
  });

  test('should have search form fields', async ({ page }) => {
    await expect(page.locator('text=Специализация')).toBeVisible();
    await expect(page.locator('text=Грейд')).toBeVisible();
    await expect(page.locator('text=Стек')).toBeVisible();
  });

  test('should have V2 search button', async ({ page }) => {
    await expect(page.locator('button:has-text("Подобрать (V2)")')).toBeVisible();
  });

  test('should have V1 and semantic mode buttons', async ({ page }) => {
    await expect(page.locator('button:has-text("V1 (legacy)")')).toBeVisible();
    await expect(page.locator('button:has-text("Семантика")')).toBeVisible();
  });

  test('should perform search and show results', async ({ page }) => {
    // Fill in search criteria
    await page.fill('textarea', 'Ищем backend разработчика на Node.js');

    // Click V2 search
    await page.click('button:has-text("Подобрать (V2)")');

    // Wait for results or empty state
    await expect(
      page.locator('text=Найдено').or(page.locator('text=Ничего не найдено')),
    ).toBeVisible({ timeout: 15000 });
  });

  test('should display ML query understanding if results found', async ({ page }) => {
    await page.fill('textarea', 'Senior backend developer with Go and Kubernetes');
    await page.click('button:has-text("Подобрать (V2)")');

    // Check for ML understanding display (if search returns results)
    const hasML = await page.locator('text=ML понял запрос').isVisible({ timeout: 15000 }).catch(() => false);
    const hasNoResults = await page.locator('text=Ничего не найдено').isVisible({ timeout: 15000 }).catch(() => false);
    expect(hasML || hasNoResults).toBeTruthy();
  });

  test('should show candidate cards in results', async ({ page }) => {
    await page.fill('textarea', 'Backend разработчик');
    await page.click('button:has-text("Подобрать (V2)")');

    // Wait for either cards or empty state
    await page.waitForTimeout(3000);

    const hasCards = await page.locator('text=Пригласить').first().isVisible().catch(() => false);
    const hasEmpty = await page.locator('text=Ничего не найдено').isVisible().catch(() => false);
    expect(hasCards || hasEmpty).toBeTruthy();
  });
});

test.describe('Employer Vacancies', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsEmployer(page);
    await page.goto('/employer/vacancies');
  });

  test('should display vacancies page', async ({ page }) => {
    await expect(page.locator('text=ваканс').first()).toBeVisible();
  });
});