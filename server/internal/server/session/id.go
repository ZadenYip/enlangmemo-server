package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

const DefaultIDLen = 32

var ErrInvalidIDByteLen = errors.New("id byte length must be positive")

// NewID 生成一个随机 ID（密码学上安全），使用 hex 编码
func NewID(byteLen int) (string, error) {
	b, err := randomIDBytes(byteLen)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// NewIDWithBase64RawURL 会生成密码学上安全的随机 ID，使用 base64 raw URL 编码
func NewIDWithBase64RawURL(byteLen int) (string, error) {
	b, err := randomIDBytes(byteLen)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomIDBytes(byteLen int) ([]byte, error) {
	if byteLen <= 0 {
		return nil, ErrInvalidIDByteLen
	}

	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
