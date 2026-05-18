package auth

import "strings"

func ValidateBearer(expectedToken, header string) bool {
	if expectedToken == "" {
		return true
	}
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	return strings.TrimPrefix(header, "Bearer ") == expectedToken
}
