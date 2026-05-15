package crypto_test

import (
	"testing"

	"auth-service/pkg/crypto"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	plaintext := "ya29.A0AXeO80T_example_access_token"

	encrypted, err := crypto.EncryptAES256GCM(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if encrypted == plaintext {
		t.Fatal("encrypted text should not equal plaintext")
	}

	decrypted, err := crypto.DecryptAES256GCM(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptDifferentNonceEachTime(t *testing.T) {
	key := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	plaintext := "same_token"
	enc1, _ := crypto.EncryptAES256GCM(key, plaintext)
	enc2, _ := crypto.EncryptAES256GCM(key, plaintext)
	if enc1 == enc2 {
		t.Fatal("two encryptions of same text should differ (different nonces)")
	}
}

func TestInvalidKey(t *testing.T) {
	_, err := crypto.EncryptAES256GCM("tooshort", "plaintext")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}
