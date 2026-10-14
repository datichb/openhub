package auth

import "testing"

func TestLogin(t *testing.T) {
	if err := Login("alice", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if err := Login("alice", "wrong"); err != ErrBadCredentials {
		t.Fatalf("err = %v", err)
	}
}

func TestValidToken(t *testing.T) {
	if !ValidToken("abc", "abc") || ValidToken("abc", "abd") {
		t.Fatal("ValidToken")
	}
}
