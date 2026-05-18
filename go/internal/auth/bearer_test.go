package auth

import "testing"

func TestBearerRejectsMissingTokenWhenConfigured(t *testing.T) {
	ok := ValidateBearer("secret-token", "")
	if ok {
		t.Fatal("expected missing bearer token to be rejected")
	}
}

func TestBearerAcceptsMatchingToken(t *testing.T) {
	ok := ValidateBearer("secret-token", "Bearer secret-token")
	if !ok {
		t.Fatal("expected matching bearer token to be accepted")
	}
}

func TestBearerAllowsRequestsWhenTokenNotConfigured(t *testing.T) {
	ok := ValidateBearer("", "")
	if !ok {
		t.Fatal("expected auth to be disabled when no token is configured")
	}
}
