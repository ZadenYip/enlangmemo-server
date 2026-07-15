package session

import (
	"crypto/rand"
	"encoding/base64"
)

// 生成一个随机 ID（密码学上安全），使用 base64 URL 编码
func NewID() (string, error) {
	const idSize = 32
	b := make([]byte, idSize)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
