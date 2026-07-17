import { test, expect } from '@playwright/test';
import { registerUser } from './helpers';

test.describe('Auth/login', () => {
  test('logs in with a registered user', async ({ page, request }) => {
    const user = await registerUser(request);

    await page.goto('/login');

    await page.getByLabel('登录 ID').fill(user.loginId);
    await page.getByLabel('密码').fill(user.password);
    const [loginResponse] = await Promise.all([
      page.waitForResponse((response) =>
        response.url().includes('/v1/auth/login') &&
        response.request().method() === 'POST' &&
        response.status() === 200,
      ),
      page.getByRole('button', { name: '登录' }).click(),
    ]);

    await expect(page.getByText('登录成功')).toBeVisible();

    const cookies = await page.context().cookies();
    const accessTokenCookie = cookies.find((cookie) => cookie.name === '__Host-sso_token');
    expect(accessTokenCookie).toBeDefined();
    expect(accessTokenCookie?.value).toBeDefined();
  });
});
