import { createHash, randomBytes } from 'node:crypto';

import { expect, type APIRequestContext, type Page } from '@playwright/test';

export const testOAuthClientId = '00000000-0000-0000-0000-000000000001';
export const testOAuthRedirectUri = 'https://client.example/callback';

export type TestUser = {
  loginId: string;
  nickname: string;
  password: string;
};

export type AuthorizePathOptions = {
  clientId?: string;
  redirectUri?: string;
  state?: string;
  codeChallenge?: string;
};

export type AuthorizationResult = {
  authorizePath: string;
  callbackURL: URL;
  code: string;
  codeVerifier: string;
  state: string;
};

export async function registerUser(
  request: APIRequestContext,
  user?: Partial<TestUser>,
): Promise<TestUser> {
  const testUser: TestUser = {
    loginId: `u${randomBytes(6).toString('hex')}`,
    nickname: '测试用户',
    password: 'testpassword',
    ...user,
  };

  const response = await request.post('/v1/auth/register', {
    data: testUser,
  });
  expect(response.status()).toBe(201);

  return testUser;
}

export async function loginUser(page: Page, user: TestUser): Promise<void> {
  const response = await page.context().request.post('/v1/auth/login', {
    data: {
      loginId: user.loginId,
      password: user.password,
    },
  });

  expect(response.status()).toBe(200);

  const cookies = await page.context().cookies();
  expect(cookies.find((cookie) => cookie.name === '__Host-sso_token')?.value).toBeTruthy();
}

export function randomState(): string {
  return randomBytes(32).toString('base64url');
}

export function s256(value: string): string {
  return createHash('sha256').update(value).digest('base64url');
}

export function newAuthorizePath(options?: AuthorizePathOptions): string {
  const params = new URLSearchParams({
    response_type: 'code',
    client_id: options?.clientId ?? '00000000-0000-0000-0000-000000000000',
    redirect_uri: options?.redirectUri ?? testOAuthRedirectUri,
    state: options?.state ?? 'state-value',
    code_challenge: options?.codeChallenge ?? '0123456789012345678901234567890123456789012',
    code_challenge_method: 'S256',
  });

  return `/v1/oauth/authorize?${params.toString()}`;
}

/**
 * 直接向 /v1/oauth/authorize 发送请求（这里已包括注册和登录了）。
 * 这里不跟随重定向，直接读取第一跳 Location，避免浏览器真的加载测试用的 redirect_uri。
 * @param page - Playwright 页面对象
 * @param request - Playwright API 请求上下文
 * @param options - 可选自定义参数，包括 state、codeVerifier、clientId 和 redirectUri
 * @returns
 */
export async function authorizePKCE(
  page: Page,
  request: APIRequestContext,
  options?: {
    state?: string;
    codeVerifier?: string;
    clientId?: string;
    redirectUri?: string;
  },
): Promise<AuthorizationResult> {
  const user = await registerUser(request);
  await loginUser(page, user);

  const state = options?.state ?? randomState();
  const codeVerifier = options?.codeVerifier ?? randomState();
  const redirectUri = options?.redirectUri ?? testOAuthRedirectUri;
  const authorizePath = newAuthorizePath({
    clientId: options?.clientId ?? testOAuthClientId,
    redirectUri,
    state,
    codeChallenge: s256(codeVerifier),
  });

  const response = await page.context().request.get(authorizePath, {
    maxRedirects: 0,
  });
  expect(response.status()).toBe(302);

  const location = response.headers()['location'];
  expect(location).toBeTruthy();

  const callbackURL = new URL(location ?? '');
  expect(callbackURL.searchParams.get('error')).toBeNull();
  const code = callbackURL.searchParams.get('code');
  expect(code).toBeTruthy();

  return {
    authorizePath,
    callbackURL,
    code: code ?? '',
    codeVerifier,
    state,
  };
}
