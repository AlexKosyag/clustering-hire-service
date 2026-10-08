import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
  test('should show login page', async ({ page }) => {
    await page.goto('/login');
    await expect(page).toHaveTitle(/ФСП/);
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test('should show register page', async ({ page }) => {
    await page.goto('/register');
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
  });

  test('should redirect to login when not authenticated', async ({ page }) => {
    await page.goto('/candidate');
    await expect(page).toHaveURL(/\/login/);
  });

  test('should login with valid credentials', async ({ page }) => {
    await page.goto('/login');

    // Fill in demo credentials
    await page.fill('input[type="email"]', 'test@candidate.com');
    await page.fill('input[type="password"]', 'Test123!');

    // Click login button
    await page.click('button[type="submit"]');

    // Should redirect to candidate dashboard
    await expect(page).toHaveURL(/\/candidate/);
    await expect(page.locator('text=Привет')).toBeVisible();
  });

  test('should show error with invalid credentials', async ({ page }) => {
    await page.goto('/login');

    await page.fill('input[type="email"]', 'wrong@email.com');
    await page.fill('input[type="password"]', 'WrongPass123!');
    await page.click('button[type="submit"]');

    // Should show error message
    await expect(page.locator('.ant-message-error')).toBeVisible();
  });

  test('should login as employer', async ({ page }) => {
    await page.goto('/login');

    await page.fill('input[type="email"]', 'test@employer.com');
    await page.fill('input[type="password"]', 'Test123!');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL(/\/employer/);
  });
});