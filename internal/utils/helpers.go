package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// GenerateReferralCode generates a unique referral code
func GenerateReferralCode(name string) string {
	// Take first 3 characters of name (uppercase)
	prefix := strings.ToUpper(name)
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}

	// Generate random suffix
	bytes := make([]byte, 4)
	rand.Read(bytes)
	suffix := strings.ToUpper(hex.EncodeToString(bytes))[:6]

	return prefix + "-" + suffix
}

// GenerateRandomString generates a random string of specified length
func GenerateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}
