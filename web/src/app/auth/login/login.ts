import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import {
  form,
  FormField,
  FormRoot,
  maxLength,
  minLength,
  pattern,
  required,
} from '@angular/forms/signals';
import { firstValueFrom } from 'rxjs';
import { Auth as AuthService, LoginRequest, LoginResponse } from '../auth';
import { MsgService } from '../../shared/msg-service';

@Component({
  selector: 'app-login',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatIconModule,
    ReactiveFormsModule,
    FormField,
    FormRoot,
  ],
  templateUrl: './login.html',
  styleUrl: './login.scss',
})
export class Login {
  private readonly INITIAL_MODEL = {
    loginId: '',
    password: '',
  };
  private auth = inject(AuthService);
  private msg = inject(MsgService);

  loginModel = signal(this.INITIAL_MODEL);

  loginForm = form(
    this.loginModel,
    (schema) => {
      required(schema.loginId, { message: '登录 ID 不能为空' });
      maxLength(schema.loginId, 16, { message: '登录 ID 最多 16 个字符' });
      pattern(schema.loginId, /^[A-Za-z0-9]+$/, {
        message: '登录 ID 只能包含英文和数字',
      });

      required(schema.password, { message: '密码不能为空' });
      minLength(schema.password, 8, { message: '密码至少 8 个字符' });
      maxLength(schema.password, 16, { message: '密码最多 16 个字符' });
    },
    {
      submission: {
        action: async (field) => {
          const obj = field().value();
          const request: LoginRequest = {
            loginId: obj.loginId,
            password: obj.password,
          };

          try {
            const response = await firstValueFrom(this.auth.login(request));
            this.handleSuccess(response);
          } catch (error) {
            this.handleError(error);
          }
        },
      },
    },
  );

  private handleSuccess(response: LoginResponse) {
    this.loginForm().reset({ ...this.INITIAL_MODEL });
    this.msg.success('登录成功', response);
  }

  private handleError(error: unknown) {
    if (error instanceof HttpErrorResponse) {
      if (error.status === 404) {
        this.msg.warn('用户不存在', error);
        return;
      }

      if (error.status === 401) {
        this.msg.warn('登录 ID 或密码错误', error);
        return;
      }
    }

    this.msg.handleCommonError(error);
  }
}
