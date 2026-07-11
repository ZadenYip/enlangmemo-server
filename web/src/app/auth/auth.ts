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
