package service

import (
	"fmt"

	"ling-shu/pkg/secret"
)

func encryptSecret(codec secret.Codec, value string) (string, error) {
	if codec == nil {
		codec = secret.PlainCodec{}
	}
	encrypted, err := codec.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSecretEncryptFailed, err)
	}
	return encrypted, nil
}

func decryptSecret(codec secret.Codec, value string) (string, error) {
	if codec == nil {
		codec = secret.PlainCodec{}
	}
	plaintext, err := codec.Decrypt(value)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSecretDecryptFailed, err)
	}
	return plaintext, nil
}
