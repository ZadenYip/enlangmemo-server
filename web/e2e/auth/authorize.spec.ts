import { expect, test } from '@playwright/test';
import { authorizePKCE, loginUser, newAuthorizePath, randomState, registerUser } from './helpers';

test.describe('Auth/authorize', () => {
  /**
   * 测试如果没登录的话，访问 /v1/oauth/authorize 会跳转到登录页
   */
  test('redirects unauthenticated users to login', async ({ page }) => {
    const authorizePath = newAuthorizePath();

    await page.goto(authorizePath);

    await expect(page).toHaveURL(/\/login\?/);

    const loginURL = new URL(page.url());
    expect(loginURL.pathname).toBe('/login');
    expect(loginURL.searchParams.get('return_to')).toBe(authorizePath);
  });

  /**
   * 测试授权重定向到登录后，登录成功后会返回到授权页，并此时有 SSO cookie 了
   */
  test('returns to authorize with session cookie after login', async ({ page, request }) => {
    const user = await registerUser(request);
    const authorizePath = newAuthorizePath();

    await page.goto(authorizePath);

    await page.getByLabel('登录 ID').fill(user.loginId);
    await page.getByLabel('密码').fill(user.password);
    await Promise.all([
      page.waitForResponse(
        (response) =>
          response.url().includes('/v1/auth/login') &&
          response.request().method() === 'POST' &&
          response.status() === 200,
      ),
      page.getByRole('button', { name: '登录' }).click(),
    ]);

    await expect(page).toHaveURL((url) => url.pathname === '/v1/oauth/authorize');

    const curURL = new URL(page.url());
    expect(`${curURL.pathname}${curURL.search}`).toBe(authorizePath);

    const ssoCookie = (await page.context().cookies()).find(
      (cookie) => cookie.name === '__Host-sso_token',
    );
    expect(ssoCookie?.value).toBeTruthy();
  });

  /**
   * 授权时 client_id 不合法
   */
  test('already logged in users can access authorize endpoint', async ({ page, request }) => {
    const user = await registerUser(request);

    // 登录用户
    await loginUser(page, user);
    const authorizePath = newAuthorizePath();

    const response = await page.goto(authorizePath);
    const invalidStatus = 400;
    expect(response?.status()).toBe(invalidStatus);
  });

  /**
   * 测试已经登录的用户访问 /v1/oauth/authorize 时，如果请求合法，会返回授权码
   */
  test('returns authorization code for logged in users with valid PKCE request', async ({
    page,
    request,
  }) => {
    const state = randomState();
    const authorization = await authorizePKCE(page, request, {
      state,
    });

    expect(authorization.callbackURL.searchParams.get('state')).toBe(state);
    expect(authorization.code).toBeTruthy();
    expect(authorization.callbackURL.searchParams.get('error')).toBeNull();
  });

  /**
   * 测试回调的 state 与客户端预期不匹配时，客户端不能继续信任该授权结果
   */
  test('detects mismatched callback state', async ({ page, request }) => {
    const expectedState = 'expected-state';
    const authorization = await authorizePKCE(page, request, {
      state: 'actual-state',
    });

    expect(authorization.callbackURL.searchParams.get('state')).not.toBe(expectedState);
  });
});
