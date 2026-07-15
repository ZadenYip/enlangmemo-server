import { HttpErrorResponse } from '@angular/common/http';
import { inject, Service } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';

@Service()
export class MsgService {
  private snackBar = inject(MatSnackBar);

  success(message: string, data?: unknown): void {
    console.info(message, data);
    this.snackBar.open(message, '关闭', {
      duration: 2000,
    });
  }

  warn(message: string, error?: unknown): void {
    console.warn(message, error);
    this.snackBar.open(message, '关闭', {
      duration: 3000,
    });
  }

  error(message: string, error?: unknown): void {
    console.error(message, error);
    this.snackBar.open(message, '关闭', {
      duration: 3000,
    });
  }

  handleCommonError(error: unknown): void {
    if (error instanceof HttpErrorResponse) {
      if (error.status === 0 && error.error instanceof ProgressEvent) {
        this.error('连接服务器失败，请检查网络连接', error);
        return;
      }

      if (error.status === 0) {
        this.error('未知错误，请联系开发者', error);
        return;
      }

      this.error('内部服务器错误，请联系开发者', error);
      return;
    }

    this.error('未知错误，请联系开发者', error);
  }
}
