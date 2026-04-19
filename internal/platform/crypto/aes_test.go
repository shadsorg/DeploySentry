package crypto

import (
	"crypto/rand"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}
	plaintext := []byte("my-webhook-secret-value")
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if string(ciphertext) == string(plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}
	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("got %q, want %q", decrypted, plaintext)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}
	ciphertext, _ := Encrypt([]byte("secret"), key1)
	_, err := Decrypt(ciphertext, key2)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	_, err := Encrypt([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}
