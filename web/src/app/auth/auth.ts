import { HttpClient } from '@angular/common/http';
import { inject, Service } from '@angular/core';
import { Observable } from 'rxjs';
import { environment } from '../../environments/environment';

export interface LoginRequest {
  loginId: string;
  password: string;
};
  
export type LoginResponse = Record<string, never>;

export interface RegisterRequest {
  loginId: string;
  nickname: string;
  password: string;
};

export interface RegisterResponse {
  userId: string;
}

@Service()
export class Auth {
  private http = inject(HttpClient);
  private readonly baseUrl = environment.apiUrl + "auth/";

  register(request: RegisterRequest): Observable<RegisterResponse> {
    return this.http.post<RegisterResponse>(this.baseUrl + "register", request, {
      timeout: 10 * 1000,
    });
  }

  login (request: LoginRequest): Observable<LoginResponse> {
    return this.http.post<LoginResponse>(this.baseUrl + "login", request, {
      timeout: 10 * 1000,
    });
  }
}

/**
 * 检查给定的 returnTo 是不是相对当前网站的路径，而不是外部 URL，路径开头是不是 /v1/oauth/authorize
 * @param returnTo - 要检查的返回路径
 * @returns 如果 returnTo 是一个安全的 OAuth 返回路径，则返回该路径，否则返回 null
 * 
 * 这个函数用于检查给定的 returnTo 参数是否是一个安全的 OAuth 返回路径。它确保：
 */
export function safeOAuthReturnTo(returnTo: string | null): string | null {
  if (returnTo === null || !returnTo.startsWith('/') || returnTo.startsWith('//')) {
    return null;
  }

  const url = new URL(returnTo, window.location.origin);
  if (url.origin !== window.location.origin || url.pathname !== '/v1/oauth/authorize') {
    return null;
  }

  return `${url.pathname}${url.search}`;
}
