package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("hash is empty")
	}
	if hash == "secret123" {
		t.Fatal("hash equals plain text")
	}

	if err := VerifyPassword(hash, "secret123"); err != nil {
		t.Errorf("VerifyPassword correct: %v", err)
	}
	if err := VerifyPassword(hash, "wrong"); err == nil {
		t.Error("VerifyPassword wrong password should fail")
	}
}

func TestHashPasswordUnique(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("same password should produce different hashes (salt)")
	}
}
