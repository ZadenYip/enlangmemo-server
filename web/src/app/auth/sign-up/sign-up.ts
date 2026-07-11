import { Component, inject, signal } from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import {
  form,
  FormField,
  maxLength,
  minLength,
  pattern,
  required,
  FormRoot,
} from '@angular/forms/signals';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { Auth as AuthService, RegisterRequest, RegisterResponse } from '../auth';
import { MatSnackBar } from '@angular/material/snack-bar';

@Component({
  selector: 'app-sign-up',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatIconModule,
    ReactiveFormsModule,
    FormField,
    FormRoot,
  ],
  templateUrl: './sign-up.html',
  styleUrl: './sign-up.scss',
})
export class SignUp {
  private readonly INITIAL_MODEL = {
    loginId: '',
    nickname: '',
    password: '',
  };
  private http = inject(HttpClient);
  private auth = inject(AuthService);
  private snackBar = inject(MatSnackBar);

  signUpModel = signal(this.INITIAL_MODEL);

  signUpForm = form(
    this.signUpModel,
    (schema) => {
      required(schema.loginId, { message: '登录 ID 不能为空' });
      maxLength(schema.loginId, 16, { message: '登录 ID 最多 16 个字符' });
      pattern(schema.loginId, /^[A-Za-z0-9]+$/, {
        message: '登录 ID 只能包含英文和数字',
      });

      required(schema.nickname, { message: '昵称不能为空' });
      maxLength(schema.nickname, 16, { message: '昵称最多 16 个字符' });

      required(schema.password, {message: '密码不能为空'})
      minLength(schema.password, 8, { message: '密码至少 8 个字符' });
      maxLength(schema.password, 16, { message: '密码最多 16 个字符' });
    },
    {
      submission: {
        action: async (field) => {
          const obj = field().value();
          const request: RegisterRequest = {
            loginId: obj.loginId,
            nickname: obj.nickname,
            password: obj.password,
          };
          try {
            const response = await firstValueFrom(this.auth.register(request));
            this.handleSuccess(response);
          } catch (error) {
            this.handleError(error);
          }
        },
      },
    },
  );

  private handleSuccess(response: RegisterResponse) {
    console.log('注册成功', response);
    this.signUpForm().reset({...this.INITIAL_MODEL});
    this.snackBar.open('注册成功', '关闭', {
      duration: 2000,
    });
  }

  private handleError(error: unknown) {
    if (error instanceof HttpErrorResponse) {
      // 处理账号已经注册
      switch (error.status) {
        case 409:
          console.error('账号已经注册', error);
          this.snackBar.open('该ID已被注册', '关闭', {
            duration: 3000,
          });
          break;
        case 0:
          if (error.error instanceof ProgressEvent) {
            this.snackBar.open('连接服务器失败，请检查网络连接', '关闭', {
              duration: 3000,
            });
            console.error('连接服务器失败，请检查网络连接');
          } else {
            console.error('未知错误', error);
            this.snackBar.open('未知错误，请联系开发者', '关闭', {
              duration: 3000,
            });
          }
          break;
        default:
          console.error('内部服务器错误', error);
          this.snackBar.open('内部服务器错误，请联系开发者', '关闭', {
            duration: 3000,
          });
          break;
      }
    } else {
      console.error('未知错误', error);
      this.snackBar.open('未知错误，请联系开发者', '关闭', {
        duration: 3000,
      });
    }
  }
}
