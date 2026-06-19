package secret

import "testing"

func TestAESGCMCodecEncryptsAndDecrypts(t *testing.T) {
	codec, err := NewAESGCMCodec("test-secret")
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}

	ciphertext, err := codec.Encrypt("root:root@tcp(127.0.0.1:3306)/ling_shu")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == "root:root@tcp(127.0.0.1:3306)/ling_shu" {
		t.Fatal("expected ciphertext to differ from plaintext")
	}

	plaintext, err := codec.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plaintext != "root:root@tcp(127.0.0.1:3306)/ling_shu" {
		t.Fatalf("unexpected plaintext: %s", plaintext)
	}
}

func TestAESGCMCodecKeepsPlaintextBackwardCompatible(t *testing.T) {
	codec, err := NewAESGCMCodec("test-secret")
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}

	plaintext, err := codec.Decrypt("legacy-dsn")
	if err != nil {
		t.Fatalf("decrypt legacy plaintext: %v", err)
	}
	if plaintext != "legacy-dsn" {
		t.Fatalf("expected legacy plaintext, got %s", plaintext)
	}
}
