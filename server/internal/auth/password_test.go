package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("integration-test-password")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword("integration-test-password", hash)
	if err != nil || !ok {
		t.Fatalf("verify ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong", hash)
	if err != nil || ok {
		t.Fatalf("expected wrong password to fail")
	}
}
