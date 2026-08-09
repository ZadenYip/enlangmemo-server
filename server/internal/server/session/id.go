package session

import (
	"crypto/rand"
	"encoding/base64"
)

// 生成一个随机 ID（密码学上安全），使用标准 base64 编码
func NewID() (string, error) {
	b, err := randomIDBytes()
	if err != nil {
		return "", err
	}
	return string(b[:]), nil
}

// 生成一个随机 ID（密码学上安全），使用 base64 raw URL 编码
func NewIDWithBase64RawURL() (string, error) {
	b, err := randomIDBytes()
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomIDBytes() ([]byte, error) {
	const idSize = 32
	b := make([]byte, idSize)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
