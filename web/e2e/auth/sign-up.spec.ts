import { test, expect } from '@playwright/test';

test.describe('Auth/signUp', () => {
  test('renders the sign-up form', async ({ page }) => {
    await page.goto('/signup');

    await expect(page.getByRole('heading', { name: '注册账号' })).toBeVisible();
    await expect(page.getByLabel('登录 ID')).toBeVisible();
    await expect(page.getByLabel('昵称')).toBeVisible();
    await expect(page.getByLabel('密码')).toBeVisible();
    await expect(page.getByRole('button', { name: '注册' })).toBeDisabled();
  });

  test('shows validation errors before submission', async ({ page }) => {
    await page.goto('/signup');

    await page.getByLabel('登录 ID').fill('invalid-id!');
    await page.getByLabel('昵称').fill('测试用户');
    await page.getByLabel('密码').fill('short');
    await page.getByLabel('登录 ID').blur();
    await page.getByLabel('密码').blur();

    await expect(page.getByText('登录 ID 只能包含英文和数字')).toBeVisible();
    await expect(page.getByText('密码至少 8 个字符')).toBeVisible();
    await expect(page.getByRole('button', { name: '注册' })).toBeDisabled();
  });

  test('submits a valid register request', async ({ page }) => {
    const loginId = `u${Date.now()}`;

    await page.goto('/signup');

    await page.getByLabel('登录 ID').fill(loginId);
    await page.getByLabel('昵称').fill('测试用户');
    await page.getByLabel('密码').fill('testpassword');
    await Promise.all([
      page.waitForResponse((response) =>
        response.url().includes('/v1/auth/register') &&
        response.request().method() === 'POST' &&
        response.status() === 201,
      ),
      page.getByRole('button', { name: '注册' }).click(),
    ]);

    await expect(page.getByText('注册成功')).toBeVisible();
  });
});
