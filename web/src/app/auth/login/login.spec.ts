import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Login, safeOAuthReturnTo } from './login';

describe('Login', () => {
  let component: Login;
  let fixture: ComponentFixture<Login>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [Login],
    }).compileComponents();

    fixture = TestBed.createComponent(Login);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});

describe('safeOAuthReturnTo', () => {
  // 测试 safeOAuthReturnTo 函数的行为，确保它只允许安全的 OAuth 返回路径，并拒绝外部 URL
  it('allows OAuth authorize return paths', () => {
    expect(safeOAuthReturnTo('/v1/oauth/authorize?client_id=client-id')).toBe(
      '/v1/oauth/authorize?client_id=client-id',
    );
  });

  // 测试 safeOAuthReturnTo 拒绝外部 URL
  it('rejects external return URLs', () => {
    expect(safeOAuthReturnTo('https://attacker.example')).toBeNull();
    expect(safeOAuthReturnTo('//attacker.example/v1/oauth/authorize')).toBeNull();
  });

  // 测试 safeOAuthReturnTo 拒绝非授权开头的路径
  it('rejects non-authorize return paths', () => {
    expect(safeOAuthReturnTo('/login')).toBeNull();
  });
});
