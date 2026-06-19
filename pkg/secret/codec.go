package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const encryptedPrefix = "enc:v1:"

var ErrInvalidCiphertext = errors.New("invalid encrypted secret")

type Codec interface {
	Encrypt(value string) (string, error)
	Decrypt(value string) (string, error)
}

type PlainCodec struct{}

func (PlainCodec) Encrypt(value string) (string, error) {
	return value, nil
}

func (PlainCodec) Decrypt(value string) (string, error) {
	return value, nil
}

type AESGCMCodec struct {
	aead cipher.AEAD
}

func NewAESGCMCodec(secret string) (Codec, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return PlainCodec{}, nil
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCMCodec{aead: aead}, nil
}

func (c *AESGCMCodec) Encrypt(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(value), nil)
	return encryptedPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *AESGCMCodec) Decrypt(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	payload := strings.TrimPrefix(value, encryptedPrefix)
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	nonceSize := c.aead.NonceSize()
	if len(raw) <= nonceSize {
		return "", ErrInvalidCiphertext
	}
	plain, err := c.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plain), nil
}
