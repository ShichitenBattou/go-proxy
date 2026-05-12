package redis

import (
	"bytes"
	"testing"
)

var testKey = bytes.Repeat([]byte{0x00}, 32)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plaintext := []byte(`{"user_id":"sub-123","access_token":"secret-token"}`)

	ciphertext, err := encrypt(plaintext, testKey)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	got, err := decrypt(ciphertext, testKey)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch: want %q, got %q", plaintext, got)
	}
}

func TestEncrypt_ProducesDistinctCiphertexts(t *testing.T) {
	plaintext := []byte("same plaintext")

	c1, err := encrypt(plaintext, testKey)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}
	c2, err := encrypt(plaintext, testKey)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	if bytes.Equal(c1, c2) {
		t.Error("expected distinct ciphertexts due to random nonce, but got identical output")
	}
}

func TestDecrypt_FailsOnTamperedData(t *testing.T) {
	plaintext := []byte("tamper test")

	ciphertext, err := encrypt(plaintext, testKey)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Flip the last byte to simulate tampering
	ciphertext[len(ciphertext)-1] ^= 0xff

	if _, err := decrypt(ciphertext, testKey); err == nil {
		t.Error("expected decryption to fail on tampered ciphertext, but it succeeded")
	}
}

func TestDecrypt_FailsOnWrongKey(t *testing.T) {
	plaintext := []byte("wrong key test")

	ciphertext, err := encrypt(plaintext, testKey)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	wrongKey := bytes.Repeat([]byte{0xff}, 32)
	if _, err := decrypt(ciphertext, wrongKey); err == nil {
		t.Error("expected decryption to fail with wrong key, but it succeeded")
	}
}

func TestDecrypt_FailsOnTooShortData(t *testing.T) {
	if _, err := decrypt([]byte("short"), testKey); err == nil {
		t.Error("expected error for too-short ciphertext, but got nil")
	}
}
