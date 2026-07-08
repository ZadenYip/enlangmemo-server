import { Component } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLinkActive, RouterLinkWithHref, RouterOutlet } from '@angular/router';
import { APP_PATHS } from './app.routes';

@Component({
  selector: 'app-root',
  imports: [
    RouterOutlet,
    MatButtonModule,
    RouterLinkWithHref,
    RouterLinkActive,
  ],
  templateUrl: './app.html',
  styleUrl: './app.scss'
})
export class App {
  readonly navTab = {
    login: APP_PATHS.LOGIN,
    signup: APP_PATHS.SIGNUP,
  };
}
