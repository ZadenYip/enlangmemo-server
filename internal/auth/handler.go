package auth

import "github.com/alexedwards/argon2id"

type AuthHandler struct {
	users    UserStore
	sessions SessionStore
}

func NewAuthHandler(users UserStore, sessions SessionStore) *AuthHandler {
	return &AuthHandler{
		users:    users,
		sessions: sessions,
	}
}

// 参数设置参考
// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#introduction
var argon2Params = argon2id.Params{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}
