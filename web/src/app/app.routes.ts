import { Routes } from '@angular/router';

export const APP_PATHS = {
    HOME: '',
    LOGIN: 'login',
    SIGNUP: 'signup',
    NOT_FOUND: '**'
}

export const routes: Routes = [
    {
        path: APP_PATHS.HOME,
        loadComponent: () => import('./home/home').then(
            (m) => m.Home
        ),
    },
    {
        path: APP_PATHS.LOGIN,
        loadComponent: () => import('./login/login').then(
            (m) => m.Login
        )
    },
    {
        path: APP_PATHS.SIGNUP,
        loadComponent: () => import('./sign-up/sign-up').then(
            (m) => m.SignUp
        )
    },
    {
        path: APP_PATHS.NOT_FOUND,
        loadComponent: () => import('./not-found/not-found').then(
            (m) => m.NotFound
        )
    }
];
