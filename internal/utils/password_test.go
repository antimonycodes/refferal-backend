package utils

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"simple password", "password123"},
		{"complex password", "MyP@ssw0rd!123"},
		{"long password", "thisisaverylongpasswordthatshouldalsowork123456"},
		{"unicode password", "密码123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if err != nil {
				t.Fatalf("HashPassword() error = %v", err)
			}

			if hash == "" {
				t.Error("HashPassword() returned empty hash")
			}

			if hash == tt.password {
				t.Error("HashPassword() returned unhashed password")
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	password := "testPassword123!"
	hash, _ := HashPassword(password)

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{"correct password", password, hash, true},
		{"wrong password", "wrongPassword", hash, false},
		{"empty password", "", hash, false},
		{"similar password", "testPassword123", hash, false},
		{"case sensitive", "TestPassword123!", hash, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckPassword(tt.password, tt.hash); got != tt.want {
				t.Errorf("CheckPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashPasswordUniqueness(t *testing.T) {
	password := "samePassword123"
	
	hash1, _ := HashPassword(password)
	hash2, _ := HashPassword(password)

	if hash1 == hash2 {
		t.Error("HashPassword() should produce unique hashes for same password")
	}

	// Both should still validate
	if !CheckPassword(password, hash1) {
		t.Error("hash1 should validate")
	}
	if !CheckPassword(password, hash2) {
		t.Error("hash2 should validate")
	}
}
