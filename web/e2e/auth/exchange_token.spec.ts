import { expect, test } from '@playwright/test';

import { authorizePKCE, testOAuthClientId, testOAuthRedirectUri } from './helpers';

test.describe('Auth/exchange_token', () => {
  /**
   * 测试正常的 PKCE 授权码交换流程
   */
  test('exchanges an authorization code for an access token', async ({ page, request }) => {
    const authorization = await authorizePKCE(page, request);

    const response = await request.post('/v1/oauth/token', {
      form: {
        grant_type: 'authorization_code',
        code: authorization.code,
        redirect_uri: testOAuthRedirectUri,
        client_id: testOAuthClientId,
        code_verifier: authorization.codeVerifier,
      },
    });

    expect(response.status()).toBe(200);
    expect(response.headers()['cache-control']).toBe('no-store');
    expect(response.headers()['pragma']).toBe('no-cache');

    const body = await response.json();
    expect(body.access_token).toBeTruthy();
    expect(body.token_type).toBe('bearer');
    const expiresIn = 3600 * 24;
    expect(body.expires_in).toBe(expiresIn);
  });
});
